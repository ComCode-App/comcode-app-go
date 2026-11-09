package handler

import (
	"errors"
	"net/http"

	billingusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/billing/usecase"
	ordersusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/payment"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
	"github.com/google/uuid"
)

// Error codes of the order refund routes (TEC-935).
const (
	CodeRefundFailed     = "REFUND_FAILED"
	CodeRefundInProgress = "REFUND_IN_PROGRESS"
	CodeNothingToRefund  = "NOTHING_TO_REFUND"
)

// ListOrderRefunds returns an order's online payments and their refunds (platform).
func (h *Handler) ListOrderRefunds(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}
	out, err := h.svc.ListOrderRefunds(r.Context(), id)
	if err != nil {
		WriteRefundError(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, out)
}

// RefundOrder refunds an online payment of the order at its provider (platform, step-up).
func (h *Handler) RefundOrder(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}
	var in billingusecase.RefundOrderInput
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}
	out, created, err := h.svc.RefundOrder(r.Context(), id, in, clientIP(r))
	if err != nil {
		WriteRefundError(w, r, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	response.JSON(w, r, status, out)
}

// RetryOrderRefund sends a failed refund to the provider again (platform, step-up).
func (h *Handler) RetryOrderRefund(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}
	refundID, err := uuid.Parse(r.PathValue("refundUuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid refund uuid")
		return
	}
	out, err := h.svc.RetryOrderRefund(r.Context(), id, refundID, clientIP(r))
	if err != nil {
		WriteRefundError(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, out)
}

// WriteRefundError maps refund errors; the orders module uses it for status changes to refunded.
func WriteRefundError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, ordersusecase.ErrRefundFailed):
		response.Error(w, r, http.StatusBadGateway, CodeRefundFailed, err.Error())
	case errors.Is(err, ordersusecase.ErrRefundInProgress):
		response.Conflict(w, r, CodeRefundInProgress, err.Error())
	case errors.Is(err, billingusecase.ErrNothingToRefund):
		response.Conflict(w, r, CodeNothingToRefund, err.Error())
	case errors.Is(err, payment.ErrNotConfigured):
		response.ServiceUnavailable(w, r, "", "online payment is not configured")
	case errors.Is(err, billingusecase.ErrNotFound):
		response.NotFound(w, r, "order or refund not found")
	case errors.Is(err, billingusecase.ErrInvalidRequest):
		response.BadRequest(w, r, response.CodeValidationError, err.Error())
	default:
		response.InternalErr(w, r, err, "refund failed")
	}
}
