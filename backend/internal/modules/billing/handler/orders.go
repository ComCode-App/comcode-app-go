package handler

import (
	"errors"
	"net/http"

	billingusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/billing/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/payment"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
	"github.com/google/uuid"
)

// StartOrderCheckout starts an online payment for a pending order of the active account (TEC-943).
func (h *Handler) StartOrderCheckout(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}
	out, err := h.svc.StartOrderCheckout(r.Context(), payerFrom(p), id, clientIP(r))
	if err != nil {
		writeOrderCheckoutError(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusCreated, out)
}

func (h *Handler) GetOrderCheckout(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}
	out, err := h.svc.GetOrderCheckout(r.Context(), payerFrom(p), id)
	if err != nil {
		writeOrderCheckoutError(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, out)
}

func writeOrderCheckoutError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, billingusecase.ErrOrderNotPayable):
		response.Conflict(w, r, "", "order is not payable")
	case errors.Is(err, payment.ErrNotConfigured):
		response.ServiceUnavailable(w, r, "", "online payment is not configured")
	default:
		writeCheckoutError(w, r, err)
	}
}
