// Package qrlogin implements "sign in on the web by scanning a QR with the signed-in mobile app"
// (TEC-1184, docs/design/mobile.md §3). One direction only: the web login page shows the QR and
// the phone approves (KARAR-6).
//
// Flow: the web login page creates a session (POST /v1/auth/qr/sessions) and gets an unguessable
// session id (encoded in the QR as {frontend}/login/qr/{id}), a browser secret that never leaves
// the tab, and an anonymous Centrifugo connection + subscription token for one channel
// (qrlogin:{random channel id}). The phone opens the session (claiming it), then approves or
// rejects; the backend publishes the outcome to the channel. On approval the tab receives a
// one-time exchange token and the web server trades it, with the browser secret, for a normal
// session on the approver's personal account (POST /v1/auth/qr/exchange). State lives in Redis
// only, with short TTLs.
package qrlogin

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/model"
	authusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/geoip"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/realtime"
	"github.com/google/uuid"
)

const (
	// SessionTTL is how long a QR stays valid; the web page creates a new one after that.
	SessionTTL = 2 * time.Minute
	// ExchangeTTL is the window the web server has to redeem an approval.
	ExchangeTTL = time.Minute
	// realtimeGrace keeps the channel tokens valid slightly past the session so a late approval
	// still reaches the tab.
	realtimeGrace = 30 * time.Second
	// MaxSecretAttempts deletes the session after this many wrong browser secrets / tokens.
	MaxSecretAttempts = 5
	secretBytes       = 32
	// geoTimeout bounds the location lookup on create; location is only a hint.
	geoTimeout = 500 * time.Millisecond
)

var (
	// ErrRealtimeDisabled is returned when Centrifugo is not configured.
	ErrRealtimeDisabled = errors.New("realtime is disabled")
	// ErrImpersonating blocks QR sign-in actions from impersonation sessions.
	ErrImpersonating = errors.New("qr login is not available while impersonating")
	// ErrStepUpRequired means a user with platform permissions must re-authenticate before
	// approving (KARAR-15).
	ErrStepUpRequired = errors.New("step-up required to approve a qr login")
)

var idPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)

// ValidID reports whether s has the shape of a generated session id / secret / token.
func ValidID(s string) bool { return idPattern.MatchString(s) }

// Accounts is the slice of the auth use case QR sign-in needs.
type Accounts interface {
	CheckQRLoginApprover(ctx context.Context, userUUID uuid.UUID) error
	IssueQRLoginSession(ctx context.Context, userUUID uuid.UUID, meta model.SessionMeta) (model.Tokens, authusecase.QRLoginUser, error)
	NotifyQRLoginApproved(ctx context.Context, userUUID uuid.UUID, notice authusecase.QRLoginNotice)
}

// StepUp reports whether a user holds a fresh step-up grant (platform/stepup).
type StepUp interface {
	HasValidGrant(ctx context.Context, userUUID uuid.UUID) (bool, error)
}

// TokenIssuer mints anonymous Centrifugo tokens.
type TokenIssuer interface {
	Enabled() bool
	WSURL() string
	AnonymousConnectionToken(exp time.Time) (string, error)
	AnonymousSubscriptionToken(channel string, exp time.Time) (string, error)
}

// Location is the approximate location of the browser that asked for the QR (GeoIP engine).
type Location struct {
	CountryCode string `json:"country_code,omitempty"`
	Country     string `json:"country,omitempty"`
	City        string `json:"city,omitempty"`
}

// Empty reports whether nothing is known.
func (l Location) Empty() bool { return l.CountryCode == "" && l.Country == "" && l.City == "" }

// String is "City, Country" for notifications.
func (l Location) String() string {
	parts := make([]string, 0, 2)
	if l.City != "" {
		parts = append(parts, l.City)
	}
	if l.Country != "" {
		parts = append(parts, l.Country)
	} else if l.CountryCode != "" {
		parts = append(parts, l.CountryCode)
	}
	return strings.Join(parts, ", ")
}

// Deps are the collaborators of Service. Geo, StepUp and Publisher may be nil.
type Deps struct {
	Store       *Store
	Accounts    Accounts
	Issuer      TokenIssuer
	Publisher   realtime.Publisher
	Geo         geoip.GeoIPResolver
	StepUp      StepUp
	FrontendURL string
	Log         *slog.Logger
}

// Service orchestrates QR sign-in.
type Service struct {
	store       *Store
	accounts    Accounts
	issuer      TokenIssuer
	pub         realtime.Publisher
	geo         geoip.GeoIPResolver
	stepUp      StepUp
	frontendURL string
	log         *slog.Logger
	now         func() time.Time
}

// NewService wires the QR sign-in service.
func NewService(d Deps) *Service {
	pub := d.Publisher
	if pub == nil {
		pub = realtime.NoopPublisher{}
	}
	log := d.Log
	if log == nil {
		log = slog.Default()
	}
	return &Service{
		store: d.Store, accounts: d.Accounts, issuer: d.Issuer, pub: pub, geo: d.Geo, stepUp: d.StepUp,
		frontendURL: strings.TrimRight(d.FrontendURL, "/"), log: log, now: time.Now,
	}
}

