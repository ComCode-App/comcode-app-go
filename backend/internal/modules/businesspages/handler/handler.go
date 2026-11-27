// Package handler exposes ComCode Business pages (TEC-1327).
package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/businesspages/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/businesspages/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
)

type Handler struct{ svc *usecase.Service }

func New(svc *usecase.Service) *Handler { return &Handler{svc: svc} }

func writeErr(w http.ResponseWriter, r *http.Request, err error) {
	if entitlement.WriteError(w, r, err) { // business_page.enabled missing from the plan
		return
	}
	switch {
	case errors.Is(err, model.ErrForbidden):
		response.Forbidden(w, r, "the business page needs a company account and an owner/admin role for changes")
	case errors.Is(err, model.ErrNotFound):
		response.NotFound(w, r, "business page not found")
	case errors.Is(err, model.ErrUsernameTaken):
		response.Conflict(w, r, response.CodeConflict, err.Error())
	case errors.Is(err, model.ErrInvalid):
		response.BadRequest(w, r, response.CodeValidationError, err.Error())
	default:
		response.InternalErr(w, r, err, "business page operation failed")
	}
}

// Get returns the active company's business page (404 until the first save).
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Get(r.Context(), authctx.MustPrincipal(r.Context()))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, out)
}

// Save replaces the page settings and blocks.
func (h *Handler) Save(w http.ResponseWriter, r *http.Request) {
	var in model.SaveInput
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<18))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid body")
		return
	}
	out, err := h.svc.Save(r.Context(), authctx.MustPrincipal(r.Context()), in)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, out)
}

// Publish makes the page visible at /{username}.
func (h *Handler) Publish(w http.ResponseWriter, r *http.Request) { h.setPublished(w, r, true) }

// Unpublish turns the page back into a draft (404 for visitors).
func (h *Handler) Unpublish(w http.ResponseWriter, r *http.Request) { h.setPublished(w, r, false) }

func (h *Handler) setPublished(w http.ResponseWriter, r *http.Request, published bool) {
	out, err := h.svc.SetPublished(r.Context(), authctx.MustPrincipal(r.Context()), published)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, out)
}

// Public is the visitor view of a published business page (no auth).
func (h *Handler) Public(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Public(r.Context(), r.PathValue("username"))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=30")
	response.JSON(w, r, http.StatusOK, out)
}
