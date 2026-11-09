package httpserver

// GET /v1/app/config publishes the mobile version settings, and the version gate reads the same
// row (TEC-1178). Postgres integration test: skipped unless TEST_DATABASE_URL is set.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	settingsusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/settings/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/appversion"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
)

func TestMain(m *testing.M) { pgtest.Main(m) }

func strPtr(s string) *string { return &s }

type appConfigBody struct {
	Success bool `json:"success"`
	Data    struct {
		Mobile struct {
			MinVersion    string `json:"min_version"`
			LatestVersion string `json:"latest_version"`
			StoreURLs     struct {
				IOS     string `json:"ios"`
				Android string `json:"android"`
			} `json:"store_urls"`
		} `json:"mobile"`
	} `json:"data"`
}

func getAppConfig(t *testing.T, s *Server, r *http.Request) appConfigBody {
	t.Helper()
	rec := httptest.NewRecorder()
	s.handleAppConfig(rec, r)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var body appConfigBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body
}

func TestIntegrationAppConfigMobileAndVersionGate(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	svc := settingsusecase.New(d.Queries)
	s := &Server{queries: d.Queries}

	// Defaults: nothing configured, so no gate.
	body := getAppConfig(t, s, httptest.NewRequest(http.MethodGet, "/v1/app/config", nil))
	if !body.Success || body.Data.Mobile.MinVersion != "" || body.Data.Mobile.StoreURLs.IOS != "" {
		t.Fatalf("defaults = %+v", body.Data.Mobile)
	}

	if _, err := svc.Patch(ctx, settingsusecase.PatchInput{
		MobileMinVersion:      strPtr(" 1.2.0 "),
		MobileLatestVersion:   strPtr("1.4.1"),
		MobileIOSStoreURL:     strPtr("https://apps.apple.com/app/id123"),
		MobileAndroidStoreURL: strPtr("https://play.google.com/store/apps/details?id=x"),
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		empty := strPtr("")
		_, _ = svc.Patch(ctx, settingsusecase.PatchInput{
			MobileMinVersion: empty, MobileLatestVersion: empty, MobileIOSStoreURL: empty, MobileAndroidStoreURL: empty,
		})
	})

	// An outdated app still reads the config (it needs the store URLs).
	req := httptest.NewRequest(http.MethodGet, "/v1/app/config", nil)
	req.Header.Set(appversion.HeaderClient, appversion.ClientMobile)
	req.Header.Set(appversion.HeaderVersion, "1.0.0")
	body = getAppConfig(t, s, req)
	m := body.Data.Mobile
	if m.MinVersion != "1.2.0" || m.LatestVersion != "1.4.1" ||
		m.StoreURLs.IOS != "https://apps.apple.com/app/id123" ||
		m.StoreURLs.Android != "https://play.google.com/store/apps/details?id=x" {
		t.Fatalf("mobile = %+v", m)
	}

	gate := appversion.Gate(svc.MinMobileVersion)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	for _, tc := range []struct {
		version string
		want    int
	}{
		{"1.1.9", http.StatusUpgradeRequired},
		{"1.2.0", http.StatusNoContent},
		{"2.0.0", http.StatusNoContent},
	} {
		r := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
		r.Header.Set(appversion.HeaderClient, appversion.ClientMobile)
		r.Header.Set(appversion.HeaderVersion, tc.version)
		rec := httptest.NewRecorder()
		gate.ServeHTTP(rec, r)
		if rec.Code != tc.want {
			t.Fatalf("version %s: status = %d, want %d", tc.version, rec.Code, tc.want)
		}
	}
	// The web (no X-Client) is never gated.
	rec := httptest.NewRecorder()
	gate.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("web status = %d", rec.Code)
	}
}
