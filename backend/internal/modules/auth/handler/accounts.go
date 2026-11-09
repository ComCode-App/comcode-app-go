package handler

import (
	"encoding/json"
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
	"github.com/google/uuid"
)

// ListAccounts returns the caller's account memberships with the active one flagged.
func (h *Handler) ListAccounts(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	items, err := h.uc.ListAccounts(r.Context(), p.UserInternal, p.Account.ID)
	if err != nil {
		writeUsecaseError(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, map[string]any{"items": items})
}

type switchAccountRequest struct {
	AccountUUID uuid.UUID `json:"account_uuid"`
}

// SwitchAccount changes the active account of the current session and returns a new token pair.
func (h *Handler) SwitchAccount(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	var in switchAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "Invalid JSON body")
		return
	}
	result, err := h.uc.SwitchAccount(r.Context(), p, in.AccountUUID)
	if err != nil {
		writeUsecaseError(w, r, err)
		return
	}
	if h.activity != nil {
		h.activity.Record(r.Context(), &p.UserInternal, "auth.account_switched", "auth.accounts", &result.Account.UUID, map[string]any{
			"from_account_uuid": p.Account.UUID.String(),
			"to_account_uuid":   result.Account.UUID.String(),
		}, r)
	}
	response.JSON(w, r, http.StatusOK, result)
}
