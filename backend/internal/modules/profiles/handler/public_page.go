package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/model"
	profilesusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/usecase"
	routesmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/ingestion"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/clientip"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
	"github.com/google/uuid"
)

// LegacyResolver uses the normal backend route cache and never runs in middleware.
type LegacyResolver interface {
	ResolveLegacy(context.Context, string, ...ingestion.HTTPContext) (*routesmodel.RouteResolution, error)
}

func (h *Handler) SetLegacyResolver(resolver LegacyResolver) { h.legacyResolver = resolver }

func writePublicPageErr(w http.ResponseWriter, r *http.Request, err error) {
	if entitlement.WriteError(w, r, err) { // premium theme / background without design.premium
		return
	}
	switch {
	case errors.Is(err, profilesusecase.ErrNotFound):
		response.NotFound(w, r, "profile not found")
	case errors.Is(err, profilesusecase.ErrForbidden):
		response.Forbidden(w, r, "your account role cannot change public pages")
	case errors.Is(err, profilesusecase.ErrUsernameTaken):
		response.Conflict(w, r, response.CodeConflict, err.Error())
	case errors.Is(err, profilesusecase.ErrInvalidRequest):
		response.BadRequest(w, r, response.CodeValidationError, err.Error())
	default:
		response.InternalErr(w, r, err, "public page operation failed")
	}
}

// GetPublicPage returns a profile's public page settings (username, visibility, modules, banks).
func (h *Handler) GetPublicPage(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	profileUUID, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid profile uuid")
		return
	}
	out, err := h.svc.GetPublicPage(r.Context(), p, profileUUID)
	if err != nil {
		writePublicPageErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, out)
}

// UpdatePublicPage changes a profile's public page settings.
func (h *Handler) UpdatePublicPage(w http.ResponseWriter, r *http.Request) {
	p := authctx.MustPrincipal(r.Context())
	profileUUID, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid profile uuid")
		return
	}
	var in model.UpdatePublicPageInput
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}
	out, err := h.svc.UpdatePublicPage(r.Context(), p, profileUUID, in)
	if err != nil {
		writePublicPageErr(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, out)
}

// PublicProfile is the unauthenticated visitor view of comcode.../{username}.
func (h *Handler) PublicProfile(w http.ResponseWriter, r *http.Request) {
	httpContext := ingestion.HTTPContext{
		ClientIP: clientip.IP(r), UserAgent: r.UserAgent(),
		Referer: r.Referer(), RequestID: r.Header.Get("X-Request-ID"),
		Source: r.URL.Query().Get("src"),
	}
	if h.legacyResolver != nil {
		legacy, err := h.legacyResolver.ResolveLegacy(r.Context(), r.PathValue("username"), httpContext)
		if err != nil {
			response.InternalErr(w, r, err, "legacy resolution failed")
			return
		}
		if legacy != nil {
			target, err := url.Parse(legacy.TargetURL)
			validTarget := err == nil && target.Hostname() != "" && target.User == nil &&
				(target.Scheme == "http" || target.Scheme == "https")
			if !legacy.IsActive || legacy.IsLocked || legacy.RequiresPasscode || !validTarget {
				response.NotFound(w, r, "legacy destination unavailable")
				return
			}
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Location", legacy.TargetURL)
			w.WriteHeader(http.StatusTemporaryRedirect)
			return
		}
	}
	if r.URL.Query().Get("src") == "qr" && h.qrTracker != nil {
		h.qrTracker.RecordQRScan(r.Context(), r.PathValue("username"), httpContext)
	}
	out, err := h.svc.PublicProfileVia(r.Context(), r.PathValue("username"), r.URL.Query().Get("via"))
	if err != nil {
		writePublicPageErr(w, r, err)
		return
	}
	if r.URL.Query().Get("src") == "qr" {
		w.Header().Set("Cache-Control", "no-store")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=30")
	}
	response.JSON(w, r, http.StatusOK, out)
}

// PublicVCard downloads the visitor vCard of /{username}. A renamed username redirects to the
// new one with a relative Location, so it works behind the BFF prefix too.
func (h *Handler) PublicVCard(w http.ResponseWriter, r *http.Request) {
	content, filename, redirect, err := h.svc.PublicVCard(r.Context(), r.PathValue("username"), r.URL.Query().Get("locale"))
	if err != nil {
		writePublicPageErr(w, r, err)
		return
	}
	if redirect != "" {
		target := "../" + url.PathEscape(redirect) + "/vcard"
		if q := r.URL.RawQuery; q != "" {
			target += "?" + q
		}
		w.Header().Set("Location", target)
		w.WriteHeader(http.StatusMovedPermanently)
		return
	}
	w.Header().Set("Content-Type", "text/vcard; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"contact.vcf\"; filename*=UTF-8''%s", url.PathEscape(filename)))
	w.Header().Set("Cache-Control", "public, max-age=60")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(content))
}
