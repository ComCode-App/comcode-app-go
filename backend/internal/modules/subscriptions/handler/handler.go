package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	subscriptionsusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/subscriptions/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/resourcemeta"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/apiquery"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
	"github.com/google/uuid"
)

type Handler struct {
	svc *subscriptionsusecase.Service
}

func New(svc *subscriptionsusecase.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Meta(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, r, http.StatusOK, resourcemeta.PlatformSubscriptions())
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := apiquery.Parse(r.URL.Query())
	status := r.URL.Query().Get("status")
	items, total, err := h.svc.List(r.Context(), q.Limit, q.Offset, q.Q, status)
	if err != nil {
		response.InternalErr(w, r, err, "failed to list subscriptions")
		return
	}
	response.JSON(w, r, http.StatusOK, apiquery.NewPage(items, total, q.Limit, q.Offset))
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}
	item, err := h.svc.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, subscriptionsusecase.ErrNotFound) {
			response.NotFound(w, r, "subscription not found")
			return
		}
		response.InternalErr(w, r, err, "failed to load subscription")
		return
	}
	response.JSON(w, r, http.StatusOK, item)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		PlanUUID    uuid.UUID  `json:"plan_uuid"`
		Status      string     `json:"status"`
		UserUUID    *uuid.UUID `json:"user_uuid"`
		CompanyUUID *uuid.UUID `json:"company_uuid"`
		StartsAt    *time.Time `json:"starts_at"`
		EndsAt      *time.Time `json:"ends_at"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}
	item, err := h.svc.Create(r.Context(), subscriptionsusecase.CreateSubscriptionInput{
		PlanUUID: in.PlanUUID, Status: in.Status,
		UserUUID: in.UserUUID, CompanyUUID: in.CompanyUUID,
		StartsAt: in.StartsAt, EndsAt: in.EndsAt,
	})
	if err != nil {
		if errors.Is(err, subscriptionsusecase.ErrNotFound) || errors.Is(err, subscriptionsusecase.ErrInvalidRequest) {
			response.BadRequest(w, r, response.CodeValidationError, err.Error())
			return
		}
		response.InternalErr(w, r, err, "failed to create subscription")
		return
	}
	response.JSON(w, r, http.StatusCreated, item)
}

func (h *Handler) Patch(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}
	var in struct {
		PlanUUID *uuid.UUID `json:"plan_uuid"`
		Status   *string    `json:"status"`
		StartsAt *time.Time `json:"starts_at"`
		EndsAt   *time.Time `json:"ends_at"`
		ClearEnd *bool      `json:"clear_ends_at"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}
	patch := subscriptionsusecase.PatchSubscriptionInput{
		PlanUUID: in.PlanUUID, Status: in.Status, StartsAt: in.StartsAt,
	}
	if in.ClearEnd != nil && *in.ClearEnd {
		var nilTime *time.Time
		patch.EndsAt = &nilTime
	} else if in.EndsAt != nil {
		patch.EndsAt = &in.EndsAt
	}
	item, err := h.svc.Patch(r.Context(), id, patch)
	if err != nil {
		if errors.Is(err, subscriptionsusecase.ErrNotFound) {
			response.NotFound(w, r, "subscription not found")
			return
		}
		if errors.Is(err, subscriptionsusecase.ErrInvalidRequest) {
			response.BadRequest(w, r, response.CodeValidationError, err.Error())
			return
		}
		response.InternalErr(w, r, err, "failed to update subscription")
		return
	}
	response.JSON(w, r, http.StatusOK, item)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := dec.Decode(dst); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid body")
		return err
	}
	return nil
}
