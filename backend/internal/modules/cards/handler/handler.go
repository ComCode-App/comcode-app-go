package handler

import (
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/clientip"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/cards/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/cards/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/resourcemeta"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/apiquery"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
)

// ActivationLimiter caps card activation attempts (activation codes are bearer secrets).
type ActivationLimiter interface {
	AllowCardActivation(ctx context.Context, userID, ip string) (bool, time.Duration)
}

type Handler struct {
	svc     *usecase.Service
	limiter ActivationLimiter
}

func New(svc *usecase.Service, limiter ActivationLimiter) *Handler {
	return &Handler{svc: svc, limiter: limiter}
}

// writeForbidden maps the account-role error; reports whether it handled err.
func writeForbidden(w http.ResponseWriter, r *http.Request, err error) bool {
	if errors.Is(err, usecase.ErrForbidden) {
		response.Forbidden(w, r, "your account role cannot modify cards")
		return true
	}
	return false
}

func (h *Handler) Meta(w http.ResponseWriter, r *http.Request) {
	meta := resourcemeta.AppCards()
	response.JSON(w, r, http.StatusOK, meta)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	q := apiquery.Parse(r.URL.Query())

	var isActive *bool
	if act := r.URL.Query().Get("is_active"); act != "" {
		b := act == "true" || act == "1"
		isActive = &b
	}

	var qStr *string
	if q.Q != "" {
		qStr = &q.Q
	}

	items, total, err := h.svc.List(r.Context(), p.Account, qStr, isActive, q.Limit, q.Offset)
	if err != nil {
		response.InternalErr(w, r, err, "failed to list cards")
		return
	}

	response.JSON(w, r, http.StatusOK, apiquery.NewPage(items, total, q.Limit, q.Offset))
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}

	card, err := h.svc.Get(r.Context(), id, p.Account)
	if err != nil {
		if errors.Is(err, usecase.ErrCardNotFound) {
			response.NotFound(w, r, "card not found")
			return
		}
		response.InternalErr(w, r, err, "failed to get card")
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]any{"data": card})
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())

	var input model.UpsertCardInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid request body")
		return
	}

	card, err := h.svc.Create(r.Context(), p.Account, p.UserInternal, input)
	if err != nil {
		if entitlement.WriteError(w, r, err) {
			return
		}
		if errors.Is(err, usecase.ErrInvalidProfile) {
			response.BadRequest(w, r, response.CodeValidationError, "selected profile is invalid or belongs to another user")
			return
		}
		if writeForbidden(w, r, err) {
			return
		}
		response.InternalErr(w, r, err, "failed to create card")
		return
	}

	response.JSON(w, r, http.StatusCreated, map[string]any{"data": card})
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}

	var input model.UpsertCardInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid request body")
		return
	}

	card, err := h.svc.Update(r.Context(), id, p.Account, input)
	if err != nil {
		if errors.Is(err, usecase.ErrCardNotFound) {
			response.NotFound(w, r, "card not found")
			return
		}
		if errors.Is(err, usecase.ErrInvalidProfile) {
			response.BadRequest(w, r, response.CodeValidationError, "selected profile is invalid or belongs to another user")
			return
		}
		if writeForbidden(w, r, err) {
			return
		}
		response.InternalErr(w, r, err, "failed to update card")
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]any{"data": card})
}

func (h *Handler) ToggleLock(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}

	isLocked, err := h.svc.ToggleLock(r.Context(), id, p.Account)
	if err != nil {
		if errors.Is(err, usecase.ErrCardNotFound) {
			response.NotFound(w, r, "card not found")
			return
		}
		if writeForbidden(w, r, err) {
			return
		}
		response.InternalErr(w, r, err, "failed to toggle card lock")
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]any{"data": map[string]any{"is_locked": isLocked}})
}

func (h *Handler) Activate(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())

	var input model.ActivateCardInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid request body")
		return
	}

	if h.limiter != nil {
		if ok, retry := h.limiter.AllowCardActivation(r.Context(), p.UserID.String(), clientip.String(r)); !ok {
			if retry > 0 {
				w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())+1))
			}
			response.TooManyRequests(w, r, "Too many activation attempts. Try again later.")
			return
		}
	}

	card, err := h.svc.Activate(r.Context(), p.Account, p.UserInternal, input)
	if err != nil {
		if entitlement.WriteError(w, r, err) {
			return
		}
		if errors.Is(err, usecase.ErrInvalidActivation) {
			response.BadRequest(w, r, response.CodeValidationError, "invalid activation code or card has already been activated")
			return
		}
		if errors.Is(err, usecase.ErrInvalidProfile) {
			response.BadRequest(w, r, response.CodeValidationError, "selected profile is invalid")
			return
		}
		if writeForbidden(w, r, err) {
			return
		}
		response.InternalErr(w, r, err, "failed to activate card")
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]any{"data": card})
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}

	if err := h.svc.Delete(r.Context(), id, p.Account); err != nil {
		if errors.Is(err, usecase.ErrCardNotFound) {
			response.NotFound(w, r, "card not found")
			return
		}
		if writeForbidden(w, r, err) {
			return
		}
		response.InternalErr(w, r, err, "failed to delete card")
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]any{"success": true})
}

func (h *Handler) ResolveSmartLink(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	if code == "" {
		response.BadRequest(w, r, response.CodeValidationError, "smart code is required")
		return
	}

	res, err := h.svc.ResolveSmartLink(r.Context(), code)
	if err != nil {
		if errors.Is(err, usecase.ErrCardNotFound) {
			response.NotFound(w, r, "smart link not found")
			return
		}
		response.InternalErr(w, r, err, "failed to resolve smart link")
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]any{"data": res})
}
