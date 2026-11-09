package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/bankaccounts/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/bankaccounts/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/iban"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/resourcemeta"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
	"github.com/google/uuid"
)

type Handler struct {
	svc *usecase.Service
}

func New(svc *usecase.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Meta(w http.ResponseWriter, r *http.Request) {
	meta := resourcemeta.AppBankAccounts()
	response.JSON(w, r, http.StatusOK, meta)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	if p.Account.ID == 0 {
		response.Unauthorized(w, r, "unauthorized")
		return
	}

	items, err := h.svc.List(r.Context(), p.Account)
	if err != nil {
		response.InternalErr(w, r, err, "failed to list bank accounts")
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]any{"data": items})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	if p.Account.ID == 0 {
		response.Unauthorized(w, r, "unauthorized")
		return
	}

	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}

	acc, err := h.svc.Get(r.Context(), id, p.Account)
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			response.NotFound(w, r, "bank account not found")
			return
		}
		response.InternalErr(w, r, err, "failed to get bank account")
		return
	}

	response.JSON(w, r, http.StatusOK, acc)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	if p.Account.ID == 0 {
		response.Unauthorized(w, r, "unauthorized")
		return
	}

	var input model.CreateUserBankAccountInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid request body")
		return
	}

	acc, err := h.svc.Create(r.Context(), p.Account, p.UserInternal, input)
	if err != nil {
		if errors.Is(err, usecase.ErrAccountHolderRequired) || errors.Is(err, iban.ErrEmptyIBAN) || errors.Is(err, usecase.ErrInvalidIBAN) {
			response.BadRequest(w, r, response.CodeValidationError, err.Error())
			return
		}
		if errors.Is(err, usecase.ErrForbidden) {
			response.Forbidden(w, r, "your account role cannot modify bank accounts")
			return
		}
		response.InternalErr(w, r, err, "failed to create bank account")
		return
	}

	response.JSON(w, r, http.StatusCreated, acc)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	if p.Account.ID == 0 {
		response.Unauthorized(w, r, "unauthorized")
		return
	}

	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}

	var input model.UpdateUserBankAccountInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid request body")
		return
	}

	acc, err := h.svc.Update(r.Context(), id, p.Account, input)
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			response.NotFound(w, r, "bank account not found")
			return
		}
		if errors.Is(err, usecase.ErrInvalidIBAN) {
			response.BadRequest(w, r, response.CodeValidationError, err.Error())
			return
		}
		if errors.Is(err, usecase.ErrForbidden) {
			response.Forbidden(w, r, "your account role cannot modify bank accounts")
			return
		}
		response.InternalErr(w, r, err, "failed to update bank account")
		return
	}

	response.JSON(w, r, http.StatusOK, acc)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	if p.Account.ID == 0 {
		response.Unauthorized(w, r, "unauthorized")
		return
	}

	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}

	if err := h.svc.Delete(r.Context(), id, p.Account); err != nil {
		if errors.Is(err, usecase.ErrForbidden) {
			response.Forbidden(w, r, "your account role cannot modify bank accounts")
			return
		}
		response.InternalErr(w, r, err, "failed to delete bank account")
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]bool{"success": true})
}

func (h *Handler) ToggleActive(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	if p.Account.ID == 0 {
		response.Unauthorized(w, r, "unauthorized")
		return
	}

	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}

	acc, err := h.svc.ToggleActive(r.Context(), id, p.Account)
	if err != nil {
		if errors.Is(err, usecase.ErrNotFound) {
			response.NotFound(w, r, "bank account not found")
			return
		}
		if errors.Is(err, usecase.ErrForbidden) {
			response.Forbidden(w, r, "your account role cannot modify bank accounts")
			return
		}
		response.InternalErr(w, r, err, "failed to toggle bank account status")
		return
	}

	response.JSON(w, r, http.StatusOK, acc)
}

func (h *Handler) Reorder(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	if p.Account.ID == 0 {
		response.Unauthorized(w, r, "unauthorized")
		return
	}

	var input model.ReorderAccountsInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid request body")
		return
	}

	if err := h.svc.Reorder(r.Context(), p.Account, input.UUIDs); err != nil {
		if errors.Is(err, usecase.ErrForbidden) {
			response.Forbidden(w, r, "your account role cannot modify bank accounts")
			return
		}
		response.InternalErr(w, r, err, "failed to reorder bank accounts")
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]bool{"success": true})
}
