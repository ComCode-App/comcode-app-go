package qrlogin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	authusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/clientip"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
)

// Rate limits per 15-minute window (docs/design/mobile.md §2.6). Wrong browser secrets are
// limited separately by the session itself (MaxSecretAttempts).
const (
	rateWindow = 15 * time.Minute
	// A login tab creates a new QR every 2 minutes while it stays open.
	createIPLimit   = 60
	stateIPLimit    = 60
	exchangeIPLimit = 60
	// One approval needs one exchange; a few retries are fine.
	exchangeSessionLimit = 10
	viewUserLimit        = 30
	resolveUserLimit     = 30
)

// Limiter is the generic fixed-window limiter (ratelimit.Limiter.Allow).
type Limiter interface {
	Allow(ctx context.Context, action, subject string, limit int, window time.Duration) (bool, time.Duration)
}

// Handler serves /v1/auth/qr/*.
type Handler struct {
	svc     *Service
	limiter Limiter
}

// NewHandler builds the HTTP handler. limiter may be nil (no limits, tests only).
func NewHandler(svc *Service, limiter Limiter) *Handler {
	return &Handler{svc: svc, limiter: limiter}
}

// Create is POST /v1/auth/qr/sessions (anonymous web tab).
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	ip := clientip.String(r)
	if !h.allow(w, r, rateCheck{"qr_create_ip", ip, createIPLimit}) {
		return
	}
	out, err := h.svc.Create(r.Context(), r.UserAgent(), ip)
	if err != nil {
		writeError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	response.JSON(w, r, http.StatusCreated, out)
}

// State is POST /v1/auth/qr/sessions/{id}/state (anonymous, needs the browser secret).
func (h *Handler) State(w http.ResponseWriter, r *http.Request) {
	var in struct {
		BrowserSecret string `json:"browser_secret"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !h.allow(w, r, rateCheck{"qr_state_ip", clientip.String(r), stateIPLimit}) {
		return
	}
	out, err := h.svc.State(r.Context(), r.PathValue("id"), in.BrowserSecret)
	if err != nil {
		writeError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	response.JSON(w, r, http.StatusOK, out)
}

// Exchange is POST /v1/auth/qr/exchange (anonymous; only the web server calls it).
func (h *Handler) Exchange(w http.ResponseWriter, r *http.Request) {
	var in struct {
		SessionID     string `json:"session_id"`
		BrowserSecret string `json:"browser_secret"`
		ExchangeToken string `json:"exchange_token"`
	}
	if !decode(w, r, &in) {
		return
	}
	if !h.allow(w, r,
		rateCheck{"qr_exchange_ip", clientip.String(r), exchangeIPLimit},
		// Never the raw id in a Redis key.
		rateCheck{"qr_exchange_session", sessionSubject(in.SessionID), exchangeSessionLimit},
	) {
		return
	}
	out, err := h.svc.Exchange(r.Context(), in.SessionID, in.BrowserSecret, in.ExchangeToken)
	if err != nil {
		writeError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	response.JSON(w, r, http.StatusOK, out)
}

// Get is GET /v1/auth/qr/sessions/{id} (Bearer, the phone). It claims the session.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	viewer := approverFrom(r)
	if !h.allow(w, r, rateCheck{"qr_view_user", viewer.UserUUID.String(), viewUserLimit}) {
		return
	}
	out, err := h.svc.Get(r.Context(), r.PathValue("id"), viewer)
	if err != nil {
		writeError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	response.JSON(w, r, http.StatusOK, out)
}

// Approve is POST /v1/auth/qr/sessions/{id}/approve (Bearer).
func (h *Handler) Approve(w http.ResponseWriter, r *http.Request) {
	viewer := approverFrom(r)
	if !h.allow(w, r, rateCheck{"qr_resolve_user", viewer.UserUUID.String(), resolveUserLimit}) {
		return
	}
	if err := h.svc.Approve(r.Context(), r.PathValue("id"), viewer); err != nil {
		writeError(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, map[string]string{"status": StatusApproved})
}

// Reject is POST /v1/auth/qr/sessions/{id}/reject (Bearer).
func (h *Handler) Reject(w http.ResponseWriter, r *http.Request) {
	viewer := approverFrom(r)
	if !h.allow(w, r, rateCheck{"qr_resolve_user", viewer.UserUUID.String(), resolveUserLimit}) {
		return
	}
	if err := h.svc.Reject(r.Context(), r.PathValue("id"), viewer); err != nil {
		writeError(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, map[string]string{"status": StatusRejected})
}

func approverFrom(r *http.Request) Approver {
	p := authctx.MustPrincipal(r.Context())
	return Approver{
		UserUUID:      p.UserID,
		Impersonating: p.ImpersonatorUserID != nil,
		PlatformUser:  hasPlatformAccess(p),
	}
}

// hasPlatformAccess is true for super admins and for any platform.* permission.
func hasPlatformAccess(p authctx.Principal) bool {
	if p.IsSuperAdmin {
		return true
	}
	for _, perm := range p.Permissions {
		if strings.HasPrefix(perm, "platform.") {
			return true
		}
	}
	return false
}

func sessionSubject(id string) string {
	if strings.TrimSpace(id) == "" {
		return ""
	}
	return hashSecret(id)[:32]
}

type rateCheck struct {
	action  string
	subject string
	limit   int
}

// allow applies every check in order and writes 429 + Retry-After on the first denial.
func (h *Handler) allow(w http.ResponseWriter, r *http.Request, checks ...rateCheck) bool {
	if h.limiter == nil {
		return true
	}
	for _, c := range checks {
		if strings.TrimSpace(c.subject) == "" {
			continue
		}
		ok, retry := h.limiter.Allow(r.Context(), c.action, c.subject, c.limit, rateWindow)
		if ok {
			continue
		}
		if retry > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())+1))
		}
		response.TooManyRequests(w, r, "Too many requests, try again later")
		return false
	}
	return true
}

func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "Invalid request body")
		return false
	}
	return true
}

func writeError(w http.ResponseWriter, r *http.Request, err error) {
	response.RecordFailure(w, err)
	switch {
	case errors.Is(err, ErrNotFound):
		response.Error(w, r, http.StatusNotFound, response.CodeQRSessionNotFound, "This QR code has expired or was already used")
	case errors.Is(err, ErrResolved):
		response.Conflict(w, r, response.CodeQRSessionResolved, "This sign-in request was already answered")
	case errors.Is(err, ErrClaimed):
		response.Conflict(w, r, response.CodeQRSessionClaimed, "This QR code was opened by another account")
	case errors.Is(err, ErrInvalidSecret):
		response.BadRequest(w, r, response.CodeQRLoginInvalid, "QR sign-in could not be verified")
	case errors.Is(err, ErrImpersonating):
		response.Forbidden(w, r, "Stop impersonating before using QR sign-in")
	case errors.Is(err, ErrStepUpRequired):
		response.StepUpRequired(w, r)
	case errors.Is(err, ErrRealtimeDisabled):
		response.ServiceUnavailable(w, r, response.CodeRealtimeDisabled, "Realtime is disabled")
	case errors.Is(err, authusecase.ErrUserDisabled):
		response.Forbidden(w, r, "User is disabled")
	case errors.Is(err, authusecase.ErrInvalidCredentials):
		response.Unauthorized(w, r, "Account is no longer available")
	default:
		response.InternalErr(w, r, err, "Unexpected server error")
	}
}
