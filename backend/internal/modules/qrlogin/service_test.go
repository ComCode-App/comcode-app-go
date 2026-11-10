package qrlogin

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/model"
	authusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/geoip"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type fakeIssuer struct{}

func (fakeIssuer) Enabled() bool { return true }
func (fakeIssuer) WSURL() string { return "wss://rt.example/connection/websocket" }
func (fakeIssuer) AnonymousConnectionToken(time.Time) (string, error) {
	return "conn-token", nil
}
func (fakeIssuer) AnonymousSubscriptionToken(ch string, _ time.Time) (string, error) {
	return "sub:" + ch, nil
}

type event struct {
	channel string
	data    map[string]any
}

type recorder struct {
	mu     sync.Mutex
	events []event
}

func (r *recorder) Publish(_ context.Context, ch string, data any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, _ := data.(map[string]any)
	r.events = append(r.events, event{ch, m})
	return nil
}

func (r *recorder) types() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.events))
	for _, e := range r.events {
		out = append(out, e.data["type"].(string))
	}
	return strings.Join(out, ",")
}

func (r *recorder) exchangeToken(t *testing.T) string {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.events {
		if tok, ok := e.data["exchange_token"].(string); ok {
			return tok
		}
	}
	t.Fatal("no approved event")
	return ""
}

type fakeAccounts struct {
	mu       sync.Mutex
	disabled map[uuid.UUID]bool
	issued   atomic.Int32
	lastMeta model.SessionMeta
	notices  []authusecase.QRLoginNotice
}

func (f *fakeAccounts) CheckQRLoginApprover(_ context.Context, id uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.disabled[id] {
		return authusecase.ErrUserDisabled
	}
	return nil
}

func (f *fakeAccounts) IssueQRLoginSession(_ context.Context, id uuid.UUID, meta model.SessionMeta) (model.Tokens, authusecase.QRLoginUser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.disabled[id] {
		return model.Tokens{}, authusecase.QRLoginUser{}, authusecase.ErrUserDisabled
	}
	f.issued.Add(1)
	f.lastMeta = meta
	return model.Tokens{AccessToken: "at", RefreshToken: "rt", TokenType: "Bearer"},
		authusecase.QRLoginUser{UUID: id, Email: "u@example.com", Name: "U"}, nil
}

func (f *fakeAccounts) NotifyQRLoginApproved(_ context.Context, _ uuid.UUID, n authusecase.QRLoginNotice) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.notices = append(f.notices, n)
}

func (f *fakeAccounts) setDisabled(id uuid.UUID) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.disabled[id] = true
}

type fakeStepUp struct{ granted map[uuid.UUID]bool }

func (f fakeStepUp) HasValidGrant(_ context.Context, id uuid.UUID) (bool, error) {
	return f.granted[id], nil
}

type fakeGeo struct{}

func (fakeGeo) Resolve(_ context.Context, ip net.IP) (*geoip.GeoLocation, error) {
	if ip.String() == "203.0.113.9" {
		return &geoip.GeoLocation{CountryCode: "TR", CountryName: "Turkey", City: "Istanbul"}, nil
	}
	return nil, errors.New("not found")
}

type fixture struct {
	svc    *Service
	mr     *miniredis.Miniredis
	rdb    *redis.Client
	pub    *recorder
	acc    *fakeAccounts
	stepUp fakeStepUp
	user   uuid.UUID
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	pub := &recorder{}
	acc := &fakeAccounts{disabled: map[uuid.UUID]bool{}}
	stepUp := fakeStepUp{granted: map[uuid.UUID]bool{}}
	svc := NewService(Deps{
		Store: NewStore(rdb, "test"), Accounts: acc, Issuer: fakeIssuer{}, Publisher: pub,
		Geo: fakeGeo{}, StepUp: stepUp, FrontendURL: "https://app.example.com/",
	})
	return &fixture{svc: svc, mr: mr, rdb: rdb, pub: pub, acc: acc, stepUp: stepUp, user: uuid.New()}
}

func (f *fixture) approver() Approver { return Approver{UserUUID: f.user} }

