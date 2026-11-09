package handler

import (
	"errors"
	"net/http"

	billingusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/billing/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/i18n"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/apiquery"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
	"github.com/google/uuid"
)

// ListPayments returns the active account's payment history (TEC-590).
func (h *Handler) ListPayments(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	if bad := apiquery.ForbiddenParams(r.URL.Query()); len(bad) > 0 {
		response.BadRequest(w, r, response.CodeValidationError, "unsupported query parameter: "+bad[0])
		return
	}
	q := apiquery.Parse(r.URL.Query())
	limit, offset := apiquery.Clamp(q.Limit, q.Offset, 20, 100)
	out, err := h.svc.ListPayments(r.Context(), p.Account, limit, offset)
	if err != nil {
		writeCheckoutError(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, out)
}

// DownloadReceipt streams the PDF receipt of one payment of the active account (TEC-590).
func (h *Handler) DownloadReceipt(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}
	locale := r.URL.Query().Get("locale")
	if locale == "" {
		locale = r.Header.Get("Accept-Language")
	}
	locale = string(i18n.Normalize(locale))
	receipt, err := h.svc.ReceiptPDF(r.Context(), p.Account, id, locale)
	if err != nil {
		if errors.Is(err, billingusecase.ErrNotFound) {
			response.NotFound(w, r, "payment not found")
			return
		}
		writeCheckoutError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="`+receipt.Filename+`"`)
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(receipt.PDF)
}
