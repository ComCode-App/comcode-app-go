package usecase

import (
	"context"
	"errors"
	"log/slog"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/repository"
	notifmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/reqid"
	"github.com/google/uuid"
)

// qrLoginApprovedTemplate is the security email + in-app notice sent after a QR sign-in approval
// (TEC-1184). Seeded by migration 000081.
const qrLoginApprovedTemplate = "auth.qr_login_approved"

// qrLoginActionURL is where the in-app notice points: the account page lists and revokes sessions.
const qrLoginActionURL = "/account"

// QRLoginUser is the user who approved a QR sign-in and receives the web session.
type QRLoginUser struct {
	UUID  uuid.UUID
	Email string
	Name  string
}

// QRLoginNotice describes the approved browser for the security notification. All fields are
// display hints taken from the browser's request (User-Agent, client IP, GeoIP).
type QRLoginNotice struct {
	Device   string
	IP       string
	Location string
}

// CheckQRLoginApprover reports whether the user may approve a QR sign-in: the user must exist and
// be active (a disabled or pending user gets ErrUserDisabled).
func (u *AuthUseCase) CheckQRLoginApprover(ctx context.Context, userUUID uuid.UUID) error {
	_, err := u.qrLoginUser(ctx, userUUID)
	return err
}

// IssueQRLoginSession issues a web session for the user who approved a QR sign-in on their phone.
// Like every sign-in, the session starts on the user's personal account (issueTokensForUser); the
// approving phone's active account is not carried over. The phone already holds a signed-in
// session, so no second factor is asked again (same model as native / OAuth sign-in).
func (u *AuthUseCase) IssueQRLoginSession(ctx context.Context, userUUID uuid.UUID, meta model.SessionMeta) (model.Tokens, QRLoginUser, error) {
	user, err := u.qrLoginUser(ctx, userUUID)
	if err != nil {
		return model.Tokens{}, QRLoginUser{}, err
	}
	// Always a web session (the browser that showed the QR), whatever the caller passes.
	meta.ImpersonatorUserID = nil
	meta.ActiveAccountID = nil
	meta.ClientType = model.ClientWeb
	meta.DeviceName, meta.AppVersion = "", ""
	if err := u.repo.UpdateLastLogin(ctx, user.ID); err != nil {
		return model.Tokens{}, QRLoginUser{}, err
	}
	tokens, err := u.issueTokensForUser(ctx, user, meta)
	if err != nil {
		return model.Tokens{}, QRLoginUser{}, err
	}
	return tokens, QRLoginUser{UUID: user.UUID, Email: user.Email, Name: user.Name}, nil
}

// NotifyQRLoginApproved sends the "a web sign-in was approved" security email (it ignores the
// email preference, like the other security emails) and an in-app notice. Best effort: a failure
// is logged and never undoes the approval.
func (u *AuthUseCase) NotifyQRLoginApproved(ctx context.Context, userUUID uuid.UUID, notice QRLoginNotice) {
	if u.notifier == nil {
		return
	}
	for _, v := range []*string{&notice.Device, &notice.IP, &notice.Location} {
		if *v == "" {
			*v = "-"
		}
	}
	user, err := u.repo.FindUserByUUID(ctx, userUUID)
	if err == nil {
		uid := user.ID
		action := qrLoginActionURL
		_, err = u.notifier.Enqueue(ctx, notifmodel.EnqueueInput{
			UserID:       &uid,
			Channels:     []string{notifmodel.ChannelEmail, notifmodel.ChannelInapp},
			Priority:     notifmodel.PriorityHigh,
			TemplateCode: qrLoginApprovedTemplate, SourceEvent: qrLoginApprovedTemplate,
			SecurityEmail: true, Language: user.Locale, ActionURL: &action,
			TemplateVars: map[string]string{
				"name": user.Name, "device": notice.Device, "ip": notice.IP, "location": notice.Location,
			},
		})
	}
	if err != nil {
		log := u.log
		if log == nil {
			log = slog.Default()
		}
		log.WarnContext(ctx, "qrlogin_notify_failed", "request_id", reqid.From(ctx), "error", err)
	}
}

func (u *AuthUseCase) qrLoginUser(ctx context.Context, userUUID uuid.UUID) (model.User, error) {
	user, err := u.repo.FindUserByUUID(ctx, userUUID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.User{}, ErrInvalidCredentials
		}
		return model.User{}, err
	}
	if user.Status != "active" {
		return model.User{}, ErrUserDisabled
	}
	return user, nil
}
