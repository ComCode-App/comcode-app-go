package usecase

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/repository"
	notifmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/errreport"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/logredact"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/reqid"
)

const (
	otpTypeLoginCode = "login_code"
	// emailCodeTTL is the lifetime of an email sign-in code.
	emailCodeTTL = 10 * time.Minute
	// loginCodeTemplate is the security email that carries the code (sealed, TEC-1099).
	loginCodeTemplate = "auth.login_code"
)

// ErrInvalidEmailCode covers wrong, expired, exhausted or already used email sign-in codes,
// and unknown emails (TEC-1121).
var ErrInvalidEmailCode = errors.New("invalid email code")

// SetReviewAccounts installs the App Store / Play review allowlist (email -> fixed 6-digit
// code, AUTH_REVIEW_ACCOUNTS, validated by config at boot). nil / empty disables it (TEC-1182).
//
// A fixed code signs in only its own email, only an existing active account, and never
// creates one. Request for such an account sends no email; MFA and the handler's rate limits
// apply as for any email code.
func (u *AuthUseCase) SetReviewAccounts(accounts map[string]string) {
	out := make(map[string]string, len(accounts))
	for email, code := range accounts {
		email = normalizeEmail(email)
		code = strings.TrimSpace(code)
		if email != "" && code != "" {
			out[email] = code
		}
	}
	u.reviewAccounts = out
}

// isReviewAccount reports whether user is a configured review account that may use its fixed
// code: the email is listed and the account exists and is active.
func (u *AuthUseCase) isReviewAccount(user model.User) bool {
	if user.ID == 0 || user.Status != "active" {
		return false
	}
	_, ok := u.reviewAccounts[normalizeEmail(user.Email)]
	return ok
}

// reviewCodeMatches compares code with user's fixed review code in constant time.
func (u *AuthUseCase) reviewCodeMatches(user model.User, code string) bool {
	if !u.isReviewAccount(user) || code == "" {
		return false
	}
	fixed := u.reviewAccounts[normalizeEmail(user.Email)]
	return subtle.ConstantTimeCompare([]byte(fixed), []byte(code)) == 1
}

// emailFingerprint is a short, non-reversible tag for an email in audit logs.
func emailFingerprint(email string) string {
	sum := sha256.Sum256([]byte(normalizeEmail(email)))
	return hex.EncodeToString(sum[:6])
}

// RequestLoginCode emails a one-time sign-in code (TEC-1121). From the caller's point of view
// it always succeeds for a well-formed email, so the response never reveals whether the email
// belongs to an account:
//   - known email (any status): a code is sent; verify reports the account state only after
//     the mailbox is proven (TEC-1101);
//   - unknown email with self-registration open: a sign-up code is sent;
//   - unknown email with registration closed: nothing is sent.
//
// A failure while issuing the code or enqueuing the email is logged and reported instead of
// returned, like ForgotPassword (TEC-1116).
func (u *AuthUseCase) RequestLoginCode(ctx context.Context, email string) error {
	email = normalizeEmail(email)
	if email == "" || !strings.Contains(email, "@") {
		return fmt.Errorf("%w: a valid email is required", ErrInvalidRequest)
	}
	if err := u.sendLoginCode(ctx, email); err != nil {
		u.reportLoginCodeFailure(ctx, err)
	}
	return nil
}

func (u *AuthUseCase) sendLoginCode(ctx context.Context, email string) error {
	user, err := u.repo.FindUserByEmail(ctx, email)
	if err != nil {
		if !errors.Is(err, repository.ErrNotFound) {
			return fmt.Errorf("find user: %w", err)
		}
		if !u.selfRegistrationOpen(ctx) {
			if u.log != nil {
				u.log.InfoContext(ctx, "auth_email_code_unknown_account")
			}
			return nil
		}
		// Sign-up by email code: the code proves the mailbox and verify creates the account.
		return u.issueLoginCode(ctx, model.User{Email: email, Name: emailLocalPart(email)})
	}
	if u.isReviewAccount(user) {
		// The store reviewer signs in with the fixed code; nothing is mailed (TEC-1182).
		if u.log != nil {
			u.log.InfoContext(ctx, "auth_review_account_code_request", "user_id", user.ID)
		}
		return nil
	}
	return u.issueLoginCode(ctx, user)
}

