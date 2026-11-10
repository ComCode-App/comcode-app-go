package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/model"
	notifmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/model"
)

type reuseNotifier struct{ sent []notifmodel.EnqueueInput }

func (n *reuseNotifier) Enqueue(_ context.Context, in notifmodel.EnqueueInput) ([]notifmodel.Notification, error) {
	n.sent = append(n.sent, in)
	return nil, nil
}

// ageRotation moves the rotation of the session holding hash out of the race window.
func (r *memRepo) ageRotation(hash string, by time.Duration) {
	for _, s := range r.sessions {
		if s.hash == hash {
			s.rotatedAt = s.rotatedAt.Add(-by)
		}
	}
}

// A rotated token replayed after the race window revokes the whole family and sends the
// security email; the same replay again (family already dead) sends nothing more.
func TestRefreshReuseAfterRaceWindowRevokesFamily(t *testing.T) {
	f := newSessionFixture(t)
	notif := &reuseNotifier{}
	f.uc.SetNotifier(notif)
	first := f.login(t)
	second, err := f.uc.Refresh(context.Background(), first.RefreshToken, model.SessionMeta{})
	if err != nil {
		t.Fatal(err)
	}
	f.repo.ageRotation(tokenHash(first.RefreshToken), refreshRaceWindow+time.Second)

	if _, err := f.uc.Refresh(context.Background(), first.RefreshToken, model.SessionMeta{}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("replay: expected ErrInvalidCredentials, got %v", err)
	}
	if _, err := f.uc.Refresh(context.Background(), second.RefreshToken, model.SessionMeta{}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("successor must be revoked with the family, got %v", err)
	}
	if _, err := f.principal(t, second.AccessToken); !errors.Is(err, ErrSessionRevoked) {
		t.Fatalf("access token of the family: expected ErrSessionRevoked, got %v", err)
	}
	if len(notif.sent) != 1 {
		t.Fatalf("security emails = %d, want 1", len(notif.sent))
	}
	in := notif.sent[0]
	if in.TemplateCode != refreshReuseTemplate || !in.SecurityEmail || in.Recipient == nil || *in.Recipient != "member@example.com" {
		t.Fatalf("email = %+v", in)
	}
	if _, err := f.uc.Refresh(context.Background(), first.RefreshToken, model.SessionMeta{}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("second replay: %v", err)
	}
	if len(notif.sent) != 1 {
		t.Fatalf("replay of a revoked family must not email again: %d", len(notif.sent))
	}
}

// A token revoked by logout (never rotated) is just invalid: no email.
func TestRefreshAfterLogoutIsPlainInvalid(t *testing.T) {
	f := newSessionFixture(t)
	notif := &reuseNotifier{}
	f.uc.SetNotifier(notif)
	tok := f.login(t)
	if err := f.uc.Logout(context.Background(), tok.RefreshToken); err != nil {
		t.Fatal(err)
	}
	if _, err := f.uc.Refresh(context.Background(), tok.RefreshToken, model.SessionMeta{}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
	if len(notif.sent) != 0 {
		t.Fatalf("logout must not send a reuse email")
	}
}

// Mobile sessions get the mobile lifetime at login and keep it on refresh; a refresh request
// claiming to be mobile cannot extend a web session.
func TestMobileRefreshTTL(t *testing.T) {
	f := newSessionFixture(t)
	if err := f.tokens.SetMobileRefreshTTL(48 * time.Hour); err != nil {
		t.Fatal(err)
	}
	mobile := model.SessionMeta{ClientType: model.ClientMobile, DeviceName: "Pixel 9", AppVersion: "1.0.0"}
	tok, err := f.uc.Login(context.Background(), "member@example.com", "Password1!", "", mobile)
	if err != nil {
		t.Fatal(err)
	}
	if d := time.Until(tok.RefreshExpiresAt); d < 47*time.Hour || d > 48*time.Hour {
		t.Fatalf("mobile login expiry in %v, want ~48h", d)
	}
	next, err := f.uc.Refresh(context.Background(), tok.RefreshToken, model.SessionMeta{})
	if err != nil {
		t.Fatal(err)
	}
	if d := time.Until(next.RefreshExpiresAt); d < 47*time.Hour {
		t.Fatalf("mobile refresh expiry in %v, want ~48h", d)
	}

	web := f.login(t)
	if d := time.Until(web.RefreshExpiresAt); d > time.Hour {
		t.Fatalf("web login expiry in %v, want <= 1h", d)
	}
	upgraded, err := f.uc.Refresh(context.Background(), web.RefreshToken, mobile)
	if err != nil {
		t.Fatal(err)
	}
	if d := time.Until(upgraded.RefreshExpiresAt); d > time.Hour {
		t.Fatalf("refresh must keep the web lifetime, got %v", d)
	}
}
