// Package handler exposes lead capture over HTTP: the visitor form and contact exchange (no
// auth, plus a signed-in profile share) and the account's lead inbox.
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/leads/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/leads/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/clientip"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/resourcemeta"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/apiquery"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
	"github.com/google/uuid"
)

// SubmitLimiter caps public lead submissions per client IP (ratelimit.Limiter).
type SubmitLimiter interface {
	AllowLeadSubmit(ctx context.Context, ip string) (bool, time.Duration)
}

// codeSelfExchange: the profile page belongs to the caller's own active account.
const codeSelfExchange = "SELF_EXCHANGE"

// codeInvalidDeleteToken: the visitor copy's delete link is malformed, forged or expired.
const codeInvalidDeleteToken = "INVALID_DELETE_TOKEN"

// Handler serves /v1/public/leads, /v1/public/exchanges, /v1/exchanges and /v1/leads.
type Handler struct {
	svc     *usecase.Service
	limiter SubmitLimiter
}

// New creates the handler. limiter may be nil (no limit).
func New(svc *usecase.Service, limiter SubmitLimiter) *Handler {
	return &Handler{svc: svc, limiter: limiter}
}

// FormConfig answers GET /v1/public/leads/form?path=: whether a visitor page shows the form.
func (h *Handler) FormConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.svc.FormConfig(r.Context(), r.URL.Query().Get("path"))
	if err != nil {
		response.InternalErr(w, r, err, "failed to load lead form")
		return
	}
	response.JSON(w, r, http.StatusOK, cfg)
}

// Submit answers POST /v1/public/leads.
func (h *Handler) Submit(w http.ResponseWriter, r *http.Request) {
	if !h.allow(w, r) {
		return
	}
	var in model.SubmitInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&in); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid request body")
		return
	}
	res, err := h.svc.Submit(r.Context(), in, visitorOf(r))
	if err != nil {
		writeSubmitErr(w, r, err, "lead form not found")
		return
	}
	response.JSON(w, r, http.StatusCreated, res)
}

// Exchange answers POST /v1/public/exchanges: a visitor shares their details with the owner of
// the profile page (TEC-567).
func (h *Handler) Exchange(w http.ResponseWriter, r *http.Request) {
	if !h.allow(w, r) {
		return
	}
	var in model.ExchangeInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&in); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid request body")
		return
	}
	res, err := h.svc.Exchange(r.Context(), in, visitorOf(r))
	if err != nil {
		writeSubmitErr(w, r, err, "contact exchange not found")
		return
	}
	response.JSON(w, r, http.StatusCreated, res)
}

// ShareProfile answers POST /v1/exchanges: a signed-in visitor shares one of their profiles.
func (h *Handler) ShareProfile(w http.ResponseWriter, r *http.Request) {
	if !h.allow(w, r) {
		return
	}
	p := authctx.MustPrincipal(r.Context())
	var in model.ShareProfileInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&in); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid request body")
		return
	}
	res, err := h.svc.ShareProfile(r.Context(), p, in, visitorOf(r))
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrSenderProfileNotFound):
			response.NotFound(w, r, "profile not found")
		case errors.Is(err, usecase.ErrSenderProfileForbidden):
			response.Forbidden(w, r, "your account role cannot share this profile")
		case errors.Is(err, usecase.ErrImpersonating):
			response.Forbidden(w, r, "contact exchange is not available while impersonating")
		case errors.Is(err, usecase.ErrSelfExchange):
			response.Conflict(w, r, codeSelfExchange, "cannot exchange contact details with your own account")
		default:
			writeSubmitErr(w, r, err, "contact exchange not found")
		}
		return
	}
	response.JSON(w, r, http.StatusCreated, res)
}

