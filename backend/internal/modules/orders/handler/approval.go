package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
	"github.com/google/uuid"
)

// RejectOrderInput is the body of the reject route (TEC-1317).
type RejectOrderInput struct {
	Reason string `json:"reason"`
}

func writeReviewError(w http.ResponseWriter, r *http.Request, err error) {
	var quota *usecase.QuotaExceededError
	switch {
	case errors.Is(err, usecase.ErrNotFound):
		response.NotFound(w, r, "order not found")
	case errors.Is(err, usecase.ErrNotAwaitingApproval):
		response.Conflict(w, r, "ORDER_NOT_AWAITING_APPROVAL", err.Error())
	case errors.Is(err, usecase.ErrRejectionReasonRequired):
		response.BadRequest(w, r, response.CodeValidationError, err.Error())
	case errors.As(err, &quota):
		response.ErrorWithDetails(w, r, http.StatusConflict, "PROFILE_QUOTA_EXCEEDED", quota.Error(), []response.Detail{
			{Field: "missing", Code: "missing", Message: strconv.FormatInt(quota.Missing, 10)},
			{Field: "used", Code: "used", Message: strconv.FormatInt(quota.Used, 10)},
			{Field: "limit", Code: "limit", Message: strconv.FormatInt(int64(quota.Limit), 10)},
		})
	default:
		response.InternalErr(w, r, err, "failed to review order")
	}
}

func (h *Handler) recordReview(r *http.Request, ord *model.OrderDetail, action string, extra map[string]any) {
	if h.activity == nil || ord == nil {
		return
	}
	payload := map[string]any{"order_number": ord.OrderNumber, "status": ord.Status}
	for k, v := range extra {
		payload[k] = v
	}
	var actor *int64
	if p, ok := authctx.PrincipalFrom(r.Context()); ok && p.UserInternal > 0 {
		v := p.UserInternal
		actor = &v
	}
	id, err := uuid.Parse(ord.UUID)
	if err != nil {
		return
	}
	h.activity.Record(r.Context(), actor, action, "orders", &id, payload, r)
}

// ApproveOrder approves a company bulk order awaiting approval; the customer can then pay.
func (h *Handler) ApproveOrder(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}
	ord, err := h.svc.ApproveOrder(r.Context(), id)
	if err != nil {
		writeReviewError(w, r, err)
		return
	}
	h.recordReview(r, ord, "order.approved", nil)
	response.JSON(w, r, http.StatusOK, ord)
}

// RejectOrder rejects a company bulk order awaiting approval with a required reason.
func (h *Handler) RejectOrder(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}
	var in RejectOrderInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid request body")
		return
	}
	ord, err := h.svc.RejectOrder(r.Context(), id, in.Reason)
	if err != nil {
		writeReviewError(w, r, err)
		return
	}
	h.recordReview(r, ord, "order.rejected", map[string]any{"reason": ord.RejectionReason})
	response.JSON(w, r, http.StatusOK, ord)
}