// RealtimeGrant is everything the tab needs to listen on its channel.
type RealtimeGrant struct {
	WSURL             string `json:"ws_url"`
	ConnectionToken   string `json:"connection_token"`
	SubscriptionToken string `json:"subscription_token"`
	Channel           string `json:"channel"`
}

// Created is the response to the web tab.
type Created struct {
	SessionID     string        `json:"session_id"`
	QRURL         string        `json:"qr_url"`
	BrowserSecret string        `json:"browser_secret"`
	ExpiresAt     time.Time     `json:"expires_at"`
	ExpiresIn     int64         `json:"expires_in"`
	Realtime      RealtimeGrant `json:"realtime"`
}

// Create starts a QR session for the requesting browser.
func (s *Service) Create(ctx context.Context, userAgent, ip string) (Created, error) {
	if s.issuer == nil || !s.issuer.Enabled() {
		return Created{}, ErrRealtimeDisabled
	}
	sessionID, err := randomToken()
	if err != nil {
		return Created{}, err
	}
	channelID, err := randomToken()
	if err != nil {
		return Created{}, err
	}
	secret, err := randomToken()
	if err != nil {
		return Created{}, err
	}
	now := s.now().UTC()
	expires := now.Add(SessionTTL)
	channel := realtime.QRLoginChannel(channelID)
	tokenExp := expires.Add(realtimeGrace)
	connToken, err := s.issuer.AnonymousConnectionToken(tokenExp)
	if err != nil {
		return Created{}, err
	}
	subToken, err := s.issuer.AnonymousSubscriptionToken(channel, tokenExp)
	if err != nil {
		return Created{}, err
	}
	userAgent = strings.TrimSpace(userAgent)
	if len(userAgent) > maxUserAgentLen {
		userAgent = userAgent[:maxUserAgentLen]
	}
	if err := s.store.Create(ctx, sessionID, Session{
		ChannelID: channelID, SecretHash: hashSecret(secret),
		UserAgent: userAgent, IP: ip, Location: s.locate(ctx, ip),
		CreatedAt: now, ExpiresAt: expires,
	}, SessionTTL); err != nil {
		return Created{}, err
	}
	return Created{
		SessionID:     sessionID,
		QRURL:         fmt.Sprintf("%s/login/qr/%s", s.frontendURL, sessionID),
		BrowserSecret: secret,
		ExpiresAt:     expires,
		ExpiresIn:     int64(SessionTTL.Seconds()),
		Realtime: RealtimeGrant{
			WSURL: s.issuer.WSURL(), ConnectionToken: connToken,
			SubscriptionToken: subToken, Channel: channel,
		},
	}, nil
}

// locate resolves the approximate location with the GeoIP engine; failures leave it empty.
func (s *Service) locate(ctx context.Context, ip string) Location {
	parsed := net.ParseIP(ip)
	if s.geo == nil || parsed == nil {
		return Location{}
	}
	ctx, cancel := context.WithTimeout(ctx, geoTimeout)
	defer cancel()
	loc, err := s.geo.Resolve(ctx, parsed)
	if err != nil || loc == nil {
		return Location{}
	}
	return Location{CountryCode: loc.CountryCode, Country: loc.CountryName, City: loc.City}
}

