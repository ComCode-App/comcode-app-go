// Package handler exposes the company media library and branding (TEC-946).
package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companybranding/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companybranding/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/upload"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/apiquery"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
	"github.com/google/uuid"
)

type Handler struct{ svc *usecase.Service }

func New(svc *usecase.Service) *Handler { return &Handler{svc: svc} }

func writeErr(w http.ResponseWriter, r *http.Request, err error) {
	if entitlement.WriteError(w, r, err) { // premium company theme without design.premium (TEC-947)
		return
	}
	if upload.WriteError(w, r, err) { // too large / unsafe SVG logo / unsupported type (TEC-1097)
		return
	}
	var inUse *model.InUseError
	switch {
	case errors.As(err, &inUse):
		details := make([]response.Detail, 0, len(inUse.Usage))
		for _, u := range inUse.Usage {
			d := response.Detail{Field: u.Type, Message: u.ProfileName, Code: "in_use"}
			if u.ProfileUUID != nil {
				d.Field = u.Type + ":" + u.ProfileUUID.String()
			}
			details = append(details, d)
		}
		response.ErrorWithDetails(w, r, http.StatusConflict, response.CodeConflict, "file is in use", details)
	case errors.Is(err, model.ErrForbidden):
		response.Forbidden(w, r, "company branding needs a company account and an owner/admin role for changes")
	case errors.Is(err, model.ErrNotFound):
		response.NotFound(w, r, "file not found")
	case errors.Is(err, model.ErrInvalid):
		response.BadRequest(w, r, response.CodeValidationError, err.Error())
	case errors.Is(err, model.ErrLimitReached):
		response.Conflict(w, r, response.CodeConflict, err.Error())
	default:
		response.InternalErr(w, r, err, "company branding operation failed")
	}
}

func (h *Handler) ListMedia(w http.ResponseWriter, r *http.Request) {
	q := apiquery.Parse(r.URL.Query())
	f := model.Filter{Kind: strings.TrimSpace(r.URL.Query().Get("kind")), Limit: q.Limit, Offset: q.Offset}
	items, total, err := h.svc.List(r.Context(), authctx.MustPrincipal(r.Context()), f)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if f.Limit <= 0 {
		f.Limit = 50
	}
	response.JSON(w, r, http.StatusOK, apiquery.NewPage(items, total, min(f.Limit, 100), max(f.Offset, 0)))
}

func (h *Handler) UploadMedia(w http.ResponseWriter, r *http.Request) {
	file, header, ok := upload.FormFile(w, r, model.MaxDocumentBytes, "file")
	defer upload.Cleanup(r)
	if !ok {
		return
	}
	defer func() { _ = file.Close() }()
	out, err := h.svc.Upload(r.Context(), authctx.MustPrincipal(r.Context()), model.UploadInput{
		Kind: strings.TrimSpace(r.FormValue("kind")), Name: r.FormValue("name"),
		Filename: header.Filename, Size: header.Size,
	}, file)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusCreated, out)
}

func (h *Handler) DeleteMedia(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}
	if err := h.svc.Delete(r.Context(), authctx.MustPrincipal(r.Context()), id); err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *Handler) GetBranding(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.GetBranding(r.Context(), authctx.MustPrincipal(r.Context()))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, out)
}

func (h *Handler) UpdateBranding(w http.ResponseWriter, r *http.Request) {
	var in model.UpdateBrandingInput
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid body")
		return
	}
	out, err := h.svc.UpdateBranding(r.Context(), authctx.MustPrincipal(r.Context()), in)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, out)
}
