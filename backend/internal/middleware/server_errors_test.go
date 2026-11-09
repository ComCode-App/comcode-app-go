package middleware

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/logging"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
)

// observed runs one request through the production chain order (RequestID → ServerErrors →
// Recover → CaptureRoute(mux)) and returns the log output.
func observed(t *testing.T, mux *http.ServeMux, req *http.Request) (string, *httptest.ResponseRecorder) {
	t.Helper()
	var buf bytes.Buffer
	log := logging.NewWithWriter(&buf, "debug", "json")
	h := Chain(CaptureRoute(mux), RequestID, ServerErrors(log), Recover)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return buf.String(), rr
}

func TestServerErrorsLogsAuthFailureWithoutPersonalData(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		response.RecordFailure(w, errors.New("invalid credentials for ayse@example.com password=hunter2"))
		response.Error(w, r, http.StatusUnauthorized, response.CodeInvalidCredentials, "Email or password is incorrect")
	})
	out, rr := observed(t, mux, httptest.NewRequest(http.MethodPost, "/v1/auth/login", strings.NewReader(`{"email":"ayse@example.com"}`)))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rr.Code)
	}
	for _, want := range []string{`"msg":"http_request"`, `"route":"POST /v1/auth/login"`, `"status":401`, `"error_code":"INVALID_CREDENTIALS"`, `"reason":"invalid credentials for [email]`} {
		if !strings.Contains(out, want) {
			t.Errorf("log missing %s\n%s", want, out)
		}
	}
	for _, bad := range []string{"ayse", "example.com", "hunter2"} {
		if strings.Contains(out, bad) {
			t.Errorf("log leaks %q\n%s", bad, out)
		}
	}
}

func TestServerErrorsLogsRoutePatternNotRawPath(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/invites/{token}", func(w http.ResponseWriter, r *http.Request) {
		response.NotFound(w, r, "")
	})
	out, _ := observed(t, mux, httptest.NewRequest(http.MethodGet, "/v1/invites/secret-invite-token?x=1", nil))
	if !strings.Contains(out, `"route":"GET /v1/invites/{token}"`) || !strings.Contains(out, `"error_code":"NOT_FOUND"`) {
		t.Fatalf("missing route/error_code\n%s", out)
	}
	if strings.Contains(out, "secret-invite-token") {
		t.Fatalf("raw path leaked\n%s", out)
	}
}

func TestServerErrorsSkipsSuccessfulNonAuthRequests(t *testing.T) {
	mux := http.NewServeMux()
	ok := func(w http.ResponseWriter, r *http.Request) { response.JSON(w, r, http.StatusOK, map[string]string{}) }
	mux.HandleFunc("GET /v1/cards", ok)
	mux.HandleFunc("POST /v1/auth/refresh", ok)

	if out, _ := observed(t, mux, httptest.NewRequest(http.MethodGet, "/v1/cards", nil)); out != "" {
		t.Fatalf("2xx non-auth request was logged\n%s", out)
	}
	out, _ := observed(t, mux, httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", nil))
	if !strings.Contains(out, `"msg":"http_request"`) || !strings.Contains(out, `"status":200`) {
		t.Fatalf("auth request not logged\n%s", out)
	}
}

func TestServerErrorsClientCancelIs499NotServerError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/slow", func(w http.ResponseWriter, r *http.Request) {
		response.InternalErr(w, r, r.Context().Err(), "Unexpected server error")
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out, _ := observed(t, mux, httptest.NewRequest(http.MethodGet, "/v1/slow", nil).WithContext(ctx))
	if !strings.Contains(out, `"msg":"http_request_canceled"`) || !strings.Contains(out, `"status":499`) || !strings.Contains(out, `"level":"INFO"`) {
		t.Fatalf("cancel not logged as 499 INFO\n%s", out)
	}
	if strings.Contains(out, "http_handler_failed") || strings.Contains(out, `"level":"ERROR"`) {
		t.Fatalf("client cancel logged as server error\n%s", out)
	}
}

func TestServerErrorsRedactsHandlerErrors(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/boom", func(w http.ResponseWriter, r *http.Request) {
		response.InternalErr(w, r, errors.New("smtp send to mehmet@example.org failed"), "Unexpected server error")
	})
	out, _ := observed(t, mux, httptest.NewRequest(http.MethodGet, "/v1/boom", nil))
	if !strings.Contains(out, `"msg":"http_handler_failed"`) || !strings.Contains(out, `"route":"GET /v1/boom"`) {
		t.Fatalf("5xx not logged\n%s", out)
	}
	if strings.Contains(out, "mehmet@example.org") {
		t.Fatalf("email leaked\n%s", out)
	}
}