// DeleteExchange answers POST /v1/public/exchanges/delete: the KVKK "delete my data" link of
// the visitor copy email (TEC-940). The page posts the token; a GET never deletes, so mail
// scanners that open links cannot use it up.
func (h *Handler) DeleteExchange(w http.ResponseWriter, r *http.Request) {
	if !h.allow(w, r) {
		return
	}
	var in model.DeleteExchangeInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&in); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid request body")
		return
	}
	switch err := h.svc.DeleteByToken(r.Context(), in); {
	case err == nil:
		response.JSON(w, r, http.StatusOK, map[string]any{"deleted": true})
	case errors.Is(err, usecase.ErrInvalidDeleteToken):
		response.BadRequest(w, r, codeInvalidDeleteToken, "invalid or expired link")
	case errors.Is(err, usecase.ErrExchangeGone):
		response.NotFound(w, r, "already deleted")
	default:
		response.InternalErr(w, r, err, "failed to delete")
	}
}

// allow applies the per-IP submission limit shared by lead forms and exchanges.
func (h *Handler) allow(w http.ResponseWriter, r *http.Request) bool {
	if h.limiter == nil {
		return true
	}
	ok, retry := h.limiter.AllowLeadSubmit(r.Context(), clientip.String(r))
	if ok {
		return true
	}
	if retry > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())+1))
	}
	response.TooManyRequests(w, r, "Too many submissions. Try again later.")
	return false
}

func visitorOf(r *http.Request) model.Visitor {
	return model.Visitor{IP: clientip.String(r), UserAgent: r.UserAgent()}
}

func writeSubmitErr(w http.ResponseWriter, r *http.Request, err error, notFound string) {
	var verr *usecase.ValidationError
	switch {
	case errors.As(err, &verr):
		details := make([]response.Detail, 0, len(verr.Fields))
		for _, f := range verr.Fields {
			details = append(details, response.Detail{Field: f.Field, Code: f.Code, Message: f.Field + " " + f.Code})
		}
		response.ValidationError(w, r, details)
	case errors.Is(err, usecase.ErrFormUnavailable):
		response.NotFound(w, r, notFound)
	default:
		response.InternalErr(w, r, err, "failed to submit")
	}
}

// Meta answers GET /v1/leads/meta.
func (h *Handler) Meta(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, r, http.StatusOK, resourcemeta.AppLeads())
}

// List answers GET /v1/leads (q, source_type, kind, unread, limit, offset).
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	q := apiquery.Parse(r.URL.Query())
	f := model.ListFilter{Limit: q.Limit, Offset: q.Offset}
	if q.Q != "" {
		f.Q = &q.Q
	}
	if s := r.URL.Query().Get("source_type"); s != "" {
		if !slices.Contains(model.Sources, s) {
			response.BadRequest(w, r, response.CodeValidationError, "invalid source_type")
			return
		}
		f.SourceType = &s
	}
	if k := r.URL.Query().Get("kind"); k != "" {
		if !slices.Contains(model.Kinds, k) {
			response.BadRequest(w, r, response.CodeValidationError, "invalid kind")
			return
		}
		f.Kind = &k
	}
	if u := r.URL.Query().Get("unread"); u != "" {
		b := u == "true" || u == "1"
		f.Unread = &b
	}
	items, total, err := h.svc.List(r.Context(), p, f)
	if err != nil {
		response.InternalErr(w, r, err, "failed to list leads")
		return
	}
	response.JSON(w, r, http.StatusOK, apiquery.NewPage(items, total, q.Limit, q.Offset))
}

// Get answers GET /v1/leads/{uuid}.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	id, ok := pathUUID(w, r)
	if !ok {
		return
	}
	lead, err := h.svc.Get(r.Context(), p, id)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, lead)
}

// MarkRead answers POST /v1/leads/{uuid}/read.
func (h *Handler) MarkRead(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	id, ok := pathUUID(w, r)
	if !ok {
		return
	}
	lead, err := h.svc.MarkRead(r.Context(), p, id)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, lead)
}

// Delete answers DELETE /v1/leads/{uuid}.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	id, ok := pathUUID(w, r)
	if !ok {
		return
	}
	if err := h.svc.Delete(r.Context(), p, id); err != nil {
		writeErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, map[string]any{"deleted": true})
}

func pathUUID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return uuid.Nil, false
	}
	return id, true
}

func writeErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, usecase.ErrNotFound):
		response.NotFound(w, r, "lead not found")
	case errors.Is(err, usecase.ErrForbidden):
		response.Forbidden(w, r, "your account role cannot modify leads")
	default:
		response.InternalErr(w, r, err, "unexpected error")
	}
}
