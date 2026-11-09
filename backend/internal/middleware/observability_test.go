package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/logging"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/reqid"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
)

func TestRequestIDReusesValidIncomingID(t *testing.T) {
	var seen string
	h := RequestID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = reqid.From(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/v1/x", nil)
	req.Header.Set(RequestIDHeader, "bff-abc-123")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if seen != "bff-abc-123" || rr.Header().Get(RequestIDHeader) != "bff-abc-123" {
		t.Fatalf("ctx=%q header=%q", seen, rr.Header().Get(RequestIDHeader))
	}
}

func TestRequestIDReplacesInvalidIncomingID(t *testing.T) {
	h := RequestID(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "/v1/x", nil)
	req.Header.Set(RequestIDHeader, "bad id\r\nX-Evil: 1")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	got := rr.Header().Get(RequestIDHeader)
	if got == "" || strings.Contains(got, " ") || !reqid.Valid(got) {
		t.Fatalf("generated id = %q", got)
	}
}

func TestRecoverWritesEnvelopeAndServerErrorsLogsRequestID(t *testing.T) {
	var buf bytes.Buffer
	log := logging.NewWithWriter(&buf, "debug", "json")

	h := Chain(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("kaboom")
	}), RequestID, ServerErrors(log), Recover)

	req := httptest.NewRequest(http.MethodGet, "/v1/boom", nil)
	req.Header.Set(RequestIDHeader, "rid-panic-1")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rr.Code)
	}
	var env response.Envelope
	if err := json.Unmarshal(rr.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Success || env.Error == nil || env.Error.Code != response.CodeInternalError {
		t.Fatalf("envelope = %+v", env)
	}
	if env.Meta.RequestID != "rid-panic-1" {
		t.Fatalf("meta.request_id = %q", env.Meta.RequestID)
	}
	out := buf.String()
	for _, want := range []string{`"msg":"http_handler_failed"`, `"request_id":"rid-panic-1"`, `panic: kaboom`, `"stack":`} {
		if !strings.Contains(out, want) {
			t.Errorf("log missing %s\n%s", want, out)
		}
	}
}

func TestRecoverRepanicsAbortHandler(t *testing.T) {
	h := Recover(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic(http.ErrAbortHandler)
	}))
	defer func() {
		if v := recover(); v != http.ErrAbortHandler { //nolint:errorlint // sentinel
			t.Fatalf("recovered %v", v)
		}
	}()
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
}
