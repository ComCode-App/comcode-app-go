package handler

import (
	"encoding/json"
	"errors"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/clientip"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
	"net"
	"net/http"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/ingestion"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/resourcemeta"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/apiquery"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
	"github.com/google/uuid"
)

type Handler struct {
	svc              *usecase.Service
	qrPreviewLimiter QRPreviewLimiter
	passcodeLimiter  PasscodeLimiter
}

func New(svc *usecase.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Meta(w http.ResponseWriter, r *http.Request) {
	meta := resourcemeta.AppRoutes()
	response.JSON(w, r, http.StatusOK, meta)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	q := apiquery.Parse(r.URL.Query())

	var routeType *string
	if rt := r.URL.Query().Get("route_type"); rt != "" {
		routeType = &rt
	}

	var status *string
	if st := r.URL.Query().Get("status"); st != "" {
		status = &st
	}

	var qStr *string
	if q.Q != "" {
		qStr = &q.Q
	}

	items, total, err := h.svc.List(r.Context(), p.Account, routeType, status, qStr, q.Limit, q.Offset)
	if err != nil {
		response.InternalErr(w, r, err, "failed to list routes")
		return
	}

	response.JSON(w, r, http.StatusOK, apiquery.NewPage(items, total, q.Limit, q.Offset))
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}

	p := authctx.MustPrincipal(r.Context())
	route, err := h.svc.Get(r.Context(), id, p.Account)
	if err != nil {
		if errors.Is(err, usecase.ErrRouteNotFound) {
			response.NotFound(w, r, "route not found")
			return
		}
		response.InternalErr(w, r, err, "failed to get route")
		return
	}

	response.JSON(w, r, http.StatusOK, route)
}

func (h *Handler) CreateSmartLink(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())

	var input model.CreateSmartLinkInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid request body")
		return
	}

	route, err := h.svc.CreateCustomSmartLink(r.Context(), p.Account, p.UserInternal, input)
	if err != nil {
		if entitlement.WriteError(w, r, err) {
			return
		}
		if errors.Is(err, usecase.ErrRoutePathReserved) {
			response.BadRequest(w, r, response.CodeValidationError, "path is reserved by system")
			return
		}
		if errors.Is(err, usecase.ErrRoutePathConflict) {
			response.Conflict(w, r, response.CodeConflict, "route path already in use")
			return
		}
		if errors.Is(err, usecase.ErrInvalidRoutePath) || errors.Is(err, usecase.ErrInvalidPresentation) ||
			errors.Is(err, usecase.ErrInvalidTarget) {
			response.BadRequest(w, r, response.CodeValidationError, err.Error())
			return
		}
		if errors.Is(err, usecase.ErrInvalidProfileTarget) {
			response.BadRequest(w, r, response.CodeValidationError, "selected profile is invalid or belongs to another user")
			return
		}
		if errors.Is(err, usecase.ErrForbidden) {
			response.Forbidden(w, r, "your account role cannot modify routes")
			return
		}
		response.InternalErr(w, r, err, "failed to create smart link")
		return
	}

	response.JSON(w, r, http.StatusCreated, route)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}

	var input model.UpdateRouteInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid request body")
		return
	}

	route, err := h.svc.Update(r.Context(), id, p.Account, input)
	if err != nil {
		if entitlement.WriteError(w, r, err) {
			return
		}
		if errors.Is(err, usecase.ErrRouteNotFound) {
			response.NotFound(w, r, "route not found")
			return
		}
		if errors.Is(err, usecase.ErrRouteImmutable) {
			response.BadRequest(w, r, response.CodeValidationError, "route identity is immutable")
			return
		}
		if errors.Is(err, usecase.ErrInvalidProfileTarget) {
			response.BadRequest(w, r, response.CodeValidationError, "selected profile is invalid")
			return
		}
		if errors.Is(err, usecase.ErrRoutePathReserved) {
			response.BadRequest(w, r, response.CodeValidationError, "path is reserved by system")
			return
		}
		if errors.Is(err, usecase.ErrRoutePathConflict) {
			response.Conflict(w, r, response.CodeConflict, "route path already in use")
			return
		}
		if errors.Is(err, usecase.ErrInvalidRoutePath) || errors.Is(err, usecase.ErrInvalidPresentation) ||
			errors.Is(err, usecase.ErrInvalidTarget) {
			response.BadRequest(w, r, response.CodeValidationError, err.Error())
			return
		}
		if errors.Is(err, usecase.ErrRouteFieldForbidden) {
			response.Forbidden(w, r, "route status and non smart-link paths cannot be changed")
			return
		}
		if errors.Is(err, usecase.ErrForbidden) {
			response.Forbidden(w, r, "your account role cannot modify routes")
			return
		}
		response.InternalErr(w, r, err, "failed to update route")
		return
	}

	response.JSON(w, r, http.StatusOK, route)
}

