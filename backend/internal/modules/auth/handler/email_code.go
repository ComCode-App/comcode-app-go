package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
)

// Email sign-in code limits (TEC-1121), per 15-minute window. Codes are 6 digits with 5 attempts
// each; the per-email caps hold across IPs so many addresses cannot guess one account's code.
const (
	emailCodeRequestIPLimit    = 10
	emailCodeRequestEmailLimit = 5
	emailCodeVerifyIPLimit     = 30
	emailCodeVerifyEmailLimit  = 10
	emailCodeRateWindow        = 15 * time.Minute
)

type rateCheck struct {
	action  string
	subject string
	limit   int
}

// allowRates applies every check in order and writes 429 + Retry-After on the first denial.
func (h *Handler) allowRates(w http.ResponseWriter, r *http.Request, checks ...rateCheck) bool {
	if h.limiter == nil {
		return true
	}
	for _, c := range checks {
		if strings.TrimSpace(c.subject) == "" {
			continue
		}
		ok, retry := h.limiter.Allow(r.Context(), c.action, c.subject, c.limit, emailCodeRateWindow)
		if h.writeRateLimited(w, r, ok, retry) {
			return false
		}
	}
	return true
}

func emailSubject(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// RequestEmailCode is POST /v1/auth/email-code/request. Known and unknown emails get the
// identical accepted response.
func (h *Handler) RequestEmailCode(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email string `json:"email"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}
	if !h.allowRates(w, r,
		rateCheck{"email_code_request_ip", sessionMeta(r).IP, emailCodeRequestIPLimit},
		rateCheck{"email_code_request_email", emailSubject(in.Email), emailCodeRequestEmailLimit},
	) {
		return
	}
	if err := h.uc.RequestLoginCode(r.Context(), in.Email); err != nil {
		writeUsecaseError(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, map[string]string{"status": "accepted"})
}

// VerifyEmailCode is POST /v1/auth/email-code/verify; it returns a token pair like login.
func (h *Handler) VerifyEmailCode(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		Code     string `json:"code"`
		TOTPCode string `json:"totp_code"`
	}
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}
	if !h.allowRates(w, r,
		rateCheck{"email_code_verify_ip", sessionMeta(r).IP, emailCodeVerifyIPLimit},
		rateCheck{"email_code_verify_email", emailSubject(in.Email), emailCodeVerifyEmailLimit},
	) {
		return
	}
	tokens, err := h.uc.VerifyLoginCode(r.Context(), in.Email, in.Code, in.TOTPCode, sessionMeta(r))
	if err != nil {
		writeUsecaseError(w, r, err)
		return
	}
	response.JSON(w, r, http.StatusOK, tokens)
}