// issueLoginCode replaces any active login code for the address and emails a fresh one.
// user.ID is 0 for a sign-up code (no account yet).
func (u *AuthUseCase) issueLoginCode(ctx context.Context, user model.User) error {
	code, err := generateOTP()
	if err != nil {
		return fmt.Errorf("generate otp: %w", err)
	}
	email := normalizeEmail(user.Email)
	_ = u.repo.InvalidateOTPs(ctx, email, otpTypeLoginCode)
	var uidPtr *int64
	if user.ID != 0 {
		uid := user.ID
		uidPtr = &uid
	}
	if err := u.repo.CreateOTP(ctx, uidPtr, email, hashOTP(code), otpTypeLoginCode, u.now().UTC().Add(emailCodeTTL)); err != nil {
		return fmt.Errorf("create otp: %w", err)
	}
	if u.log != nil {
		u.log.Debug("otp_issued", "type", otpTypeLoginCode)
	}
	if u.notifier == nil {
		return nil
	}
	to := email
	_, err = u.notifier.Enqueue(ctx, notifmodel.EnqueueInput{
		UserID: uidPtr, Channels: []string{notifmodel.ChannelEmail},
		TemplateCode: loginCodeTemplate, SourceEvent: loginCodeTemplate,
		// SecurityEmail: ignores preferences and stores the code only sealed (TEC-1099).
		SecurityEmail: true, Recipient: &to, Language: user.Locale,
		TemplateVars: map[string]string{
			"name": user.Name, "code": code, "expires_minutes": strconv.Itoa(int(emailCodeTTL.Minutes())),
		},
	})
	if err != nil {
		return fmt.Errorf("enqueue login code email: %w", err)
	}
	return nil
}

// reportLoginCodeFailure logs a swallowed code-request failure with the request id and sends it
// to the error tracker. The email is never logged; the cause is redacted.
func (u *AuthUseCase) reportLoginCodeFailure(ctx context.Context, err error) {
	log := u.log
	if log == nil {
		log = slog.Default()
	}
	log.ErrorContext(ctx, "login_code_request_failed",
		"request_id", reqid.From(ctx), "error", logredact.Error(err))
	errreport.CaptureError(ctx, err, map[string]string{"auth.flow": "email_code_request"})
}

// VerifyLoginCode exchanges an email code for tokens (TEC-1121).
//
// The code is checked before anything about the account is revealed: a wrong code answers
// ErrInvalidEmailCode for known and unknown emails alike, and a disabled account is reported
// (ErrUserDisabled) only after a valid code. Users with authenticator 2FA get ErrMFARequired on
// the first call; the email code stays valid so the client retries with the same code plus
// totpCode. A wrong totpCode burns one of the email code's attempts.
//
// A configured review account (SetReviewAccounts) may use its fixed code instead of an emailed
// one; the MFA rules above still apply and the email is not marked verified by it.
func (u *AuthUseCase) VerifyLoginCode(ctx context.Context, email, code, totpCode string, meta model.SessionMeta) (model.Tokens, error) {
	email = normalizeEmail(email)
	code = strings.TrimSpace(code)
	if email == "" || code == "" {
		return model.Tokens{}, fmt.Errorf("%w: email and code are required", ErrInvalidRequest)
	}
	user, err := u.repo.FindUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return u.signUpWithEmailCode(ctx, email, code, meta)
		}
		return model.Tokens{}, err
	}
	var otpID int64 // 0 = fixed review code, nothing stored to consume
	reviewCode := u.reviewCodeMatches(user, code)
	if !reviewCode {
		if otpID, err = u.checkEmailCode(ctx, email, code); err != nil {
			return model.Tokens{}, err
		}
	}
	if user.Status != "active" {
		return model.Tokens{}, ErrUserDisabled
	}
	if err := u.checkCodeLoginMFA(ctx, user.ID, totpCode); err != nil {
		u.countFailedSecondFactor(ctx, otpID, err)
		return model.Tokens{}, err
	}
	if reviewCode {
		// Audit trail for every review sign-in; never the code or the email in clear.
		log := u.log
		if log == nil {
			log = slog.Default()
		}
		log.InfoContext(ctx, "auth_review_code_used",
			"user_id", user.ID, "email_sha256", emailFingerprint(email), "request_id", reqid.From(ctx))
	} else if err := u.consumeEmailCode(ctx, otpID); err != nil {
		return model.Tokens{}, err
	}
	if !reviewCode && !user.EmailVerified {
		// The emailed code proves control of the mailbox (a fixed review code does not).
		if updated, err := u.repo.SetEmailVerified(ctx, user.ID); err == nil {
			user = updated
		}
	}
	if err := u.repo.UpdateLastLogin(ctx, user.ID); err != nil {
		return model.Tokens{}, err
	}
	return u.issueTokensForUser(ctx, user, meta)
}