func (h *Handler) ToggleLock(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}

	route, err := h.svc.ToggleLock(r.Context(), id, p.Account)
	if err != nil {
		if errors.Is(err, usecase.ErrRouteNotFound) {
			response.NotFound(w, r, "route not found")
			return
		}
		if errors.Is(err, usecase.ErrRouteNotLockable) {
			response.Conflict(w, r, response.CodeConflict, "only active or locked routes can be toggled")
			return
		}
		if errors.Is(err, usecase.ErrForbidden) {
			response.Forbidden(w, r, "your account role cannot modify routes")
			return
		}
		response.InternalErr(w, r, err, "failed to toggle route lock")
		return
	}

	response.JSON(w, r, http.StatusOK, route)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid uuid")
		return
	}

	if err := h.svc.Delete(r.Context(), id, p.Account); err != nil {
		if errors.Is(err, usecase.ErrRouteNotFound) {
			response.NotFound(w, r, "route not found")
			return
		}
		if errors.Is(err, usecase.ErrRouteImmutable) {
			response.BadRequest(w, r, response.CodeValidationError, "immutable route cannot be deleted")
			return
		}
		if errors.Is(err, usecase.ErrForbidden) {
			response.Forbidden(w, r, "your account role cannot modify routes")
			return
		}
		response.InternalErr(w, r, err, "failed to delete route")
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]any{"success": true})
}

// Universal Resolver
func (h *Handler) Resolve(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path")
	if path == "" {
		response.BadRequest(w, r, response.CodeValidationError, "path is required")
		return
	}

	httpCtx := extractHTTPContext(r)
	res, err := h.svc.Resolve(r.Context(), path, httpCtx)
	if err != nil {
		if errors.Is(err, usecase.ErrRouteNotFound) {
			response.NotFound(w, r, "route not found")
			return
		}
		response.InternalErr(w, r, err, "failed to resolve route")
		return
	}

	response.JSON(w, r, http.StatusOK, res)
}

// Dedicated Card Resolver
func (h *Handler) ResolveCard(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	if code == "" {
		response.BadRequest(w, r, response.CodeValidationError, "card code is required")
		return
	}

	httpCtx := extractHTTPContext(r)
	res, err := h.svc.Resolve(r.Context(), "c/"+code, httpCtx)
	if err != nil {
		if errors.Is(err, usecase.ErrRouteNotFound) {
			response.NotFound(w, r, "card route not found")
			return
		}
		response.InternalErr(w, r, err, "failed to resolve card route")
		return
	}

	response.JSON(w, r, http.StatusOK, res)
}

// Dedicated Smart Link Resolver
func (h *Handler) ResolveSmartLink(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if slug == "" {
		response.BadRequest(w, r, response.CodeValidationError, "slug is required")
		return
	}

	httpCtx := extractHTTPContext(r)
	res, err := h.svc.Resolve(r.Context(), "s/"+slug, httpCtx)
	if err != nil {
		if errors.Is(err, usecase.ErrRouteNotFound) {
			response.NotFound(w, r, "smart link route not found")
			return
		}
		response.InternalErr(w, r, err, "failed to resolve smart link")
		return
	}

	response.JSON(w, r, http.StatusOK, res)
}

func extractHTTPContext(r *http.Request) ingestion.HTTPContext {
	clientIP := extractIP(r)
	return ingestion.HTTPContext{
		Source:    r.URL.Query().Get("src"),
		ClientIP:  clientIP,
		UserAgent: r.UserAgent(),
		Referer:   r.Referer(),
		RequestID: r.Header.Get("X-Request-ID"),
	}
}

func extractIP(r *http.Request) net.IP {
	return clientip.IP(r)
}