func (f *fixture) create(t *testing.T) Created {
	t.Helper()
	c, err := f.svc.Create(context.Background(), "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/128.0.0.0 Safari/537.36", "203.0.113.9")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestCreateShape(t *testing.T) {
	f := newFixture(t)
	c := f.create(t)
	if !ValidID(c.SessionID) || !ValidID(c.BrowserSecret) || c.SessionID == c.BrowserSecret {
		t.Fatalf("bad ids: %+v", c)
	}
	if c.QRURL != "https://app.example.com/login/qr/"+c.SessionID {
		t.Fatalf("qr url %s", c.QRURL)
	}
	if !strings.HasPrefix(c.Realtime.Channel, "qrlogin:") || strings.Contains(c.Realtime.Channel, c.SessionID) {
		t.Fatalf("channel must be private and unrelated to the QR id: %s", c.Realtime.Channel)
	}
	if c.Realtime.SubscriptionToken != "sub:"+c.Realtime.Channel || c.ExpiresIn != 120 {
		t.Fatalf("realtime grant %+v", c)
	}
	if ttl := f.mr.TTL(f.svc.store.key(c.SessionID)); ttl <= 0 || ttl > SessionTTL {
		t.Fatalf("ttl %v", ttl)
	}
	// Redis never stores the raw id or secret.
	for _, k := range f.mr.Keys() {
		if strings.Contains(k, c.SessionID) {
			t.Fatal("raw session id in redis key")
		}
		if v := f.mr.HGet(k, "secret_hash"); v == c.BrowserSecret || v == "" {
			t.Fatalf("secret stored wrong: %q", v)
		}
	}
	sess, err := f.svc.store.Get(context.Background(), c.SessionID)
	if err != nil || sess.Location.City != "Istanbul" || sess.Location.CountryCode != "TR" {
		t.Fatalf("location not recorded: %+v %v", sess, err)
	}
}

func TestRealtimeDisabled(t *testing.T) {
	f := newFixture(t)
	f.svc.issuer = nil
	if _, err := f.svc.Create(context.Background(), "ua", "1.2.3.4"); !errors.Is(err, ErrRealtimeDisabled) {
		t.Fatalf("err = %v", err)
	}
}

func TestClaimApproveExchangeHappyPath(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	c := f.create(t)
	d, err := f.svc.Get(ctx, c.SessionID, f.approver())
	if err != nil {
		t.Fatal(err)
	}
	if d.Status != StatusPending || d.Client.Browser != "Chrome" || d.Client.OS != "Windows" ||
		d.IP != "203.0.113.9" || d.Location == nil || d.Location.City != "Istanbul" {
		t.Fatalf("details %+v", d)
	}
	st, err := f.svc.State(ctx, c.SessionID, c.BrowserSecret)
	if err != nil || st.Status != "scanned" || st.ExchangeToken != "" {
		t.Fatalf("state after claim %+v %v", st, err)
	}
	// A second look by the same user does not announce "scanned" again.
	if _, err := f.svc.Get(ctx, c.SessionID, f.approver()); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Approve(ctx, c.SessionID, f.approver()); err != nil {
		t.Fatal(err)
	}
	if got := f.pub.types(); got != "scanned,approved" {
		t.Fatalf("events %v", got)
	}
	if f.pub.events[1].channel != c.Realtime.Channel {
		t.Fatalf("published to %s", f.pub.events[1].channel)
	}
	exch := f.pub.exchangeToken(t)
	// After approval the session lives for the exchange window only.
	if ttl := f.mr.TTL(f.svc.store.key(c.SessionID)); ttl <= 0 || ttl > ExchangeTTL {
		t.Fatalf("exchange window ttl %v", ttl)
	}
	// Security notice for the approver.
	if len(f.acc.notices) != 1 || f.acc.notices[0].Device != "Chrome 128 · Windows" ||
		f.acc.notices[0].IP != "203.0.113.9" || f.acc.notices[0].Location != "Istanbul, Turkey" {
		t.Fatalf("notices %+v", f.acc.notices)
	}
	// The tab's catch-up read returns the same token.
	st, err = f.svc.State(ctx, c.SessionID, c.BrowserSecret)
	if err != nil || st.Status != StatusApproved || st.ExchangeToken != exch {
		t.Fatalf("state %+v %v", st, err)
	}
	out, err := f.svc.Exchange(ctx, c.SessionID, c.BrowserSecret, exch)
	if err != nil {
		t.Fatal(err)
	}
	if out.AccessToken != "at" || out.User.UUID != f.user.String() {
		t.Fatalf("exchange %+v", out)
	}
	// The web session row describes the browser that showed the QR.
	if f.acc.lastMeta.IP != "203.0.113.9" || !strings.Contains(f.acc.lastMeta.UserAgent, "Chrome") {
		t.Fatalf("meta not carried: %+v", f.acc.lastMeta)
	}
	// Single use.
	if _, err := f.svc.Exchange(ctx, c.SessionID, c.BrowserSecret, exch); !errors.Is(err, ErrNotFound) {
		t.Fatalf("replay err = %v", err)
	}
}

func TestExchangeWindowExpires(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	c := f.create(t)
	if err := f.svc.Approve(ctx, c.SessionID, f.approver()); err != nil {
		t.Fatal(err)
	}
	exch := f.pub.exchangeToken(t)
	f.mr.FastForward(ExchangeTTL + time.Second)
	if _, err := f.svc.Exchange(ctx, c.SessionID, c.BrowserSecret, exch); !errors.Is(err, ErrNotFound) {
		t.Fatalf("late exchange err = %v", err)
	}
	if f.acc.issued.Load() != 0 {
		t.Fatal("no session may be issued after the window")
	}
}

func TestExchangeSecretAttemptLimit(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	c := f.create(t)
	if err := f.svc.Approve(ctx, c.SessionID, f.approver()); err != nil {
		t.Fatal(err)
	}
	exch := f.pub.exchangeToken(t)
	other, _ := randomToken()
	// Someone who only saw the channel event (or the QR) cannot redeem it.
	for i := 0; i < MaxSecretAttempts; i++ {
		if _, err := f.svc.Exchange(ctx, c.SessionID, other, exch); !errors.Is(err, ErrInvalidSecret) {
			t.Fatalf("attempt %d: %v", i, err)
		}
	}
	// After 5 wrong secrets the session is gone, even for the real tab.
	if _, err := f.svc.Exchange(ctx, c.SessionID, c.BrowserSecret, exch); !errors.Is(err, ErrNotFound) {
		t.Fatalf("burned session err = %v", err)
	}
	if f.acc.issued.Load() != 0 {
		t.Fatal("no session may be issued")
	}
}

func TestStateSecretAttemptLimit(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	c := f.create(t)
	other, _ := randomToken()
	for i := 0; i < MaxSecretAttempts-1; i++ {
		if _, err := f.svc.State(ctx, c.SessionID, other); !errors.Is(err, ErrInvalidSecret) {
			t.Fatalf("attempt %d: %v", i, err)
		}
	}
	// Still alive after 4 wrong secrets.
	if st, err := f.svc.State(ctx, c.SessionID, c.BrowserSecret); err != nil || st.Status != StatusPending {
		t.Fatalf("state %+v %v", st, err)
	}
	if _, err := f.svc.State(ctx, c.SessionID, other); !errors.Is(err, ErrInvalidSecret) {
		t.Fatalf("5th: %v", err)
	}
	if _, err := f.svc.State(ctx, c.SessionID, c.BrowserSecret); !errors.Is(err, ErrNotFound) {
		t.Fatalf("burned session err = %v", err)
	}
	if _, err := f.svc.Get(ctx, c.SessionID, f.approver()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("burned session get = %v", err)
	}
}

func TestExchangeBeforeApprovalAndMalformed(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	c := f.create(t)
	guess, _ := randomToken()
	if _, err := f.svc.Exchange(ctx, c.SessionID, c.BrowserSecret, guess); !errors.Is(err, ErrInvalidSecret) {
		t.Fatalf("pending exchange err = %v", err)
	}
	if _, err := f.svc.Exchange(ctx, c.SessionID, c.BrowserSecret, "short"); !errors.Is(err, ErrInvalidSecret) {
		t.Fatalf("malformed token err = %v", err)
	}
	if _, err := f.svc.Get(ctx, "not-an-id", f.approver()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("malformed id = %v", err)
	}
}

func TestRejectThenApproveConflicts(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	c := f.create(t)
	if err := f.svc.Reject(ctx, c.SessionID, f.approver()); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Approve(ctx, c.SessionID, f.approver()); !errors.Is(err, ErrResolved) {
		t.Fatalf("approve after reject = %v", err)
	}
	if err := f.svc.Reject(ctx, c.SessionID, f.approver()); !errors.Is(err, ErrResolved) {
		t.Fatalf("double reject = %v", err)
	}
	st, err := f.svc.State(ctx, c.SessionID, c.BrowserSecret)
	if err != nil || st.Status != StatusRejected || st.ExchangeToken != "" {
		t.Fatalf("state %+v %v", st, err)
	}
	if got := f.pub.types(); got != "rejected" {
		t.Fatalf("events %v", got)
	}
	if len(f.acc.notices) != 0 {
		t.Fatal("rejection sends no notice")
	}
	// A rejected session can never be exchanged.
	guess, _ := randomToken()
	if _, err := f.svc.Exchange(ctx, c.SessionID, c.BrowserSecret, guess); !errors.Is(err, ErrInvalidSecret) {
		t.Fatalf("exchange rejected = %v", err)
	}
}

func TestConcurrentApproveRejectOnlyOneWins(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	c := f.create(t)
	var wins, conflicts atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var err error
			if i%2 == 0 {
				err = f.svc.Approve(ctx, c.SessionID, f.approver())
			} else {
				err = f.svc.Reject(ctx, c.SessionID, f.approver())
			}
			switch {
			case err == nil:
				wins.Add(1)
			case errors.Is(err, ErrResolved):
				conflicts.Add(1)
			default:
				t.Errorf("unexpected err %v", err)
			}
		}(i)
	}
	wg.Wait()
	if wins.Load() != 1 || conflicts.Load() != 15 {
		t.Fatalf("wins = %d conflicts = %d", wins.Load(), conflicts.Load())
	}
	if n := len(f.pub.events); n != 1 {
		t.Fatalf("published %d outcomes", n)
	}
}

