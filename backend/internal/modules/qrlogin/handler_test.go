package qrlogin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/clientip"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/ratelimit"
	"github.com/google/uuid"
)

// fakeLoader turns any valid access token into a principal for the token's user.
type fakeLoader struct {
	permissions   []string
	impersonating bool
}

func (l fakeLoader) LoadPrincipal(_ *http.Request, claims jwt.Claims) (authctx.Principal, error) {
	id, err := claims.UserUUID()
	if err != nil {
		return authctx.Principal{}, err
	}
	p := authctx.Principal{UserID: id, Permissions: l.permissions, SessionID: uuid.New()}
	if l.impersonating {
		imp := uuid.New()
		p.ImpersonatorUserID = &imp
	}
	return p, nil
}

type handlerEnv struct {
	f      *fixture
	root   http.Handler
	bearer string
}

func newHandlerEnv(t *testing.T, loader fakeLoader, withLimiter bool) *handlerEnv {
	t.Helper()
	f := newFixture(t)
	tokens, err := jwt.NewManager("unit-test-access-secret-0123456789abcdef", time.Hour, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	var lim Limiter
	if withLimiter {
		lim = ratelimit.New(f.rdb, "test")
	}
	mux := http.NewServeMux()
	RegisterRoutes(mux, NewHandler(f.svc, lim), tokens, loader)
	// As in production: requests arrive from the BFF (a trusted proxy) through clientip.
	trusted, err := clientip.ParseTrusted("10.0.0.0/8")
	if err != nil {
		t.Fatal(err)
	}
	access, _, err := tokens.IssueAccess(jwt.AccessInput{UserID: f.user, SessionID: uuid.New(), AccountID: uuid.New()})
	if err != nil {
		t.Fatal(err)
	}
	return &handlerEnv{f: f, root: clientip.New(trusted).Middleware(mux), bearer: access}
}

func (e *handlerEnv) do(method, path, body string, auth bool) *httptest.ResponseRecorder {
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	req.RemoteAddr = "10.0.0.2:41000"
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0) Firefox/130.0")
	if auth {
		req.Header.Set("Authorization", "Bearer "+e.bearer)
	}
	rec := httptest.NewRecorder()
	e.root.ServeHTTP(rec, req)
	return rec
}

func (e *handlerEnv) create(t *testing.T) Created {
	t.Helper()
	rec := e.do("POST", "/v1/auth/qr/sessions", "", false)
	if rec.Code != http.StatusCreated || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("create %d %s", rec.Code, rec.Body)
	}
	var out struct {
		Data Created `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out.Data
}

func TestHandlerFlow(t *testing.T) {
	e := newHandlerEnv(t, fakeLoader{}, true)
	c := e.create(t)
	sess, err := e.f.svc.store.Get(t.Context(), c.SessionID)
	if err != nil || sess.IP != "203.0.113.9" || !strings.Contains(sess.UserAgent, "Firefox") || sess.Location.City != "Istanbul" {
		t.Fatalf("stored %+v %v", sess, err)
	}

	secretBody := `{"browser_secret":"` + c.BrowserSecret + `"}`
	rec := e.do("POST", "/v1/auth/qr/sessions/"+c.SessionID+"/state", secretBody, false)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"pending"`) {
		t.Fatalf("state %d %s", rec.Code, rec.Body)
	}

	rec = e.do("GET", "/v1/auth/qr/sessions/"+c.SessionID, "", true)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"browser":"Firefox"`) {
		t.Fatalf("get %d %s", rec.Code, rec.Body)
	}
	rec = e.do("POST", "/v1/auth/qr/sessions/"+c.SessionID+"/approve", "", true)
	if rec.Code != http.StatusOK {
		t.Fatalf("approve %d %s", rec.Code, rec.Body)
	}
	rec = e.do("POST", "/v1/auth/qr/sessions/"+c.SessionID+"/reject", "", true)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "QR_SESSION_RESOLVED") {
		t.Fatalf("reject after approve %d %s", rec.Code, rec.Body)
	}

	body := `{"session_id":"` + c.SessionID + `","browser_secret":"` + c.BrowserSecret + `","exchange_token":"` + e.f.pub.exchangeToken(t) + `"}`
	rec = e.do("POST", "/v1/auth/qr/exchange", body, false)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"access_token":"at"`) ||
		!strings.Contains(rec.Body.String(), e.f.user.String()) {
		t.Fatalf("exchange %d %s", rec.Code, rec.Body)
	}
	rec = e.do("POST", "/v1/auth/qr/exchange", body, false)
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "QR_SESSION_NOT_FOUND") {
		t.Fatalf("replay %d %s", rec.Code, rec.Body)
	}

	rec = e.do("POST", "/v1/auth/qr/exchange", `{"session_id":"x","browser_secret":"y","exchange_token":"z"}`, false)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "QR_LOGIN_INVALID") {
		t.Fatalf("malformed exchange %d %s", rec.Code, rec.Body)
	}
	rec = e.do("POST", "/v1/auth/qr/exchange", `{"unknown":1}`, false)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "VALIDATION_ERROR") {
		t.Fatalf("unknown field %d %s", rec.Code, rec.Body)
	}
}

