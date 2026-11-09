package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	exportusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/exports/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/ioengine"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/apiquery"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
	"github.com/google/uuid"
)

// LeadsExportGate takes a lead export from the account's monthly quota and unlocks what the plan
// allows before the file is built (leads usecase, TEC-941).
type LeadsExportGate interface {
	PrepareExport(ctx context.Context, p authctx.Principal) error
}

// Handler exposes export HTTP endpoints.
type Handler struct {
	svc   *exportusecase.Service
	leads LeadsExportGate
}

func New(svc *exportusecase.Service) *Handler {
	return &Handler{svc: svc}
}

// SetLeadsGate wires the lead export quota. Without it lead exports are refused (fail-closed).
func (h *Handler) SetLeadsGate(g LeadsExportGate) { h.leads = g }

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

func (h *Handler) Download(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "uuid is invalid")
		return
	}
	admin := p.HasPermission(rbac.PermPlatformSettingsWrite)
	file, ct, filename, err := h.svc.Download(r.Context(), id, p.UserInternal, admin)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	defer func() { _ = file.Close() }()
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", contentDispositionAttachment(filename))
	_, _ = io.Copy(w, file)
}

// GetOwn answers GET /v1/exports/{uuid}: an app (account) export job of the caller.
func (h *Handler) GetOwn(w http.ResponseWriter, r *http.Request) {
	job, ok := h.ownAppJob(w, r)
	if !ok {
		return
	}
	response.JSON(w, r, http.StatusOK, job)
}

// DownloadOwn answers GET /v1/exports/{uuid}/download for the caller's app export jobs.
func (h *Handler) DownloadOwn(w http.ResponseWriter, r *http.Request) {
	job, ok := h.ownAppJob(w, r)
	if !ok {
		return
	}
	p := authctx.MustPrincipal(r.Context())
	file, ct, filename, err := h.svc.Download(r.Context(), job.UUID, p.UserInternal, false)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	defer func() { _ = file.Close() }()
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", contentDispositionAttachment(filename))
	_, _ = io.Copy(w, file)
}

// ownAppJob loads a job the caller requested for an app.* resource; anything else is not found.
func (h *Handler) ownAppJob(w http.ResponseWriter, r *http.Request) (exportusecase.ExportJobView, bool) {
	p := authctx.MustPrincipal(r.Context())
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "uuid is invalid")
		return exportusecase.ExportJobView{}, false
	}
	job, err := h.svc.GetJob(r.Context(), id, p.UserInternal, false)
	if err == nil && !exportusecase.IsAppResource(job.Resource) {
		err = exportusecase.ErrNotFound
	}
	if errors.Is(err, exportusecase.ErrForbidden) {
		err = exportusecase.ErrNotFound
	}
	if err != nil {
		writeErr(w, r, err)
		return exportusecase.ExportJobView{}, false
	}
	return job, true
}

func contentDispositionAttachment(filename string) string {
	safe := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-' {
			return r
		}
		return '_'
	}, filename)
	return fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, safe, url.PathEscape(filename))
}

func (h *Handler) RequestUsersExport(w http.ResponseWriter, r *http.Request) {
	h.requestExport(w, r, "platform.users", rbac.PermPlatformUsersExport)
}

func (h *Handler) RequestRolesExport(w http.ResponseWriter, r *http.Request) {
	h.requestExport(w, r, "platform.roles", rbac.PermPlatformRolesExport)
}

func (h *Handler) RequestNotificationsExport(w http.ResponseWriter, r *http.Request) {
	h.requestExport(w, r, "platform.notifications", rbac.PermPlatformNotificationsExport)
}

func (h *Handler) RequestActivityExport(w http.ResponseWriter, r *http.Request) {
	h.requestExport(w, r, "platform.activity", rbac.PermPlatformActivityRead)
}

// RequestOrderManifestExport exports the production manifest of one order (TEC-581). The order
// comes from the path; the file holds activation codes, so the route also requires step-up.
func (h *Handler) RequestOrderManifestExport(w http.ResponseWriter, r *http.Request) {
	orderUUID, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid order uuid")
		return
	}
	h.requestExportWith(w, r, "platform.order_manifest", rbac.PermPlatformOrdersRead,
		ioengine.ExportQuery{"order_uuid": orderUUID.String()}, nil)
}

