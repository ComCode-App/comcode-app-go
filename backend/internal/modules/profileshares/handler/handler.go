// Package handler serves profile sharing with companies (TEC-1315).
package handler

import (
	"errors"
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profileshares/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/apiquery"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
	"github.com/google/uuid"
)

type Handler struct{ svc *usecase.Service }

func New(svc *usecase.Service) *Handler { return &Handler{svc: svc} }

func writeErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, usecase.ErrForbidden):
		response.Forbidden(w, r, "Forbidden")
	case errors.Is(err, usecase.ErrNotFound):
		response.NotFound(w, r, "Not found")
	default:
		response.InternalErr(w, r, err, "profile share operation failed")
	}
}

func pathUUID(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue(name))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid "+name)
		return uuid.Nil, false
	}
	return id, true
}

// Targets lists the caller's companies with the share state of the profile.
func (h *Handler) Targets(w http.ResponseWriter, r *http.Request) {
	profileUUID, ok := pathUUID(w, r, "uuid")
	if !ok {
		return
	}
	items, err := h.svc.Targets(r.Context(), authctx.MustPrincipal(r.Context()), profileUUID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) set(w http.ResponseWriter, r *http.Request, shared bool) {
	profileUUID, ok := pathUUID(w, r, "uuid")
	if !ok {
		return
	}
	companyUUID, ok := pathUUID(w, r, "company_uuid")
	if !ok {
		return
	}
	if err := h.svc.SetShared(r.Context(), authctx.MustPrincipal(r.Context()), profileUUID, companyUUID, shared, r); err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, map[string]any{"shared": shared})
}

// Share shares the profile with the company.
func (h *Handler) Share(w http.ResponseWriter, r *http.Request) { h.set(w, r, true) }

// Unshare stops sharing the profile with the company.
func (h *Handler) Unshare(w http.ResponseWriter, r *http.Request) { h.set(w, r, false) }

// CompanyProfiles lists the profiles members shared with the active company (read-only).
func (h *Handler) CompanyProfiles(w http.ResponseWriter, r *http.Request) {
	q := apiquery.Parse(r.URL.Query())
	items, total, err := h.svc.CompanyProfiles(r.Context(), authctx.MustPrincipal(r.Context()), q.Limit, q.Offset)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, apiquery.NewPage(items, total, q.Limit, q.Offset))
}
