package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/backgrounds/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/backgrounds/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
	"github.com/google/uuid"
)

type Handler struct {
	svc *usecase.Service
}

func New(svc *usecase.Service) *Handler { return &Handler{svc: svc} }

func writeErr(w http.ResponseWriter, r *http.Request, err error) {
	if entitlement.WriteError(w, r, err) { // premium design without design.premium (TEC-947)
		return
	}
	switch {
	case errors.Is(err, usecase.ErrNotFound):
		response.NotFound(w, r, err.Error())
	case errors.Is(err, usecase.ErrForbidden):
		response.Forbidden(w, r, "your account role cannot change backgrounds")
	case errors.Is(err, usecase.ErrInvalidRequest):
		response.BadRequest(w, r, response.CodeValidationError, err.Error())
	case errors.Is(err, usecase.ErrLimitReached):
		response.Conflict(w, r, response.CodeConflict, err.Error())
	default:
		response.InternalErr(w, r, err, "background operation failed")
	}
}

func decode(w http.ResponseWriter, r *http.Request) (model.UpsertBackgroundInput, bool) {
	var in model.UpsertBackgroundInput
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, model.MaxDocumentBytes+4096))
	if err := dec.Decode(&in); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid body")
		return in, false
	}
	return in, true
}

func pathUUID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return uuid.Nil, false
	}
	return id, true
}

// ---- App: active account ----

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.List(r.Context(), authctx.MustPrincipal(r.Context()))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	in, ok := decode(w, r)
	if !ok {
		return
	}
	out, err := h.svc.Create(r.Context(), authctx.MustPrincipal(r.Context()), in)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusCreated, out)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r)
	if !ok {
		return
	}
	in, ok := decode(w, r)
	if !ok {
		return
	}
	out, err := h.svc.Update(r.Context(), authctx.MustPrincipal(r.Context()), id, in)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, out)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r)
	if !ok {
		return
	}
	if err := h.svc.Delete(r.Context(), authctx.MustPrincipal(r.Context()), id); err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, map[string]string{"status": "deleted"})
}

// ---- Platform presets ----

func (h *Handler) ListPresets(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListPresets(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) CreatePreset(w http.ResponseWriter, r *http.Request) {
	in, ok := decode(w, r)
	if !ok {
		return
	}
	out, err := h.svc.CreatePreset(r.Context(), authctx.MustPrincipal(r.Context()), in)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusCreated, out)
}

func (h *Handler) UpdatePreset(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r)
	if !ok {
		return
	}
	in, ok := decode(w, r)
	if !ok {
		return
	}
	out, err := h.svc.UpdatePreset(r.Context(), id, in)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, out)
}

func (h *Handler) DeletePreset(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r)
	if !ok {
		return
	}
	if err := h.svc.DeletePreset(r.Context(), id); err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, map[string]string{"status": "deleted"})
}
