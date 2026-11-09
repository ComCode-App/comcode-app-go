// Package handler exposes the webhook endpoints API (TEC-603).
package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/webhooks/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/webhooks/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/activity"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/apiquery"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
	"github.com/google/uuid"
)

// Error codes of the webhook routes.
const (
	CodeWebhookLimit    = "WEBHOOK_LIMIT_REACHED"
	CodeDeliveryPending = "DELIVERY_PENDING"
)

type Handler struct {
	svc      *usecase.Service
	activity *activity.Recorder
}

func New(svc *usecase.Service) *Handler { return &Handler{svc: svc} }

// SetActivity records endpoint changes in the activity log.
func (h *Handler) SetActivity(a *activity.Recorder) { h.activity = a }

func (h *Handler) Events(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, r, http.StatusOK, map[string]any{"items": h.svc.Events()})
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	p, ok := principal(w, r)
	if !ok {
		return
	}
	items, err := h.svc.List(r.Context(), p.Account)
	if err != nil {
		response.InternalErr(w, r, err, "failed to list webhooks")
		return
	}
	response.JSON(w, r, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	p, ok := principal(w, r)
	if !ok {
		return
	}
	id, ok := pathUUID(w, r, "uuid")
	if !ok {
		return
	}
	item, err := h.svc.Get(r.Context(), p.Account, id)
	if err != nil {
		writeError(w, r, err, "failed to load webhook")
		return
	}
	response.JSON(w, r, http.StatusOK, item)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	p, ok := principal(w, r)
	if !ok {
		return
	}
	var in model.EndpointInput
	if err := decode(w, r, &in); err != nil {
		return
	}
	out, err := h.svc.Create(r.Context(), p.Account, p.UserInternal, in)
	if err != nil {
		writeError(w, r, err, "failed to create webhook")
		return
	}
	h.record(r, p, "webhook.created", out.UUID, map[string]any{"url": out.URL, "events": out.Events})
	w.Header().Set("Cache-Control", "private, no-store")
	response.JSON(w, r, http.StatusCreated, out)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	p, ok := principal(w, r)
	if !ok {
		return
	}
	id, ok := pathUUID(w, r, "uuid")
	if !ok {
		return
	}
	var in model.EndpointInput
	if err := decode(w, r, &in); err != nil {
		return
	}
	out, err := h.svc.Update(r.Context(), p.Account, id, in)
	if err != nil {
		writeError(w, r, err, "failed to update webhook")
		return
	}
	h.record(r, p, "webhook.updated", out.UUID, map[string]any{"url": out.URL, "events": out.Events, "is_active": out.IsActive})
	response.JSON(w, r, http.StatusOK, out)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	p, ok := principal(w, r)
	if !ok {
		return
	}
	id, ok := pathUUID(w, r, "uuid")
	if !ok {
		return
	}
	if err := h.svc.Delete(r.Context(), p.Account, id); err != nil {
		writeError(w, r, err, "failed to delete webhook")
		return
	}
	h.record(r, p, "webhook.deleted", id, nil)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) RotateSecret(w http.ResponseWriter, r *http.Request) {
	p, ok := principal(w, r)
	if !ok {
		return
	}
	id, ok := pathUUID(w, r, "uuid")
	if !ok {
		return
	}
	out, err := h.svc.RotateSecret(r.Context(), p.Account, id)
	if err != nil {
		writeError(w, r, err, "failed to rotate webhook secret")
		return
	}
	h.record(r, p, "webhook.secret_rotated", out.UUID, nil)
	w.Header().Set("Cache-Control", "private, no-store")
	response.JSON(w, r, http.StatusOK, out)
}

func (h *Handler) SendTest(w http.ResponseWriter, r *http.Request) {
	p, ok := principal(w, r)
	if !ok {
		return
	}
	id, ok := pathUUID(w, r, "uuid")
	if !ok {
		return
	}
	out, err := h.svc.SendTest(r.Context(), p.Account, id)
	if err != nil {
		writeError(w, r, err, "failed to send test webhook")
		return
	}
	response.JSON(w, r, http.StatusAccepted, out)
}

func (h *Handler) ListDeliveries(w http.ResponseWriter, r *http.Request) {
	p, ok := principal(w, r)
	if !ok {
		return
	}
	id, ok := pathUUID(w, r, "uuid")
	if !ok {
		return
	}
	if bad := apiquery.ForbiddenParams(r.URL.Query()); len(bad) > 0 {
		response.BadRequest(w, r, response.CodeValidationError, "unsupported query parameter: "+bad[0])
		return
	}
	q := apiquery.Parse(r.URL.Query())
	limit, offset := apiquery.Clamp(q.Limit, q.Offset, 20, 100)
	status := r.URL.Query().Get("status")
	switch status {
	case "", "pending", "succeeded", "failed", "exhausted":
	default:
		response.BadRequest(w, r, response.CodeValidationError, "invalid status")
		return
	}
	out, err := h.svc.ListDeliveries(r.Context(), p.Account, id, status, limit, offset)
	if err != nil {
		writeError(w, r, err, "failed to list deliveries")
		return
	}
	response.JSON(w, r, http.StatusOK, out)
}

func (h *Handler) ResendDelivery(w http.ResponseWriter, r *http.Request) {
	p, ok := principal(w, r)
	if !ok {
		return
	}
	id, ok := pathUUID(w, r, "uuid")
	if !ok {
		return
	}
	did, ok := pathUUID(w, r, "deliveryUuid")
	if !ok {
		return
	}
	out, err := h.svc.Resend(r.Context(), p.Account, id, did)
	if err != nil {
		writeError(w, r, err, "failed to resend delivery")
		return
	}
	response.JSON(w, r, http.StatusAccepted, out)
}

func (h *Handler) record(r *http.Request, p authctx.Principal, action string, id uuid.UUID, payload map[string]any) {
	if h.activity == nil {
		return
	}
	actor := p.UserInternal
	h.activity.Record(r.Context(), &actor, action, "webhook", &id, payload, r)
}

func principal(w http.ResponseWriter, r *http.Request) (authctx.Principal, bool) {
	p := authctx.MustPrincipal(r.Context())
	if p.Account.ID == 0 {
		response.Unauthorized(w, r, "unauthorized")
		return p, false
	}
	return p, true
}

func pathUUID(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue(name))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return uuid.Nil, false
	}
	return id, true
}

func decode(w http.ResponseWriter, r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	if err := dec.Decode(dst); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid request body")
		return err
	}
	return nil
}

func writeError(w http.ResponseWriter, r *http.Request, err error, fallback string) {
	switch {
	case errors.Is(err, usecase.ErrNotFound):
		response.NotFound(w, r, "webhook not found")
	case errors.Is(err, usecase.ErrForbidden):
		response.Forbidden(w, r, "your account role cannot manage webhooks")
	case errors.Is(err, usecase.ErrInvalidURL), errors.Is(err, usecase.ErrInvalidEvents):
		response.BadRequest(w, r, response.CodeValidationError, err.Error())
	case errors.Is(err, usecase.ErrTooMany):
		response.Conflict(w, r, CodeWebhookLimit, err.Error())
	case errors.Is(err, usecase.ErrDeliveryActive):
		response.Conflict(w, r, CodeDeliveryPending, err.Error())
	default:
		response.InternalErr(w, r, err, fallback)
	}
}