// RequestLeadsExport exports the leads the caller may read in the active account (TEC-552,
// TEC-940). The account and the viewer come from the token, never from the body. Each export
// takes one from the account's monthly quota: 403 PLAN_LIMIT_EXCEEDED once it is used up, and
// locked leads are masked in the file (TEC-941).
func (h *Handler) RequestLeadsExport(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	h.requestExportWith(w, r, "app.leads", rbac.PermAppLeadsRead, ioengine.ExportQuery{
		"account_id":     strconv.FormatInt(p.Account.ID, 10),
		"viewer_user_id": strconv.FormatInt(p.UserInternal, 10),
	}, h.prepareLeadsExport)
}

// prepareLeadsExport runs once the request is valid, so a rejected request uses no quota.
func (h *Handler) prepareLeadsExport(ctx context.Context, p authctx.Principal) error {
	if h.leads == nil {
		return &entitlement.QuotaError{Feature: entitlement.FeatureLeadsExportMonthlyMax}
	}
	return h.leads.PrepareExport(ctx, p)
}

func (h *Handler) requestExport(w http.ResponseWriter, r *http.Request, resource, perm string) {
	h.requestExportWith(w, r, resource, perm, nil, nil)
}

// requestExportWith queues an export; fixed query keys win over the body and the URL. prepare,
// when set, runs after the request is validated and before the job is queued.
func (h *Handler) requestExportWith(
	w http.ResponseWriter, r *http.Request, resource, perm string, fixed ioengine.ExportQuery,
	prepare func(context.Context, authctx.Principal) error,
) {
	p := authctx.MustPrincipal(r.Context())
	if !p.HasPermission(perm) {
		response.Forbidden(w, r, "export not allowed")
		return
	}
	var in struct {
		Format string            `json:"format"`
		Query  map[string]string `json:"query"`
		Locale string            `json:"locale"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}
	format := ioengine.ExportFormat(strings.ToLower(strings.TrimSpace(in.Format)))
	if in.Locale == "" {
		in.Locale = "tr"
	}
	query := ioengine.ExportQuery(in.Query)
	if query == nil {
		query = ioengine.ExportQuery{}
	}
	for k, vals := range r.URL.Query() {
		if k == "q" || k == "status" || k == "role" || k == "channel" || k == "scope" || k == "user_uuid" {
			if len(vals) > 0 {
				query[k] = vals[0]
			}
		}
	}
	if resource == "platform.notifications" {
		constrainNotificationExportQuery(p, query)
	}
	for k, v := range fixed {
		query[k] = v
	}
	if prepare != nil {
		if !validFormat(format) {
			response.BadRequest(w, r, response.CodeValidationError, "invalid format")
			return
		}
		if err := prepare(r.Context(), p); err != nil {
			if !entitlement.WriteError(w, r, err) {
				response.InternalErr(w, r, err, "failed to prepare export")
			}
			return
		}
	}
	job, err := h.svc.RequestExport(r.Context(), p.UserInternal, resource, format, query, in.Locale)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusAccepted, job)
}

func validFormat(f ioengine.ExportFormat) bool {
	return f == ioengine.ExportPDF || f == ioengine.ExportXLSX || f == ioengine.ExportCSV || f == ioengine.ExportJSON
}

func writeErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, exportusecase.ErrNotFound):
		response.NotFound(w, r, "export job not found")
	case errors.Is(err, exportusecase.ErrForbidden):
		response.Forbidden(w, r, "forbidden")
	case errors.Is(err, exportusecase.ErrInvalidRequest):
		response.BadRequest(w, r, response.CodeValidationError, err.Error())
	default:
		response.InternalErr(w, r, err, "unexpected error")
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := dec.Decode(dst); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid body")
		return err
	}
	return nil
}

func constrainNotificationExportQuery(p authctx.Principal, query ioengine.ExportQuery) {
	if !p.HasPermission(rbac.PermPlatformNotificationsReadAll) {
		query["user_uuid"] = p.UserID.String()
		delete(query, "scope")
		return
	}
	if strings.TrimSpace(query["user_uuid"]) != "" {
		delete(query, "scope")
		return
	}
	if strings.EqualFold(strings.TrimSpace(query["scope"]), "all") {
		delete(query, "user_uuid")
		return
	}
	query["user_uuid"] = p.UserID.String()
	delete(query, "scope")
}
