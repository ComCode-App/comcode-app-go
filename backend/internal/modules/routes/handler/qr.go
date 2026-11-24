package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/qr"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
	"github.com/google/uuid"
)

func qrError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, usecase.ErrRouteNotFound):
		response.NotFound(w, r, "route not found")
	case errors.Is(err, usecase.ErrForbidden):
		response.Forbidden(w, r, "your account role cannot modify routes")
	case errors.Is(err, qr.ErrInvalid):
		response.BadRequest(w, r, response.CodeValidationError, "invalid QR style, logo, format or size")
	default:
		response.InternalErr(w, r, err, "QR operation failed")
	}
}

// QRLogoLimiter caps QR logo reads per user (ratelimit.Limiter).
type QRLogoLimiter interface {
	AllowQRLogo(ctx context.Context, userID string) (bool, time.Duration)
}

// SetQRLogoLimiter enables per-user limits on GET /v1/routes/{uuid}/qr/logo.
func (h *Handler) SetQRLogoLimiter(l QRLogoLimiter) { h.qrLogoLimiter = l }

// decodeQRStyle reads one strict, size-capped style object; it writes the 400 itself.
func decodeQRStyle(w http.ResponseWriter, r *http.Request) (qr.Style, bool) {
	var style qr.Style
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&style); err != nil {
		qrError(w, r, qr.ErrInvalid)
		return style, false
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		qrError(w, r, qr.ErrInvalid)
		return style, false
	}
	return style, true
}

func (h *Handler) GetQR(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		qrError(w, r, qr.ErrInvalid)
		return
	}
	p := authctx.MustPrincipal(r.Context())
	result, err := h.svc.GetQR(r.Context(), id, p.Account)
	if err != nil {
		qrError(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, result)
}

func (h *Handler) PutQR(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		qrError(w, r, qr.ErrInvalid)
		return
	}
	style, ok := decodeQRStyle(w, r)
	if !ok {
		return
	}
	p := authctx.MustPrincipal(r.Context())
	result, err := h.svc.PutQR(r.Context(), id, p.Account, style)
	if err != nil {
		qrError(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, result)
}

func (h *Handler) RenderQR(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		qrError(w, r, qr.ErrInvalid)
		return
	}
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "png"
	}
	size := 1024
	if raw := r.URL.Query().Get("size"); raw != "" {
		size, err = strconv.Atoi(raw)
		if err != nil {
			qrError(w, r, qr.ErrInvalid)
			return
		}
	}
	p := authctx.MustPrincipal(r.Context())
	data, ct, err := h.svc.RenderQR(r.Context(), id, p.Account, format, size)
	if err != nil {
		qrError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="qr-%s.%s"`, id, format))
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	_, _ = w.Write(data)
}

// QRLogo streams a same-account profile avatar or cover for the browser QR editor, so files
// drawn in the browser can embed the logo without reaching the storage origin (TEC-1271).
func (h *Handler) QRLogo(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("uuid"))
	if err != nil {
		qrError(w, r, qr.ErrInvalid)
		return
	}
	profileUUID, err := uuid.Parse(r.URL.Query().Get("profile_uuid"))
	if err != nil {
		qrError(w, r, qr.ErrInvalid)
		return
	}
	p := authctx.MustPrincipal(r.Context())
	if h.qrLogoLimiter != nil {
		if ok, retry := h.qrLogoLimiter.AllowQRLogo(r.Context(), p.UserID.String()); !ok {
			if retry > 0 {
				w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())+1))
			}
			response.TooManyRequests(w, r, "Too many QR logo requests. Try again later.")
			return
		}
	}
	data, ct, err := h.svc.QRLogo(r.Context(), id, p.Account, profileUUID, r.URL.Query().Get("field"))
	if err != nil {
		qrError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	_, _ = w.Write(data)
}