func TestHandlerPhoneRoutesNeedBearer(t *testing.T) {
	e := newHandlerEnv(t, fakeLoader{}, false)
	c := e.create(t)
	for _, r := range []struct{ method, path string }{
		{"GET", "/v1/auth/qr/sessions/" + c.SessionID},
		{"POST", "/v1/auth/qr/sessions/" + c.SessionID + "/approve"},
		{"POST", "/v1/auth/qr/sessions/" + c.SessionID + "/reject"},
	} {
		if rec := e.do(r.method, r.path, "", false); rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s without bearer = %d", r.method, r.path, rec.Code)
		}
	}
	if got := e.f.pub.types(); got != "" {
		t.Fatalf("events %v", got)
	}
}

func TestHandlerImpersonationAndStepUp(t *testing.T) {
	imp := newHandlerEnv(t, fakeLoader{impersonating: true}, false)
	c := imp.create(t)
	rec := imp.do("POST", "/v1/auth/qr/sessions/"+c.SessionID+"/approve", "", true)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "FORBIDDEN") {
		t.Fatalf("impersonating approve %d %s", rec.Code, rec.Body)
	}

	admin := newHandlerEnv(t, fakeLoader{permissions: []string{"platform.users.read"}}, false)
	c = admin.create(t)
	rec = admin.do("POST", "/v1/auth/qr/sessions/"+c.SessionID+"/approve", "", true)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "STEP_UP_REQUIRED") {
		t.Fatalf("platform approve without step-up %d %s", rec.Code, rec.Body)
	}
	admin.f.stepUp.granted[admin.f.user] = true
	if rec = admin.do("POST", "/v1/auth/qr/sessions/"+c.SessionID+"/approve", "", true); rec.Code != http.StatusOK {
		t.Fatalf("platform approve with step-up %d %s", rec.Code, rec.Body)
	}

	// app.* permissions are not platform access.
	user := newHandlerEnv(t, fakeLoader{permissions: []string{"app.billing.write"}}, false)
	c = user.create(t)
	if rec = user.do("POST", "/v1/auth/qr/sessions/"+c.SessionID+"/approve", "", true); rec.Code != http.StatusOK {
		t.Fatalf("regular approve %d %s", rec.Code, rec.Body)
	}
}

func TestHandlerRateLimits(t *testing.T) {
	e := newHandlerEnv(t, fakeLoader{}, true)
	for i := 0; i < createIPLimit; i++ {
		if rec := e.do("POST", "/v1/auth/qr/sessions", "", false); rec.Code != http.StatusCreated {
			t.Fatalf("create %d = %d", i, rec.Code)
		}
	}
	rec := e.do("POST", "/v1/auth/qr/sessions", "", false)
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" || !strings.Contains(rec.Body.String(), "RATE_LIMITED") {
		t.Fatalf("create over limit %d %s", rec.Code, rec.Body)
	}

	missing := "/v1/auth/qr/sessions/" + strings.Repeat("a", 43)
	for i := 0; i < viewUserLimit; i++ {
		if rec := e.do("GET", missing, "", true); rec.Code != http.StatusNotFound {
			t.Fatalf("view %d = %d", i, rec.Code)
		}
	}
	if rec := e.do("GET", missing, "", true); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("view over limit = %d", rec.Code)
	}
	for i := 0; i < resolveUserLimit; i++ {
		if rec := e.do("POST", missing+"/approve", "", true); rec.Code != http.StatusNotFound {
			t.Fatalf("approve %d = %d", i, rec.Code)
		}
	}
	if rec := e.do("POST", missing+"/reject", "", true); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("resolve over limit = %d", rec.Code)
	}

	// Exchange is limited per session id as well as per IP.
	body := `{"session_id":"` + strings.Repeat("b", 43) + `","browser_secret":"x","exchange_token":"y"}`
	for i := 0; i < exchangeSessionLimit; i++ {
		if rec := e.do("POST", "/v1/auth/qr/exchange", body, false); rec.Code != http.StatusBadRequest {
			t.Fatalf("exchange %d = %d", i, rec.Code)
		}
	}
	if rec := e.do("POST", "/v1/auth/qr/exchange", body, false); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("exchange over limit = %d", rec.Code)
	}
}