// Details is what the phone shows before approving.
type Details struct {
	SessionID string     `json:"session_id"`
	Status    string     `json:"status"`
	Client    ClientInfo `json:"client"`
	IP        string     `json:"ip"`
	Location  *Location  `json:"location"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt time.Time  `json:"expires_at"`
}

// Approver is the signed-in phone user.
type Approver struct {
	UserUUID      uuid.UUID
	Impersonating bool
	// PlatformUser is true for super admins and users holding any platform.* permission; they
	// need a fresh step-up grant to approve (KARAR-15).
	PlatformUser bool
}

// Get returns session details and claims the session for the viewer.
func (s *Service) Get(ctx context.Context, sessionID string, viewer Approver) (Details, error) {
	if viewer.Impersonating {
		return Details{}, ErrImpersonating
	}
	if !ValidID(sessionID) {
		return Details{}, ErrNotFound
	}
	first, err := s.store.Claim(ctx, sessionID, viewer.UserUUID.String())
	if err != nil {
		return Details{}, err
	}
	sess, err := s.store.Get(ctx, sessionID)
	if err != nil {
		return Details{}, err
	}
	if first {
		s.publish(ctx, sess.ChannelID, map[string]any{"type": "scanned"})
	}
	status := sess.Status
	if status == StatusClaimed {
		status = StatusPending
	}
	d := Details{
		SessionID: sessionID, Status: status, Client: ParseUserAgent(sess.UserAgent),
		IP: sess.IP, CreatedAt: sess.CreatedAt, ExpiresAt: sess.ExpiresAt,
	}
	if !sess.Location.Empty() {
		loc := sess.Location
		d.Location = &loc
	}
	return d, nil
}

// Approve signs the waiting tab in as the approver.
func (s *Service) Approve(ctx context.Context, sessionID string, approver Approver) error {
	if approver.Impersonating {
		return ErrImpersonating
	}
	if !ValidID(sessionID) {
		return ErrNotFound
	}
	if err := s.accounts.CheckQRLoginApprover(ctx, approver.UserUUID); err != nil {
		return err
	}
	if approver.PlatformUser {
		if s.stepUp == nil {
			return ErrStepUpRequired
		}
		ok, err := s.stepUp.HasValidGrant(ctx, approver.UserUUID)
		if err != nil {
			return err
		}
		if !ok {
			return ErrStepUpRequired
		}
	}
	sess, err := s.store.Get(ctx, sessionID)
	if err != nil {
		return err
	}
	if !sess.ExpiresAt.IsZero() && s.now().UTC().After(sess.ExpiresAt) {
		return ErrNotFound
	}
	exchange, err := randomToken()
	if err != nil {
		return err
	}
	if err := s.store.Resolve(ctx, sessionID, approver.UserUUID.String(), StatusApproved, exchange, ExchangeTTL); err != nil {
		return err
	}
	s.publish(ctx, sess.ChannelID, map[string]any{"type": "approved", "exchange_token": exchange})
	client := ParseUserAgent(sess.UserAgent)
	s.accounts.NotifyQRLoginApproved(ctx, approver.UserUUID, authusecase.QRLoginNotice{
		Device: client.Label(), IP: sess.IP, Location: sess.Location.String(),
	})
	return nil
}

// Reject tells the waiting tab the sign-in was refused.
func (s *Service) Reject(ctx context.Context, sessionID string, viewer Approver) error {
	if viewer.Impersonating {
		return ErrImpersonating
	}
	if !ValidID(sessionID) {
		return ErrNotFound
	}
	sess, err := s.store.Get(ctx, sessionID)
	if err != nil {
		return err
	}
	if err := s.store.Resolve(ctx, sessionID, viewer.UserUUID.String(), StatusRejected, "", ExchangeTTL); err != nil {
		return err
	}
	s.publish(ctx, sess.ChannelID, map[string]any{"type": "rejected"})
	return nil
}

// State is the tab's view, read once after (re)subscribing so an event published while the socket
// was down is not lost. It is not polled.
type State struct {
	Status        string `json:"status"`
	ExchangeToken string `json:"exchange_token,omitempty"`
}

// State returns the session status to the tab holding browserSecret. A claimed session reads as
// "scanned" (the name of the channel event).
func (s *Service) State(ctx context.Context, sessionID, browserSecret string) (State, error) {
	if !ValidID(sessionID) {
		return State{}, ErrNotFound
	}
	if !ValidID(browserSecret) {
		return State{}, ErrInvalidSecret
	}
	status, token, err := s.store.State(ctx, sessionID, hashSecret(browserSecret), MaxSecretAttempts)
	if err != nil {
		return State{}, err
	}
	if status == StatusClaimed {
		status = "scanned"
	}
	out := State{Status: status}
	if status == StatusApproved {
		out.ExchangeToken = token
	}
	return out, nil
}

// Exchanged is the web session issued for an approved QR.
type Exchanged struct {
	model.Tokens
	User ExchangedUser `json:"user"`
}

// ExchangedUser identifies the signed-in user for NextAuth.
type ExchangedUser struct {
	UUID  string `json:"uuid"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// Exchange redeems an approval exactly once. The session row keeps the User-Agent / IP of the
// browser that showed the QR, not of the caller (the web server).
func (s *Service) Exchange(ctx context.Context, sessionID, browserSecret, exchangeToken string) (Exchanged, error) {
	if !ValidID(sessionID) || !ValidID(browserSecret) || !ValidID(exchangeToken) {
		return Exchanged{}, ErrInvalidSecret
	}
	consumed, err := s.store.Exchange(ctx, sessionID, hashSecret(browserSecret), exchangeToken, MaxSecretAttempts)
	if err != nil {
		return Exchanged{}, err
	}
	userUUID, err := uuid.Parse(consumed.UserUUID)
	if err != nil {
		return Exchanged{}, ErrNotFound
	}
	tokens, user, err := s.accounts.IssueQRLoginSession(ctx, userUUID, model.SessionMeta{
		UserAgent: consumed.UserAgent, IP: consumed.IP,
	})
	if err != nil {
		return Exchanged{}, err
	}
	return Exchanged{
		Tokens: tokens,
		User:   ExchangedUser{UUID: user.UUID.String(), Email: user.Email, Name: user.Name},
	}, nil
}

// publish is best effort: the tab also reads State after subscribing.
func (s *Service) publish(ctx context.Context, channelID string, data map[string]any) {
	if channelID == "" {
		return
	}
	if err := s.pub.Publish(ctx, realtime.QRLoginChannel(channelID), data); err != nil {
		s.log.WarnContext(ctx, "qrlogin_publish_failed", "error", err, "event", data["type"])
	}
}

func randomToken() (string, error) {
	buf := make([]byte, secretBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("qrlogin: random: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