// signUpWithEmailCode creates the account for an unknown email once its code is verified and
// signs it in. With registration closed it answers like a wrong code and creates nothing.
func (u *AuthUseCase) signUpWithEmailCode(ctx context.Context, email, code string, meta model.SessionMeta) (model.Tokens, error) {
	if !u.selfRegistrationOpen(ctx) {
		return model.Tokens{}, ErrInvalidEmailCode
	}
	otpID, err := u.checkEmailCode(ctx, email, code)
	if err != nil {
		return model.Tokens{}, err
	}
	// A new account has no authenticator, so the admin "2FA required" policy would lock it
	// out right away: refuse before creating anything (the code stays usable).
	if err := u.checkCodeLoginMFA(ctx, 0, ""); err != nil {
		return model.Tokens{}, err
	}
	if err := u.consumeEmailCode(ctx, otpID); err != nil {
		return model.Tokens{}, err
	}
	user, err := u.createPasswordlessUser(ctx, email, emailLocalPart(email), "", true)
	if errors.Is(err, ErrConflict) {
		// Created concurrently (double submit): sign in the existing account.
		user, err = u.repo.FindUserByEmail(ctx, email)
		if err == nil && user.Status != "active" {
			err = ErrUserDisabled
		}
	}
	if err != nil {
		return model.Tokens{}, err
	}
	if err := u.repo.UpdateLastLogin(ctx, user.ID); err != nil {
		return model.Tokens{}, err
	}
	return u.issueTokensForUser(ctx, user, meta)
}

// checkCodeLoginMFA enforces the second factor for an email-code sign-in: the user's own
// authenticator 2FA, and the admin policy that requires 2FA for sign-in without passkey or
// OAuth (an email code must not bypass it). userID 0 = an account that does not exist yet.
func (u *AuthUseCase) checkCodeLoginMFA(ctx context.Context, userID int64, totpCode string) error {
	hasTOTP := false
	if userID != 0 {
		var err error
		if hasTOTP, err = u.UserHasEnabledTOTP(ctx, userID); err != nil {
			return err
		}
	}
	if !hasTOTP && u.accessPolicy != nil {
		required, err := u.accessPolicy.PasswordLoginTOTPRequired(ctx)
		if err != nil {
			return err
		}
		if required {
			return ErrMFANotEnrolled
		}
	}
	if !hasTOTP {
		return nil
	}
	code := strings.TrimSpace(totpCode)
	if code == "" {
		return ErrMFARequired
	}
	valid, _, err := u.verifyUserTOTPCode(ctx, userID, code, true)
	if err != nil {
		return err
	}
	if !valid {
		return ErrInvalidMFACode
	}
	return nil
}

func (u *AuthUseCase) selfRegistrationOpen(ctx context.Context) bool {
	if u.authSettings == nil {
		return false
	}
	enabled, err := u.authSettings.RegistrationEnabled(ctx)
	return err == nil && enabled
}

// checkEmailCode validates code against the newest active login code without consuming it.
// A wrong code burns one attempt; the last allowed failure kills the code.
func (u *AuthUseCase) checkEmailCode(ctx context.Context, email, code string) (int64, error) {
	id, codeHash, attempts, maxAttempts, err := u.repo.GetActiveOTP(ctx, email, otpTypeLoginCode)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return 0, ErrInvalidEmailCode
		}
		return 0, err
	}
	if attempts >= maxAttempts {
		_ = u.repo.ConsumeOTP(ctx, id)
		return 0, ErrInvalidEmailCode
	}
	if subtle.ConstantTimeCompare([]byte(hashOTP(code)), []byte(codeHash)) != 1 {
		if n, maxN, err := u.repo.IncrementOTPAttempts(ctx, id); err == nil && n >= maxN {
			_ = u.repo.ConsumeOTP(ctx, id)
		}
		return 0, ErrInvalidEmailCode
	}
	return id, nil
}

// consumeEmailCode marks a checked code used; losing a race to a concurrent verify is a wrong code.
func (u *AuthUseCase) consumeEmailCode(ctx context.Context, id int64) error {
	if err := u.repo.ConsumeOTP(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrInvalidEmailCode
		}
		return err
	}
	return nil
}

// countFailedSecondFactor charges a wrong authenticator code against the email code's attempt
// budget so the pair cannot be brute-forced indefinitely.
func (u *AuthUseCase) countFailedSecondFactor(ctx context.Context, otpID int64, err error) {
	if otpID == 0 || !errors.Is(err, ErrInvalidMFACode) {
		return
	}
	if n, maxN, incErr := u.repo.IncrementOTPAttempts(ctx, otpID); incErr == nil && n >= maxN {
		_ = u.repo.ConsumeOTP(ctx, otpID)
	}
}

// emailLocalPart is the default display name for an email-code sign-up.
func emailLocalPart(email string) string {
	local, _, _ := strings.Cut(email, "@")
	local = strings.TrimSpace(local)
	if r := []rune(local); len(r) > 100 {
		local = string(r[:100])
	}
	return local
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
