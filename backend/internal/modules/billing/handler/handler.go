package handler

import (
	"encoding/json"
	"errors"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/clientip"
	"log/slog"
	"net/http"
	"strings"

	billingusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/billing/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
	"github.com/google/uuid"
)

type Handler struct {
	svc         *billingusecase.Service
	frontendURL string
	log         *slog.Logger
}

func New(svc *billingusecase.Service, frontendURL string, log *slog.Logger) *Handler {
	if log == nil {
		log = slog.Default()
	}
	return &Handler{
		svc:         svc,
		frontendURL: strings.TrimRight(frontendURL, "/"),
		log:         log.With("component", "billing.handler"),
	}
}

func (h *Handler) ListPlans(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListPlans(r.Context())
	if err != nil {
		response.InternalErr(w, r, err, "failed to list billing plans")
		return
	}
	response.JSON(w, r, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) StartCheckout(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	var in struct {
		PlanUUID uuid.UUID `json:"plan_uuid"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}
	if in.PlanUUID == uuid.Nil {
		response.BadRequest(w, r, response.CodeValidationError, "plan_uuid is required")
		return
	}
	out, err := h.svc.StartCheckout(r.Context(), payerFrom(p), in.PlanUUID, clientIP(r))
	if err != nil {
		writeCheckoutError(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusCreated, out)
}

func (h *Handler) GetCheckout(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}
	out, err := h.svc.GetCheckout(r.Context(), payerFrom(p), id)
	if err != nil {
		writeCheckoutError(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, out)
}

func (h *Handler) PayTRCallback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if err := r.ParseForm(); err != nil {
		h.log.Warn("paytr_callback_parse_failed", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("FAIL"))
		return
	}
	payload := map[string]string{}
	for k, vals := range r.PostForm {
		if len(vals) > 0 {
			payload[k] = vals[0]
		}
	}
	h.log.Info("paytr_callback_http", "method", r.Method, "payload_keys", mapKeys(payload))
	if err := h.svc.HandlePayTRCallback(r.Context(), payload); err != nil {
		h.log.Error("paytr_callback_handler_failed", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("FAIL"))
		return
	}
	h.log.Info("paytr_callback_http_ok")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

func (h *Handler) IyzicoCallback(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.log.Warn("iyzico_callback_parse_failed", "error", err)
		response.BadRequest(w, r, response.CodeValidationError, "invalid form")
		return
	}
	payload := map[string]string{}
	for k, vals := range r.PostForm {
		if len(vals) > 0 {
			payload[k] = vals[0]
		}
	}
	for k, vals := range r.URL.Query() {
		if _, ok := payload[k]; !ok && len(vals) > 0 {
			payload[k] = vals[0]
		}
	}
	if payload["merchant_oid"] == "" && payload["conversationId"] != "" {
		payload["merchant_oid"] = payload["conversationId"]
	}
	h.log.Info("iyzico_callback_http", "method", r.Method, "payload_keys", mapKeys(payload))
	outcome, err := h.svc.HandleIyzicoCallback(r.Context(), payload)
	if outcome.OrderSession != uuid.Nil {
		// Order payments (TEC-943) return to their result page, which reads the stored state.
		result := "success"
		if err != nil || !outcome.Success {
			h.log.Warn("iyzico_callback_order_payment_failed", "session_uuid", outcome.OrderSession, "error", err)
			result = "fail"
		}
		http.Redirect(w, r, h.frontendURL+"/orders/checkout/"+outcome.OrderSession.String()+"?result="+result, http.StatusFound)
		return
	}
	if err != nil {
		h.log.Error("iyzico_callback_handler_failed", "error", err)
		http.Redirect(w, r, h.frontendURL+"/billing?result=fail", http.StatusFound)
		return
	}
	if !outcome.Success {
		h.log.Warn("iyzico_callback_payment_failed", "is_card_session", outcome.IsCardSession)
		http.Redirect(w, r, h.frontendURL+"/billing?result=fail", http.StatusFound)
		return
	}
	if outcome.IsCardSession {
		h.log.Info("iyzico_callback_redirect", "target", "/billing")
		http.Redirect(w, r, h.frontendURL+"/billing", http.StatusFound)
		return
	}
	h.log.Info("iyzico_callback_redirect", "target", "/billing/success")
	http.Redirect(w, r, h.frontendURL+"/billing/success", http.StatusFound)
}

func mapKeys(payload map[string]string) []string {
	keys := make([]string, 0, len(payload))
	for k := range payload {
		keys = append(keys, k)
	}
	return keys
}

func writeCheckoutError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, billingusecase.ErrNotFound):
		response.NotFound(w, r, "resource not found")
	case errors.Is(err, billingusecase.ErrForbidden):
		response.Forbidden(w, r, "forbidden")
	case errors.Is(err, billingusecase.ErrInvalidRequest):
		response.BadRequest(w, r, response.CodeValidationError, err.Error())
	default:
		response.InternalErr(w, r, err, "checkout failed")
	}
}

func clientIP(r *http.Request) string {
	return clientip.String(r)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := dec.Decode(dst); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid body")
		return err
	}
	return nil
}
