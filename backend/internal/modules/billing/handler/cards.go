package handler

import (
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/billing/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
	"github.com/google/uuid"
)

// payerFrom builds the billing actor: the active account plus the calling member.
func payerFrom(p authctx.Principal) usecase.Payer {
	return usecase.Payer{Account: p.Account, UserID: p.UserInternal, Email: p.Email, Name: p.Email}
}

func (h *Handler) GetSubscription(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	sub, err := h.svc.GetSubscription(r.Context(), p.Account.ID)
	if err != nil {
		response.InternalErr(w, r, err, "failed to load subscription")
		return
	}
	response.JSON(w, r, http.StatusOK, map[string]any{"subscription": sub})
}

func (h *Handler) CancelSubscription(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	if err := h.svc.CancelSubscription(r.Context(), payerFrom(p)); err != nil {
		writeCheckoutError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Subscribe(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	var in struct {
		PlanUUID      uuid.UUID `json:"plan_uuid"`
		SavedCardUUID uuid.UUID `json:"saved_card_uuid"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}
	if in.PlanUUID == uuid.Nil || in.SavedCardUUID == uuid.Nil {
		response.BadRequest(w, r, response.CodeValidationError, "plan_uuid and saved_card_uuid are required")
		return
	}
	sub, err := h.svc.SubscribeWithCard(
		r.Context(), payerFrom(p), clientIP(r),
		in.PlanUUID, in.SavedCardUUID,
	)
	if err != nil {
		writeCheckoutError(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusCreated, map[string]any{"subscription": sub})
}

func (h *Handler) ListCards(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	items, err := h.svc.ListCards(r.Context(), payerFrom(p))
	if err != nil {
		writeCheckoutError(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) StartAddCard(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	session, err := h.svc.StartAddCard(r.Context(), payerFrom(p), clientIP(r))
	if err != nil {
		writeCheckoutError(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusCreated, map[string]any{"session": session})
}

func (h *Handler) GetAddCardSession(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}
	session, err := h.svc.GetAddCardSession(r.Context(), payerFrom(p), id)
	if err != nil {
		writeCheckoutError(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, session)
}

func (h *Handler) DeleteCard(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}
	if err := h.svc.DeleteCard(r.Context(), payerFrom(p), id); err != nil {
		writeCheckoutError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) SetDefaultCard(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}
	card, err := h.svc.SetDefaultCard(r.Context(), payerFrom(p), id)
	if err != nil {
		writeCheckoutError(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, card)
}

func (h *Handler) ResumeSubscription(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	if err := h.svc.ScheduleCancellation(r.Context(), payerFrom(p), false); err != nil {
		writeCheckoutError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ChangePlan(w http.ResponseWriter, r *http.Request) {
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
	if err := h.svc.ChangePlan(r.Context(), payerFrom(p), in.PlanUUID); err != nil {
		writeCheckoutError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
