package handler

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	oauthproviderusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/integrations/oauthprovider/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/crypto"
)

type oneRowStore struct{ row db.OauthProviderSetting }

func (s *oneRowStore) GetOAuthProviderSettings(context.Context, string) (db.OauthProviderSetting, error) {
	return s.row, nil
}

func (s *oneRowStore) ListOAuthProviderSettings(context.Context) ([]db.OauthProviderSetting, error) {
	return []db.OauthProviderSetting{s.row}, nil
}

func (s *oneRowStore) UpdateOAuthProviderSettings(_ context.Context, a db.UpdateOAuthProviderSettingsParams) (db.OauthProviderSetting, error) {
	if a.ClientID.Valid {
		s.row.ClientID = a.ClientID.String
	}
	if a.AppleTeamID.Valid {
		s.row.AppleTeamID = a.AppleTeamID.String
	}
	if a.AppleKeyID.Valid {
		s.row.AppleKeyID = a.AppleKeyID.String
	}
	if a.ApplePrivateKeyEnc.Valid {
		s.row.ApplePrivateKeyEnc = a.ApplePrivateKeyEnc
	}
	return s.row, nil
}

func newHandler(t *testing.T) *Handler {
	t.Helper()
	raw := make([]byte, 32)
	for i := range raw {
		raw[i] = byte(i + 3)
	}
	box, err := crypto.NewSecretBox(base64.StdEncoding.EncodeToString(raw))
	if err != nil {
		t.Fatal(err)
	}
	return New(oauthproviderusecase.New(&oneRowStore{row: db.OauthProviderSetting{Provider: "apple"}}, box), nil)
}

func p8(t *testing.T) (pemText, body string) {
	t.Helper()
	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	der, _ := x509.MarshalPKCS8PrivateKey(k)
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})), base64.StdEncoding.EncodeToString(der)
}

func patch(h *Handler, body map[string]any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(http.MethodPatch, "/v1/platform/integrations/apple", bytes.NewReader(b))
	r.SetPathValue("provider", "apple")
	w := httptest.NewRecorder()
	h.PatchSettings(w, r)
	return w
}

// The .p8 key is write-only: neither the PATCH/GET response nor a validation
// error ever contains it (TEC-1123).
func TestApplePrivateKeyIsWriteOnly(t *testing.T) {
	h := newHandler(t)
	pemText, body := p8(t)

	w := patch(h, map[string]any{"team_id": "TEAM000001", "key_id": "KEY0000001", "private_key": pemText})
	if w.Code != http.StatusOK {
		t.Fatalf("patch status %d: %s", w.Code, w.Body)
	}
	assertClean(t, "patch", w.Body.String(), body)
	if !strings.Contains(w.Body.String(), `"private_key_configured":true`) ||
		!strings.Contains(w.Body.String(), `"private_key_fingerprint":"SHA256:`) {
		t.Fatalf("missing configured/fingerprint: %s", w.Body)
	}

	r := httptest.NewRequest(http.MethodGet, "/v1/platform/integrations/apple", nil)
	r.SetPathValue("provider", "apple")
	g := httptest.NewRecorder()
	h.GetSettings(g, r)
	assertClean(t, "get", g.Body.String(), body)

	bad := pemText[:len(pemText)/2] // truncated key: rejected without echo
	e := patch(h, map[string]any{"team_id": "TEAM000001", "key_id": "KEY0000001", "private_key": bad})
	if e.Code != http.StatusBadRequest {
		t.Fatalf("invalid key status %d", e.Code)
	}
	assertClean(t, "error", e.Body.String(), body)
}

func assertClean(t *testing.T, what, out, body string) {
	t.Helper()
	for _, leak := range []string{"PRIVATE KEY", body[:20], body[len(body)-20:], "apple_private_key_enc", `"private_key"`} {
		if strings.Contains(out, leak) {
			t.Fatalf("%s response leaks %q: %s", what, leak, out)
		}
	}
}
