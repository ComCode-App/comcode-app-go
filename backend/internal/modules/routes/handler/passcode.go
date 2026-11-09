package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
)

// PasscodeLimiter caps wrong smart link passcodes (ratelimit.Limiter). Only failures count, so a
// visitor who knows the passcode is never locked out by their own visits.
type PasscodeLimiter interface {
	SmartLinkPasscodeAllowed(ctx context.Context, ip, path string) (bool, time.Duration)
	RecordSmartLinkPasscodeFailure(ctx context.Context, ip, path string)
}

// SetPasscodeLimiter enables wrong-attempt limits on POST /v1/s/{slug}/unlock.
func (h *Handler) SetPasscodeLimiter(l PasscodeLimiter) { h.passcodeLimiter = l }

type unlockInput struct {
	Passcode string `json:"passcode"`
}

// UnlockSmartLink checks a visitor's passcode for a protected /s/{slug} link. The passcode is
// sent in the body only; a correct one returns the resolution with its target_url.
func (h *Handler) UnlockSmartLink(w http.ResponseWriter, r *http.Request) {
	slug := strings.ToLower(strings.TrimSpace(r.PathValue("slug")))
	if slug == "" {
		response.BadRequest(w, r, response.CodeValidationError, "slug is required")
		return
	}
	var in unlockInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&in); err != nil || in.Passcode == "" {
		response.BadRequest(w, r, response.CodeValidationError, "passcode is required")
		return
	}

	ip := ""
	if parsed := extractIP(r); parsed != nil {
		ip = parsed.String()
	}
	path := "s/" + slug
	if h.passcodeLimiter != nil {
		if ok, retry := h.passcodeLimiter.SmartLinkPasscodeAllowed(r.Context(), ip, path); !ok {
			if retry > 0 {
				w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())+1))
			}
			response.TooManyRequests(w, r, "Too many wrong passcodes. Try again later.")
			return
		}
	}

	res, err := h.svc.UnlockSmartLink(r.Context(), slug, in.Passcode)
	switch {
	case errors.Is(err, usecase.ErrInvalidPasscode):
		if h.passcodeLimiter != nil {
			h.passcodeLimiter.RecordSmartLinkPasscodeFailure(r.Context(), ip, path)
		}
		response.Error(w, r, http.StatusForbidden, response.CodeInvalidPasscode, "wrong passcode")
	case errors.Is(err, usecase.ErrRouteNotFound):
		response.NotFound(w, r, "smart link route not found")
	case err != nil:
		response.InternalErr(w, r, err, "failed to unlock smart link")
	default:
		w.Header().Set("Cache-Control", "no-store")
		response.JSON(w, r, http.StatusOK, res)
	}
}