func TestOtherUserCannotTakeOverClaimedSession(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	c := f.create(t)
	if _, err := f.svc.Get(ctx, c.SessionID, f.approver()); err != nil {
		t.Fatal(err)
	}
	intruder := Approver{UserUUID: uuid.New()}
	if _, err := f.svc.Get(ctx, c.SessionID, intruder); !errors.Is(err, ErrClaimed) {
		t.Fatalf("get = %v", err)
	}
	if err := f.svc.Approve(ctx, c.SessionID, intruder); !errors.Is(err, ErrClaimed) {
		t.Fatalf("approve = %v", err)
	}
	if err := f.svc.Reject(ctx, c.SessionID, intruder); !errors.Is(err, ErrClaimed) {
		t.Fatalf("reject = %v", err)
	}
	if got := f.pub.types(); got != "scanned" {
		t.Fatalf("events %v", got)
	}
}

func TestDisabledUserCannotApprove(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	c := f.create(t)
	f.acc.setDisabled(f.user)
	if err := f.svc.Approve(ctx, c.SessionID, f.approver()); !errors.Is(err, authusecase.ErrUserDisabled) {
		t.Fatalf("disabled = %v", err)
	}
	if len(f.pub.events) != 0 || len(f.acc.notices) != 0 {
		t.Fatal("nothing may be published or notified")
	}
	if st, _ := f.svc.State(ctx, c.SessionID, c.BrowserSecret); st.Status != StatusPending {
		t.Fatalf("state = %+v", st)
	}
}

