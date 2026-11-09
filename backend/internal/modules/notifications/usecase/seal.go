package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/providers"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/templates"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/crypto"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// SecretMask replaces a secret template var on the stored row (TEC-1099). Migration 000072
// uses the same literal.
const SecretMask = "••••••"

// DefaultSecretVars are the template vars a SecurityEmail never stores readable.
var DefaultSecretVars = []string{"code", "token", "link", "reset_link", "reset_url", "verification_url", "invite_url"}

// ErrSecretBoxMissing means a notification carries a secret but no encryption key is set,
// so it is refused rather than stored in plaintext.
var ErrSecretBoxMissing = errors.New("notifications: secret box not configured")

// devEncryptionKey is the fallback config.Load uses when APP_ENCRYPTION_KEY is unset.
const devEncryptionKey = "app-dev-encryption-key-32bytes!!"

// NewSecretBoxForEnv returns the box secret notifications are sealed with. Outside
// development and test, a missing key or the built-in development key returns nil (and a
// reason): sealing with a public key protects nothing, and with a nil box the service
// refuses notifications that carry secrets instead of storing them readable.
func NewSecretBoxForEnv(appEnv, key string) (*crypto.SecretBox, error) {
	key = strings.TrimSpace(key)
	dev := appEnv == "development" || appEnv == "test" || appEnv == ""
	if !dev && (key == "" || key == devEncryptionKey) {
		return nil, errors.New("APP_ENCRYPTION_KEY is not configured; security emails are disabled")
	}
	return crypto.NewSecretBox(key)
}

// sealedContent is what a sealed email must still deliver.
type sealedContent struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// secretValues returns the values of the secret template vars of in.
func secretValues(in model.EnqueueInput) []string {
	names := in.SecretVars
	if in.SecurityEmail {
		names = append(append([]string{}, DefaultSecretVars...), names...)
	}
	var out []string
	seen := map[string]bool{}
	for _, name := range names {
		v := strings.TrimSpace(in.TemplateVars[name])
		// Very short values would mask unrelated text; real secrets are codes and links.
		if len(v) < 4 || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

// maskedVars returns the template vars with every secret var replaced by SecretMask.
func maskedVars(in model.EnqueueInput) map[string]string {
	secret := map[string]bool{}
	for _, n := range in.SecretVars {
		secret[n] = true
	}
	if in.SecurityEmail {
		for _, n := range DefaultSecretVars {
			secret[n] = true
		}
	}
	out := make(map[string]string, len(in.TemplateVars))
	for k, v := range in.TemplateVars {
		if secret[k] && v != "" {
			v = SecretMask
		}
		out[k] = v
	}
	return out
}

// maskSecrets removes every secret value from s, wherever it came from (a template that
// uses the secret under another var name, or a caller-supplied title/body).
func maskSecrets(s string, secrets []string) string {
	for _, v := range secrets {
		s = strings.ReplaceAll(s, v, SecretMask)
	}
	return s
}

// renderMasked renders tpl with the secret vars masked.
func renderMasked(subject, body string, in model.EnqueueInput, secrets []string) (string, string) {
	vars := maskedVars(in)
	return maskSecrets(templates.Render(subject, vars), secrets), maskSecrets(templates.Render(body, vars), secrets)
}

// sealContent encrypts the real email for the row's sealed_content column. Without a box
// it fails rather than storing the secret readable.
func (s *Service) sealContent(title, body string) (pgtype.Text, error) {
	if s.box == nil {
		return pgtype.Text{}, ErrSecretBoxMissing
	}
	raw, err := json.Marshal(sealedContent{Title: title, Body: body})
	if err != nil {
		return pgtype.Text{}, fmt.Errorf("seal notification: %w", err)
	}
	enc, err := s.box.Encrypt(string(raw))
	if err != nil {
		return pgtype.Text{}, fmt.Errorf("seal notification: %w", err)
	}
	return pgtype.Text{String: enc, Valid: true}, nil
}

// deliverVia hands the provider the real content of a sealed row. row is a copy: the
// plaintext never reaches the caller, the history or a status write.
func (s *Service) deliverVia(ctx context.Context, p providers.Provider, row db.Notification, userUUID *uuid.UUID) (providers.DeliveryResult, error) {
	if row.SealedContent.Valid && row.SealedContent.String != "" {
		if s.box == nil {
			return providers.DeliveryResult{}, ErrSecretBoxMissing
		}
		raw, err := s.box.Decrypt(row.SealedContent.String)
		if err != nil {
			return providers.DeliveryResult{}, fmt.Errorf("open sealed notification: %w", err)
		}
		var c sealedContent
		if err := json.Unmarshal([]byte(raw), &c); err != nil {
			return providers.DeliveryResult{}, fmt.Errorf("open sealed notification: %w", err)
		}
		row.Title, row.Body = c.Title, c.Body
	}
	return p.Deliver(ctx, row, userUUID)
}

// WithEncryptionKey sets the secret box from APP_ENV and APP_ENCRYPTION_KEY. When the key
// is unusable it logs and leaves the box unset, so notifications that carry secrets fail
// closed while everything else keeps working.
func (s *Service) WithEncryptionKey(appEnv, key string) *Service {
	box, err := NewSecretBoxForEnv(appEnv, key)
	if err != nil {
		s.log.Error("notifications_secret_box_disabled", "error", err)
		box = nil
	}
	return s.WithSecretBox(box)
}
