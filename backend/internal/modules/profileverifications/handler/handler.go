// Package handler serves the verified profile badge routes (TEC-549): the owner's
// /v1/profiles/{uuid}/verification and the platform review queue under
// /v1/platform/profile-verifications.
package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profileverifications/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profileverifications/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/apiquery"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
)

// Handler serves verification routes.
type Handler struct {
	svc *usecase.Service
}

// New creates the handler.
func New(svc *usecase.Service) *Handler { return &Handler{svc: svc} }

// OwnerGet answers GET /v1/profiles/{uuid}/verification.
func (h *Handler) OwnerGet(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r)
	if !ok {
		return
	}
	out, err := h.svc.OwnerGet(r.Context(), authctx.MustPrincipal(r.Context()), id)
	if err != nil {
		writeErr(w, r, err, "failed to load verification")
		return
	}
	response.JSON(w, r, http.StatusOK, out)
}

// Request answers POST /v1/profiles/{uuid}/verification.
func (h *Handler) Request(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r)
	if !ok {
		return
	}
	var in model.RequestInput
	if !decodeOptional(w, r, &in) {
		return
	}
	out, err := h.svc.Request(r.Context(), authctx.MustPrincipal(r.Context()), id, in)
	if err != nil {
		writeErr(w, r, err, "failed to request verification")
		return
	}
	response.JSON(w, r, http.StatusCreated, out)
}

// Cancel answers DELETE /v1/profiles/{uuid}/verification.
func (h *Handler) Cancel(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r)
	if !ok {
		return
	}
	out, err := h.svc.Cancel(r.Context(), authctx.MustPrincipal(r.Context()), id)
	if err != nil {
		writeErr(w, r, err, "failed to cancel verification request")
		return
	}
	response.JSON(w, r, http.StatusOK, out)
}

// List answers GET /v1/platform/profile-verifications.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := apiquery.Parse(r.URL.Query())
	items, total, err := h.svc.List(r.Context(), model.ListFilter{
		Status: r.URL.Query().Get("status"), Q: q.Q, Limit: q.Limit, Offset: q.Offset,
	})
	if err != nil {
		writeErr(w, r, err, "failed to list verification requests")
		return
	}
	response.JSON(w, r, http.StatusOK, apiquery.NewPage(items, total, q.Limit, q.Offset))
}

// Get answers GET /v1/platform/profile-verifications/{uuid}.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r)
	if !ok {
		return
	}
	out, err := h.svc.Get(r.Context(), id)
	if err != nil {
		writeErr(w, r, err, "failed to load verification request")
		return
	}
	response.JSON(w, r, http.StatusOK, out)
}

// Approve answers POST /v1/platform/profile-verifications/{uuid}/approve.
func (h *Handler) Approve(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r)
	if !ok {
		return
	}
	out, err := h.svc.Approve(r.Context(), authctx.MustPrincipal(r.Context()), id)
	if err != nil {
		writeErr(w, r, err, "failed to approve verification request")
		return
	}
	response.JSON(w, r, http.StatusOK, out)
}

// Reject answers POST /v1/platform/profile-verifications/{uuid}/reject.
func (h *Handler) Reject(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r)
	if !ok {
		return
	}
	var in model.ReasonInput
	if !decodeOptional(w, r, &in) {
		return
	}
	out, err := h.svc.Reject(r.Context(), authctx.MustPrincipal(r.Context()), id, in.Reason)
	if err != nil {
		writeErr(w, r, err, "failed to reject verification request")
		return
	}
	response.JSON(w, r, http.StatusOK, out)
}

// Revoke answers POST /v1/platform/profile-verifications/{uuid}/revoke.
func (h *Handler) Revoke(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r)
	if !ok {
		return
	}
	var in model.ReasonInput
	if !decodeOptional(w, r, &in) {
		return
	}
	out, err := h.svc.Revoke(r.Context(), authctx.MustPrincipal(r.Context()), id, in.Reason)
	if err != nil {
		writeErr(w, r, err, "failed to revoke verification")
		return
	}
	response.JSON(w, r, http.StatusOK, out)
}

func pathUUID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return uuid.Nil, false
	}
	return id, true
}

// decodeOptional reads a small JSON body; an empty body leaves dst zero.
func decodeOptional(w http.ResponseWriter, r *http.Request, dst any) bool {
	if r.Body == nil || r.ContentLength == 0 {
		return true
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(dst); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid JSON body")
		return false
	}
	return true
}

func writeErr(w http.ResponseWriter, r *http.Request, err error, msg string) {
	switch {
	case errors.Is(err, usecase.ErrNotFound):
		response.NotFound(w, r, "not found")
	case errors.Is(err, usecase.ErrForbidden):
		response.Forbidden(w, r, "your account role cannot change profile verification")
	case errors.Is(err, usecase.ErrInvalidRequest):
		response.BadRequest(w, r, response.CodeValidationError, err.Error())
	case errors.Is(err, usecase.ErrConflict):
		response.Conflict(w, r, response.CodeConflict, err.Error())
	default:
		response.InternalErr(w, r, err, msg)
	}
}