func TestDisabledAfterApprovalCannotExchange(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	c := f.create(t)
	if err := f.svc.Approve(ctx, c.SessionID, f.approver()); err != nil {
		t.Fatal(err)
	}
	f.acc.setDisabled(f.user)
	if _, err := f.svc.Exchange(ctx, c.SessionID, c.BrowserSecret, f.pub.exchangeToken(t)); !errors.Is(err, authusecase.ErrUserDisabled) {
		t.Fatalf("exchange = %v", err)
	}
}

func TestImpersonationRefused(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	c := f.create(t)
	imp := f.approver()
	imp.Impersonating = true
	if _, err := f.svc.Get(ctx, c.SessionID, imp); !errors.Is(err, ErrImpersonating) {
		t.Fatalf("get = %v", err)
	}
	if err := f.svc.Approve(ctx, c.SessionID, imp); !errors.Is(err, ErrImpersonating) {
		t.Fatalf("approve = %v", err)
	}
	if err := f.svc.Reject(ctx, c.SessionID, imp); !errors.Is(err, ErrImpersonating) {
		t.Fatalf("reject = %v", err)
	}
	if len(f.pub.events) != 0 {
		t.Fatal("nothing may be published")
	}
	// The impersonation attempt did not claim the session: the real user still can.
	if err := f.svc.Approve(ctx, c.SessionID, f.approver()); err != nil {
		t.Fatalf("real user approve = %v", err)
	}
}

