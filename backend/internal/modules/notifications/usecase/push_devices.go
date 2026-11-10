package usecase

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Mobile push device registration (TEC-1186, mobile B9). Sending is B10.

var (
	// ErrPushSessionInactive: the access token's session family is missing, revoked or expired,
	// so the device cannot be tied to a live session.
	ErrPushSessionInactive = errors.New("push device needs a live session")
	// ErrPushImpersonation: an impersonating admin must not route the user's pushes to a phone.
	ErrPushImpersonation = errors.New("push devices cannot be changed while impersonating")
)

var expoTokenRE = regexp.MustCompile(`^(ExponentPushToken|ExpoPushToken)\[[^\]\s]{1,200}\]$`)

const (
	pushAppVersionMax = 32
	pushDeviceNameMax = 100
)

// PushDeviceInput registers a mobile device.
type PushDeviceInput struct {
	Token      string
	Platform   string
	Locale     string
	AppVersion string
	DeviceName string
}

// ValidatePushDevice returns field errors (field → code) for a registration.
func ValidatePushDevice(in PushDeviceInput) map[string]string {
	errs := map[string]string{}
	if !expoTokenRE.MatchString(strings.TrimSpace(in.Token)) {
		errs["token"] = "invalid"
	}
	switch strings.ToLower(strings.TrimSpace(in.Platform)) {
	case "ios", "android":
	default:
		errs["platform"] = "invalid"
	}
	return errs
}

type pushDeviceStore interface {
	UpsertPushDeviceForSession(ctx context.Context, arg db.UpsertPushDeviceForSessionParams) (db.PushDevice, error)
	UserHasPushDevices(ctx context.Context, userID int64) (bool, error)
	EnablePushPreference(ctx context.Context, userID int64) error
	DeletePushDeviceForUser(ctx context.Context, arg db.DeletePushDeviceForUserParams) (int64, error)
	ListPushDevicesByUser(ctx context.Context, userID int64) ([]db.PushDevice, error)
}

func (s *Service) pushDevices() (pushDeviceStore, error) {
	st, ok := s.q.(pushDeviceStore)
	if !ok {
		return nil, fmt.Errorf("%w: push devices are not available", ErrInvalidRequest)
	}
	return st, nil
}

// RegisterPushDevice upserts the caller's Expo token for their current session family and
// refreshes locale / app version / last seen. A token another user registered moves to the
// caller. The user's first device turns push_enabled on; later registrations keep the
// preference as it is. Returns the device and the resulting push_enabled.
func (s *Service) RegisterPushDevice(ctx context.Context, p authctx.Principal, in PushDeviceInput) (model.PushDevice, bool, error) {
	if errs := ValidatePushDevice(in); len(errs) > 0 {
		return model.PushDevice{}, false, fmt.Errorf("%w: invalid push device", ErrInvalidRequest)
	}
	if p.ImpersonatorUserID != nil {
		return model.PushDevice{}, false, ErrPushImpersonation
	}
	if p.SessionID == uuid.Nil {
		return model.PushDevice{}, false, ErrPushSessionInactive
	}
	st, err := s.pushDevices()
	if err != nil {
		return model.PushDevice{}, false, err
	}
	hadDevices, err := st.UserHasPushDevices(ctx, p.UserInternal)
	if err != nil {
		return model.PushDevice{}, false, err
	}
	row, err := st.UpsertPushDeviceForSession(ctx, db.UpsertPushDeviceForSessionParams{
		UserID:            p.UserInternal,
		Token:             strings.TrimSpace(in.Token),
		Platform:          strings.ToLower(strings.TrimSpace(in.Platform)),
		Locale:            optText(normalizePushLocale(in.Locale)),
		AppVersion:        optText(truncateRunes(strings.TrimSpace(in.AppVersion), pushAppVersionMax)),
		DeviceName:        optText(truncateRunes(strings.TrimSpace(in.DeviceName), pushDeviceNameMax)),
		SessionFamilyUuid: p.SessionID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return model.PushDevice{}, false, ErrPushSessionInactive
	}
	if err != nil {
		return model.PushDevice{}, false, err
	}
	if !hadDevices {
		if err := st.EnablePushPreference(ctx, p.UserInternal); err != nil {
			return model.PushDevice{}, false, err
		}
	}
	prefs, err := s.GetPreferences(ctx, p.UserInternal)
	if err != nil {
		return model.PushDevice{}, false, err
	}
	return projectPushDevice(row, p.SessionID), prefs.PushEnabled, nil
}

// UnregisterPushDevice deletes the caller's own device. ErrNotFound when the token is unknown
// or belongs to another user (no existence leak).
func (s *Service) UnregisterPushDevice(ctx context.Context, p authctx.Principal, token string) error {
	if p.ImpersonatorUserID != nil {
		return ErrPushImpersonation
	}
	st, err := s.pushDevices()
	if err != nil {
		return err
	}
	n, err := st.DeletePushDeviceForUser(ctx, db.DeletePushDeviceForUserParams{
		UserID: p.UserInternal, Token: strings.TrimSpace(token),
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListPushDevices returns the caller's devices, active ones first.
func (s *Service) ListPushDevices(ctx context.Context, p authctx.Principal) ([]model.PushDevice, error) {
	st, err := s.pushDevices()
	if err != nil {
		return nil, err
	}
	rows, err := st.ListPushDevicesByUser(ctx, p.UserInternal)
	if err != nil {
		return nil, err
	}
	out := make([]model.PushDevice, 0, len(rows))
	for _, row := range rows {
		out = append(out, projectPushDevice(row, p.SessionID))
	}
	return out, nil
}

func projectPushDevice(r db.PushDevice, current uuid.UUID) model.PushDevice {
	d := model.PushDevice{
		UUID: r.Uuid, Token: r.Token, Platform: r.Platform,
		Locale: textOrNil(r.Locale), AppVersion: textOrNil(r.AppVersion), DeviceName: textOrNil(r.DeviceName),
		Current:        current != uuid.Nil && r.SessionFamilyUuid.Valid && uuid.UUID(r.SessionFamilyUuid.Bytes) == current,
		LastSeenAt:     r.LastSeenAt.Time,
		DisabledReason: textOrNil(r.DisabledReason),
		CreatedAt:      r.CreatedAt.Time,
	}
	if r.DisabledAt.Valid {
		t := r.DisabledAt.Time
		d.DisabledAt = &t
		d.Current = false
	}
	return d
}

// normalizePushLocale maps "tr-TR" / "en_US" / "EN" to tr|en; anything else → "".
func normalizePushLocale(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	if len(v) > 2 {
		v = v[:2]
	}
	switch v {
	case "tr", "en":
		return v
	}
	return ""
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func optText(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}

func textOrNil(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	v := t.String
	return &v
}
