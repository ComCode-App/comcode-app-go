package usecase

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/leads/model"
	notifmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/mail"
	"github.com/google/uuid"
)

// Visitor copy (TEC-940): after an exchange the visitor gets an email with what they shared, the
// profile owner's vCard and a KVKK "delete my data" link. The link carries a signed token for
// the exchange; deleting the exchange is what makes it single use.

// DeleteTokenTTL is how long the delete link of a visitor copy works.
const DeleteTokenTTL = 365 * 24 * time.Hour

// DeletePagePath is the frontend page the delete link opens (frontend app/kvkk/exchange).
const DeletePagePath = "/kvkk/exchange"

var (
	// ErrInvalidDeleteToken means the token is malformed, forged or expired.
	ErrInvalidDeleteToken = errors.New("invalid or expired delete token")
	// ErrExchangeGone means the exchange of a valid token is already deleted (link used).
	ErrExchangeGone = errors.New("exchange already deleted")
)

// tokenKeyLabel separates the delete-link key from other uses of the configured secret.
const tokenKeyLabel = "comcode/leads/exchange-delete/v1"

// WithVisitorCopy turns the visitor copy email on. secret signs the delete links (a key is
// derived from it); frontendURL hosts the delete page. An empty secret leaves it off.
func (s *Service) WithVisitorCopy(secret, frontendURL string) *Service {
	if secret == "" {
		return s
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(tokenKeyLabel))
	s.copyKey = mac.Sum(nil)
	s.frontendURL = strings.TrimRight(strings.TrimSpace(frontendURL), "/")
	return s
}

// DeleteToken signs a delete link for an exchange: `{uuid}.{unix exp}.{base64url hmac}`.
func (s *Service) DeleteToken(id uuid.UUID, exp time.Time) string {
	payload := id.String() + "." + strconv.FormatInt(exp.Unix(), 10)
	return payload + "." + base64.RawURLEncoding.EncodeToString(s.sign(payload))
}

func (s *Service) sign(payload string) []byte {
	mac := hmac.New(sha256.New, s.copyKey)
	_, _ = mac.Write([]byte(payload))
	return mac.Sum(nil)
}

// verifyDeleteToken returns the exchange of a valid, unexpired token.
func (s *Service) verifyDeleteToken(token string) (uuid.UUID, error) {
	if len(s.copyKey) == 0 || len(token) > 200 {
		return uuid.Nil, ErrInvalidDeleteToken
	}
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return uuid.Nil, ErrInvalidDeleteToken
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(sig, s.sign(parts[0]+"."+parts[1])) {
		return uuid.Nil, ErrInvalidDeleteToken
	}
	exp, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || s.now().Unix() > exp {
		return uuid.Nil, ErrInvalidDeleteToken
	}
	id, err := uuid.Parse(parts[0])
	if err != nil {
		return uuid.Nil, ErrInvalidDeleteToken
	}
	return id, nil
}

// DeleteByToken deletes the exchange of a visitor copy's delete link, with the copy email itself.
// A second use reports ErrExchangeGone.
func (s *Service) DeleteByToken(ctx context.Context, in model.DeleteExchangeInput) error {
	id, err := s.verifyDeleteToken(in.Token)
	if err != nil {
		return err
	}
	ok, err := s.repo.DeleteVisitorExchange(ctx, id)
	if err != nil {
		return err
	}
	if !ok {
		return ErrExchangeGone
	}
	return nil
}

var copyLabels = map[string][5]string{
	"tr": {"Ad Soyad", "E-posta", "Telefon", "Şirket", "Unvan"},
	"en": {"Name", "Email", "Phone", "Company", "Job title"},
}

// sendVisitorCopy queues the copy email. It is fail-soft: the exchange is already stored.
func (s *Service) sendVisitorCopy(ctx context.Context, leadUUID uuid.UUID, in model.ExchangeInput, profileID *int64, to string) {
	to = strings.TrimSpace(to)
	if s.notifier == nil || len(s.copyKey) == 0 || to == "" || profileID == nil {
		return
	}
	lang := in.Locale
	if lang == "" {
		lang = "tr"
	}
	owner, err := s.repo.ProfileName(ctx, *profileID, lang)
	if err != nil {
		s.log.Warn("exchange_copy_owner_failed", "error", err)
	}
	if owner == "" {
		owner = "/" + in.Path
	}
	labels := copyLabels[lang]
	var summary strings.Builder
	for i, v := range []string{in.Name, in.Email, in.Phone, in.Company, in.JobTitle} {
		if v != "" {
			summary.WriteString("- " + labels[i] + ": " + v + "\n")
		}
	}
	link := s.frontendURL + DeletePagePath + "?token=" + url.QueryEscape(s.DeleteToken(leadUUID, s.now().Add(DeleteTokenTTL)))
	if _, err := s.notifier.Enqueue(ctx, notifmodel.EnqueueInput{
		Channels:     []string{notifmodel.ChannelEmail},
		Recipient:    &to,
		Language:     lang,
		TemplateCode: model.ExchangeCopyTemplate,
		TemplateVars: map[string]string{
			"name": in.Name, "owner": owner, "summary": strings.TrimRight(summary.String(), "\n"), "delete_url": link,
		},
		// The vCard is rendered when the email is sent (CopyAttachments); the row stores no file.
		Payload:     map[string]any{"lead_uuid": leadUUID.String(), "vcard_username": in.Path, "vcard_locale": lang},
		SourceEvent: model.ExchangeCopyEvent(leadUUID),
	}); err != nil {
		s.log.Warn("exchange_copy_enqueue_failed", "error", err)
	}
}

// VCardSource renders the visitor vCard of a public profile page (profiles usecase.Service).
type VCardSource interface {
	PublicVCard(ctx context.Context, username, locale string) (content, filename, redirect string, err error)
}

// CopyAttachments attaches the profile owner's vCard to the visitor copy email at send time
// (notifications providers.AttachmentResolver). VCards may be set after construction. A page
// that went away (Gone) sends the email without the card; other errors retry the delivery.
type CopyAttachments struct {
	VCards VCardSource
	Gone   error
}

// Attachments implements providers.AttachmentResolver.
func (c *CopyAttachments) Attachments(ctx context.Context, templateCode string, payload map[string]any) ([]mail.Attachment, error) {
	if templateCode != model.ExchangeCopyTemplate || c == nil || c.VCards == nil {
		return nil, nil
	}
	username, _ := payload["vcard_username"].(string)
	locale, _ := payload["vcard_locale"].(string)
	if username == "" {
		return nil, nil
	}
	content, filename, redirect, err := c.VCards.PublicVCard(ctx, username, locale)
	if err == nil && redirect != "" { // renamed since the exchange
		content, filename, _, err = c.VCards.PublicVCard(ctx, redirect, locale)
	}
	if err != nil {
		if c.Gone != nil && errors.Is(err, c.Gone) {
			return nil, nil
		}
		return nil, err
	}
	if content == "" {
		return nil, nil
	}
	return []mail.Attachment{{Filename: filename, ContentType: "text/vcard; charset=utf-8", Data: []byte(content)}}, nil
}