func TestPlatformUserNeedsStepUp(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	c := f.create(t)
	admin := f.approver()
	admin.PlatformUser = true
	// Viewing / claiming needs no step-up; approving does (KARAR-15).
	if _, err := f.svc.Get(ctx, c.SessionID, admin); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Approve(ctx, c.SessionID, admin); !errors.Is(err, ErrStepUpRequired) {
		t.Fatalf("approve without step-up = %v", err)
	}
	if got := f.pub.types(); got != "scanned" {
		t.Fatalf("events %v", got)
	}
	f.stepUp.granted[f.user] = true
	if err := f.svc.Approve(ctx, c.SessionID, admin); err != nil {
		t.Fatalf("approve with step-up = %v", err)
	}
	// A regular user never needs step-up.
	c2 := f.create(t)
	other := Approver{UserUUID: uuid.New()}
	if err := f.svc.Approve(ctx, c2.SessionID, other); err != nil {
		t.Fatalf("regular approve = %v", err)
	}
}

func TestPlatformUserWithoutStepUpEngineIsRefused(t *testing.T) {
	f := newFixture(t)
	f.svc.stepUp = nil
	c := f.create(t)
	if err := f.svc.Approve(context.Background(), c.SessionID, Approver{UserUUID: f.user, PlatformUser: true}); !errors.Is(err, ErrStepUpRequired) {
		t.Fatalf("err = %v", err)
	}
}

func TestExpiry(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	c := f.create(t)
	f.mr.FastForward(SessionTTL + time.Second)
	if _, err := f.svc.Get(ctx, c.SessionID, f.approver()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expired get = %v", err)
	}
	if err := f.svc.Approve(ctx, c.SessionID, f.approver()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expired approve = %v", err)
	}
	if err := f.svc.Reject(ctx, c.SessionID, f.approver()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expired reject = %v", err)
	}
	if _, err := f.svc.State(ctx, c.SessionID, c.BrowserSecret); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expired state = %v", err)
	}
}

func TestParseUserAgent(t *testing.T) {
	cases := []struct{ ua, browser, ver, os, device string }{
		{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36 Edg/128.0.2739.42", "Edge", "128", "Windows", "desktop"},
		{"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.4 Safari/605.1.15", "Safari", "17", "macOS", "desktop"},
		{"Mozilla/5.0 (X11; Linux x86_64; rv:129.0) Gecko/20100101 Firefox/129.0", "Firefox", "129", "Linux", "desktop"},
		{"Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/127.0.6533.107 Mobile/15E148 Safari/604.1", "Chrome", "127", "iOS", "mobile"},
		{"Mozilla/5.0 (Linux; Android 14; SM-S918B) AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/25.0 Chrome/121.0.0.0 Mobile Safari/537.36", "Samsung Internet", "25", "Android", "mobile"},
		{"curl/8.0", "", "", "", "desktop"},
	}
	for _, c := range cases {
		got := ParseUserAgent(c.ua)
		if got.Browser != c.browser || got.BrowserVersion != c.ver || got.OS != c.os || got.DeviceType != c.device {
			t.Errorf("%q → %+v", c.ua, got)
		}
	}
	if ParseUserAgent("curl/8.0").Label() != "Unknown browser" {
		t.Error("label fallback")
	}
}
