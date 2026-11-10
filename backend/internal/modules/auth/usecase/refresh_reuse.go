package usecase

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/repository"
	notifmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/errreport"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/logredact"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/reqid"
)

const (
	// refreshRaceWindow: a rotated refresh token presented again this soon after its rotation
	// is treated as a concurrent refresh of the same client (409 REFRESH_RACE), not a replay.
	refreshRaceWindow = 30 * time.Second
	// refreshReuseTemplate is the security email sent when a session is revoked for reuse.
	refreshReuseTemplate = "auth.refresh_reuse"
)

// ErrRefreshRace: the refresh token was rotated moments ago by a concurrent request. The
// session stays live; the client should use the token the other request received.
var ErrRefreshRace = errors.New("refresh token was just rotated by another request")

// refreshTTL is the refresh token lifetime of a new session of the given client type.
func (u *AuthUseCase) refreshTTL(clientType string) time.Duration {
	if clientType == model.ClientMobile {
		return u.tokens.MobileRefreshTTL()
	}
	return u.tokens.RefreshTTL()
}

// refreshExpiry returns both candidate expiries for a rotation; the repository applies the
// one matching the family's stored client type.
func (u *AuthUseCase) refreshExpiry() model.RefreshExpiry {
	now := u.now().UTC()
	return model.RefreshExpiry{
		Web:    now.Add(u.refreshTTL(model.ClientWeb)),
		Mobile: now.Add(u.refreshTTL(model.ClientMobile)),
	}
}

// checkRefreshReuse classifies a refresh token that could not be rotated and returns the error
// Refresh answers with:
//   - unknown, expired-and-never-rotated, or revoked by logout / session revocation:
//     ErrInvalidCredentials;
//   - rotated less than refreshRaceWindow ago: ErrRefreshRace (the family is not touched);
//   - rotated earlier: reuse of a stolen or leaked token. The whole family is revoked, the
//     owner gets a security email (only when the family was still live, so replays of an
//     already revoked family send nothing), and the answer is ErrInvalidCredentials.
func (u *AuthUseCase) checkRefreshReuse(ctx context.Context, hash string) error {
	rotated, err := u.repo.FindRotatedRefresh(ctx, hash, refreshRaceWindow)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrInvalidCredentials
		}
		return err
	}
	if rotated.WithinRace {
		return ErrRefreshRace
	}
	if err := u.repo.RevokeSession(ctx, rotated.UserID, rotated.FamilyUUID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrInvalidCredentials
		}
		return err
	}
	u.logger().WarnContext(ctx, "auth_refresh_reuse_detected",
		"request_id", reqid.From(ctx), "user_id", rotated.UserID,
		"session", rotated.FamilyUUID.String(), "client_type", rotated.ClientType)
	if err := u.notifyRefreshReuse(ctx, rotated); err != nil {
		u.logger().ErrorContext(ctx, "auth_refresh_reuse_email_failed",
			"request_id", reqid.From(ctx), "error", logredact.Error(err))
		errreport.CaptureError(ctx, err, map[string]string{"auth.flow": "refresh_reuse_email"})
	}
	return ErrInvalidCredentials
}

// notifyRefreshReuse sends the security email (ignores preferences, TEC-1099 path).
func (u *AuthUseCase) notifyRefreshReuse(ctx context.Context, rotated model.RotatedRefresh) error {
	if u.notifier == nil {
		return nil
	}
	user, err := u.repo.FindUserByID(ctx, rotated.UserID)
	if err != nil {
		return err
	}
	uid, email := user.ID, user.Email
	_, err = u.notifier.Enqueue(ctx, notifmodel.EnqueueInput{
		UserID: &uid, Channels: []string{notifmodel.ChannelEmail},
		TemplateCode: refreshReuseTemplate, SourceEvent: refreshReuseTemplate,
		SecurityEmail: true, Recipient: &email, Language: user.Locale,
		TemplateVars: map[string]string{"name": user.Name, "device": reuseDeviceLabel(rotated)},
	})
	return err
}

// reuseDeviceLabel names the revoked session in the email: the app's device name, else the
// browser user agent, else the client type.
func reuseDeviceLabel(r model.RotatedRefresh) string {
	switch {
	case r.DeviceName != nil && *r.DeviceName != "":
		return *r.DeviceName
	case r.UserAgent != nil && *r.UserAgent != "":
		ua := []rune(*r.UserAgent)
		if len(ua) > 120 {
			ua = append(ua[:120], '…')
		}
		return string(ua)
	default:
		return r.ClientType
	}
}

func (u *AuthUseCase) logger() *slog.Logger {
	if u.log != nil {
		return u.log
	}
	return slog.Default()
}
