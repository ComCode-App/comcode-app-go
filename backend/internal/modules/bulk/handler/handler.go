package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	bulkusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/bulk/usecase"
	cpmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyprofiles/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/bulkengine"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/bulkengine/adapters"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/apiquery"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
	"github.com/google/uuid"
)

type Handler struct {
	svc *bulkusecase.Service
}

func New(svc *bulkusecase.Service) *Handler {
	return &Handler{svc: svc}
}

type bulkRequest struct {
	Action string                `json:"action"`
	Target bulkengine.BulkTarget `json:"target"`
	Locale string                `json:"locale"`
}

func (h *Handler) ExecuteUsers(w http.ResponseWriter, r *http.Request) {
	h.execute(w, r, adapters.ResourceUsers)
}

func (h *Handler) ExecuteRoles(w http.ResponseWriter, r *http.Request) {
	h.execute(w, r, adapters.ResourceRoles)
}

func (h *Handler) execute(w http.ResponseWriter, r *http.Request, resource string) {
	p := authctx.MustPrincipal(r.Context())
	var body bulkRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid JSON body")
		return
	}
	action := strings.TrimSpace(body.Action)
	if action == "" {
		response.BadRequest(w, r, response.CodeValidationError, "action is required")
		return
	}
	def, _, err := h.svc.Registry().ActionDef(resource, action)
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "unknown bulk action")
		return
	}
	if !p.HasPermission(def.Permission) {
		response.Forbidden(w, r, "Missing permission for bulk action")
		return
	}
	result, err := h.svc.Execute(r.Context(), p.UserInternal, bulkusecase.ExecuteInput{
		Resource: resource,
		Action:   action,
		Target:   body.Target,
		Locale:   body.Locale,
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if result.Async {
		response.JSON(w, r, http.StatusAccepted, result.Job)
		return
	}
	response.JSON(w, r, http.StatusOK, map[string]any{
		"sync":    true,
		"summary": result.Summary,
	})
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	q := apiquery.Parse(r.URL.Query())
	admin := p.HasPermission(rbac.PermPlatformSettingsWrite)
	items, total, err := h.svc.ListJobs(r.Context(), p.UserInternal, admin, q.Limit, q.Offset)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, apiquery.NewPage(items, total, q.Limit, q.Offset))
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "uuid is invalid")
		return
	}
	admin := p.HasPermission(rbac.PermPlatformSettingsWrite)
	job, err := h.svc.GetJob(r.Context(), id, p.UserInternal, admin)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, job)
}

func (h *Handler) Rollback(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "uuid is invalid")
		return
	}
	admin := p.HasPermission(rbac.PermPlatformSettingsWrite)
	job, err := h.svc.GetJob(r.Context(), id, p.UserInternal, admin)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	def, _, err := h.svc.Registry().ActionDef(job.Resource, job.Action)
	if err != nil || !p.HasPermission(def.Permission) {
		response.Forbidden(w, r, "Missing permission for bulk rollback")
		return
	}
	updated, err := h.svc.Rollback(r.Context(), id, p.UserInternal, bulkusecase.JobAccess{})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, updated)
}

type companyBulkRequest struct {
	Action string                `json:"action"`
	Target bulkengine.BulkTarget `json:"target"`
	Params json.RawMessage       `json:"params"`
	Locale string                `json:"locale"`
}

// companyAccess is the job reach of the caller in the active company account: owners/admins see
// and roll back every job of the account, office managers only their own.
func companyAccess(p authctx.Principal) (bulkusecase.JobAccess, bool) {
	if p.Account.ID <= 0 || p.Account.Type != authctx.AccountTypeCompany {
		return bulkusecase.JobAccess{}, false
	}
	switch p.Account.Role {
	case authctx.AccountRoleOwner, authctx.AccountRoleAdmin:
		return bulkusecase.JobAccess{AccountID: p.Account.ID, AnyActor: true}, true
	case authctx.AccountRoleOfficeManager:
		return bulkusecase.JobAccess{AccountID: p.Account.ID}, true
	default:
		return bulkusecase.JobAccess{}, false
	}
}

// ExecuteCompanyProfiles runs a bulk action on employee profiles of the active company account
// (TEC-573). Runs under BULK_SYNC_MAX finish inline and answer 200 with the job; larger ones 202.
func (h *Handler) ExecuteCompanyProfiles(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	access, ok := companyAccess(p)
	if !ok {
		response.Forbidden(w, r, "Company owner, admin or office manager role required")
		return
	}
	var body companyBulkRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid JSON body")
		return
	}
	action := strings.TrimSpace(body.Action)
	def, _, err := h.svc.Registry().ActionDef(cpmodel.ResourceCompanyProfiles, action)
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "unknown bulk action")
		return
	}
	if !p.HasPermission(def.Permission) {
		response.Forbidden(w, r, "Missing permission for bulk action")
		return
	}
	result, err := h.svc.Execute(r.Context(), p.UserInternal, bulkusecase.ExecuteInput{
		Resource:  cpmodel.ResourceCompanyProfiles,
		Action:    action,
		Target:    body.Target,
		Locale:    body.Locale,
		AccountID: access.AccountID,
		Params:    body.Params,
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if result.Async {
		response.JSON(w, r, http.StatusAccepted, result.Job)
		return
	}
	response.JSON(w, r, http.StatusOK, map[string]any{
		"sync":    true,
		"summary": result.Summary,
		"job":     result.Job,
	})
}

// CompanyList lists bulk jobs of the active company account.
func (h *Handler) CompanyList(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	access, ok := companyAccess(p)
	if !ok {
		response.Forbidden(w, r, "Company owner, admin or office manager role required")
		return
	}
	q := apiquery.Parse(r.URL.Query())
	items, total, err := h.svc.ListAccountJobs(r.Context(), p.UserInternal, access, cpmodel.ResourceCompanyProfiles, q.Limit, q.Offset)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, apiquery.NewPage(items, total, q.Limit, q.Offset))
}

// CompanyGet returns one bulk job of the active company account.
func (h *Handler) CompanyGet(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	access, ok := companyAccess(p)
	if !ok {
		response.Forbidden(w, r, "Company owner, admin or office manager role required")
		return
	}
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "uuid is invalid")
		return
	}
	job, err := h.svc.GetAccountJob(r.Context(), id, p.UserInternal, access)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, job)
}

// CompanyRollback reverts a bulk job of the active company account within its window.
func (h *Handler) CompanyRollback(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	access, ok := companyAccess(p)
	if !ok {
		response.Forbidden(w, r, "Company owner, admin or office manager role required")
		return
	}
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "uuid is invalid")
		return
	}
	job, err := h.svc.GetAccountJob(r.Context(), id, p.UserInternal, access)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	def, _, err := h.svc.Registry().ActionDef(job.Resource, job.Action)
	if err != nil || !p.HasPermission(def.Permission) {
		response.Forbidden(w, r, "Missing permission for bulk rollback")
		return
	}
	updated, err := h.svc.Rollback(r.Context(), id, p.UserInternal, access)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, updated)
}

func writeErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, bulkusecase.ErrNotFound):
		response.NotFound(w, r, "Bulk job not found")
	case errors.Is(err, bulkusecase.ErrForbidden):
		response.Forbidden(w, r, "Forbidden")
	case errors.Is(err, bulkusecase.ErrInvalidRequest):
		response.BadRequest(w, r, response.CodeValidationError, err.Error())
	default:
		response.InternalErr(w, r, err, "unexpected error")
	}
}
