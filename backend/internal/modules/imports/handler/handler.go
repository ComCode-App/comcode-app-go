package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	cpmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyprofiles/model"
	importusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/imports/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/ioengine"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/apiquery"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
	"github.com/google/uuid"
)

type Handler struct {
	svc *importusecase.Service
}

func New(svc *importusecase.Service) *Handler {
	return &Handler{svc: svc}
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

func (h *Handler) UploadUsers(w http.ResponseWriter, r *http.Request) {
	h.upload(w, r, "platform.users")
}

func (h *Handler) UploadRoles(w http.ResponseWriter, r *http.Request) {
	h.upload(w, r, "platform.roles")
}

func (h *Handler) upload(w http.ResponseWriter, r *http.Request, resource string) {
	h.uploadFor(w, r, resource, 0)
}

func (h *Handler) uploadFor(w http.ResponseWriter, r *http.Request, resource string, accountID int64) {
	p := authctx.MustPrincipal(r.Context())
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid multipart form")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "file is required")
		return
	}
	defer func() { _ = file.Close() }()
	format := ioengine.ImportFormat(strings.ToLower(strings.TrimSpace(r.FormValue("format"))))
	if format == "" {
		format = ioengine.ImportCSV
	}
	locale := strings.TrimSpace(r.FormValue("locale"))
	if locale == "" {
		locale = "tr"
	}
	job, err := h.svc.Upload(r.Context(), p.UserInternal, accountID, resource, format, locale, header.Filename, file)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusCreated, job)
}

func (h *Handler) SampleUsers(w http.ResponseWriter, r *http.Request) {
	h.sample(w, r, "platform.users")
}

func (h *Handler) SampleRoles(w http.ResponseWriter, r *http.Request) {
	h.sample(w, r, "platform.roles")
}

func (h *Handler) sample(w http.ResponseWriter, r *http.Request, resource string) {
	format := ioengine.ImportFormat(strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format"))))
	if format == "" {
		format = ioengine.ImportXLSX
	}
	locale := strings.TrimSpace(r.URL.Query().Get("locale"))
	if locale == "" {
		locale = "tr"
	}
	data, ct, err := h.svc.Sample(r.Context(), resource, format, locale)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", "attachment; filename=\"sample."+string(format)+"\"")
	_, _ = w.Write(data)
}

func (h *Handler) UpdateMapping(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "uuid is invalid")
		return
	}
	var in struct {
		Mapping  map[string]string `json:"mapping"`
		Defaults map[string]string `json:"defaults"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}
	job, err := h.svc.UpdateMapping(r.Context(), id, p.UserInternal, accountOf(r), in.Mapping, in.Defaults)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, job)
}

func (h *Handler) Preview(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "uuid is invalid")
		return
	}
	job, err := h.svc.Preview(r.Context(), id, p.UserInternal, accountOf(r))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, job)
}

func (h *Handler) Confirm(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "uuid is invalid")
		return
	}
	job, err := h.svc.Confirm(r.Context(), id, p.UserInternal, accountOf(r))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusAccepted, job)
}

func (h *Handler) Rollback(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "uuid is invalid")
		return
	}
	job, err := h.svc.Rollback(r.Context(), id, p.UserInternal, accountOf(r))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, job)
}

type accountKey struct{}

// accountOf is the account the job routes act in: 0 on platform routes, the active company
// account on company routes (set by RequireCompanyImporter).
func accountOf(r *http.Request) int64 {
	id, _ := r.Context().Value(accountKey{}).(int64)
	return id
}

// RequireCompanyImporter admits owners/admins of a company account to the company import routes
// and makes the job routes act on that account's jobs only.
func RequireCompanyImporter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := authctx.MustPrincipal(r.Context())
		if p.Account.ID <= 0 || p.Account.Type != authctx.AccountTypeCompany || !p.Account.CanWrite() {
			response.Forbidden(w, r, "Company owner or admin role required")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), accountKey{}, p.Account.ID)))
	})
}

// UploadCompanyEmployees starts an employee import in the active company (TEC-573).
func (h *Handler) UploadCompanyEmployees(w http.ResponseWriter, r *http.Request) {
	h.uploadFor(w, r, cpmodel.ResourceCompanyEmployees, accountOf(r))
}

// SampleCompanyEmployees downloads the employee import sample.
func (h *Handler) SampleCompanyEmployees(w http.ResponseWriter, r *http.Request) {
	h.sample(w, r, cpmodel.ResourceCompanyEmployees)
}

// CompanyList lists the caller's import jobs in the active company.
func (h *Handler) CompanyList(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	q := apiquery.Parse(r.URL.Query())
	items, total, err := h.svc.ListAccountJobs(r.Context(), p.UserInternal, accountOf(r), cpmodel.ResourceCompanyEmployees, q.Limit, q.Offset)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, apiquery.NewPage(items, total, q.Limit, q.Offset))
}

// CompanyGet returns one of the caller's import jobs in the active company.
func (h *Handler) CompanyGet(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "uuid is invalid")
		return
	}
	job, err := h.svc.GetAccountJob(r.Context(), id, p.UserInternal, accountOf(r))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, job)
}

func writeErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, importusecase.ErrNotFound):
		response.NotFound(w, r, "import job not found")
	case errors.Is(err, importusecase.ErrForbidden):
		response.Forbidden(w, r, "forbidden")
	case errors.Is(err, importusecase.ErrInvalidRequest):
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

// suppress unused io import when build tags differ
var _ = io.EOF
