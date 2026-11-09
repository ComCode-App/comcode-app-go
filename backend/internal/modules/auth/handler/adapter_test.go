package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireAdapterKey(t *testing.T) {
	const secret = "adapter-secret-0123456789abcdef0123456789"
	cases := []struct {
		name   string
		secret string
		header string
		ok     bool
		status int
	}{
		{"match", secret, secret, true, http.StatusOK},
		{"match with whitespace", secret, "  " + secret + " ", true, http.StatusOK},
		{"missing header", secret, "", false, http.StatusUnauthorized},
		{"wrong key", secret, "adapter-secret-0123456789abcdef012345678X", false, http.StatusUnauthorized},
		{"prefix of key", secret, secret[:10], false, http.StatusUnauthorized},
		{"key with suffix", secret, secret + "x", false, http.StatusUnauthorized},
		{"not configured", "", "anything", false, http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := &Handler{adapterSecret: tc.secret}
			r := httptest.NewRequest(http.MethodGet, "/v1/auth/adapter/user", nil)
			if tc.header != "" {
				r.Header.Set("X-Auth-Adapter-Key", tc.header)
			}
			w := httptest.NewRecorder()
			if got := h.requireAdapterKey(w, r); got != tc.ok {
				t.Fatalf("requireAdapterKey = %v, want %v", got, tc.ok)
			}
			if w.Code != tc.status {
				t.Fatalf("status = %d, want %d", w.Code, tc.status)
			}
		})
	}
}
