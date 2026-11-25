package handler

// Export job routes reach only their own family's jobs (TEC-1302), against Postgres:
// /v1/platform/exports (get, download, list) only platform exports, /v1/exports (get, download)
// only the caller's app.* exports of the active account. Another family's or account's export
// is 404 and missing from the list.

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	exportusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/exports/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/ioengine"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/storage"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) { pgtest.Main(m) }

// memStore is the in-memory storage for the export files.
type memStore struct {
	storage.Driver
	m *storage.Memory
}

func (s memStore) Download(ctx context.Context, path string) (io.ReadCloser, int64, error) {
	return s.m.Download(ctx, path)
}

func TestIntegrationExportJobRoutesStayInTheirFamily(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	mem := storage.NewMemory()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := New(exportusecase.New(d.Queries, memStore{m: mem}, ioengine.NewRegistry(), nil, nil, nil, log))

	user := d.CreateUser(t)
	accountA, accountB := user.AccountID, d.CreateUser(t).AccountID // B: another account id

	job := func(resource string, account int64) string {
		t.Helper()
		query := "{}"
		if account > 0 {
			query = `{"account_id":"` + jsonInt(account) + `"}`
		}
		key := "exports/" + pgtest.Unique() + ".csv"
		require.NoError(t, mem.Upload(ctx, storage.File{Body: strings.NewReader("a,b\n"), Size: 4, ContentType: "text/csv"}, key))
		var id uuid.UUID
		require.NoError(t, d.Pool.QueryRow(ctx, `
			INSERT INTO export_jobs (resource, actor_id, format, query_json, status, file_key)
			VALUES ($1, $2, 'csv', $3::jsonb, 'completed', $4) RETURNING uuid`,
			resource, user.ID, query, key).Scan(&id))
		return id.String()
	}
	platformJob := job("platform.users", 0)
	leadsA := job("app.leads", accountA)
	leadsB := job("app.leads", accountB)

	appP := authctx.Principal{UserID: user.UUID, UserInternal: user.ID,
		Account: authctx.Account{ID: accountA, Type: authctx.AccountTypePersonal, Role: authctx.AccountRoleOwner}}
	platformP := appP
	platformP.IsSuperAdmin = true

	call := func(fn http.HandlerFunc, p authctx.Principal, path, id string) (int, []byte) {
		t.Helper()
		r := httptest.NewRequest("GET", path, nil)
		if id != "" {
			r.SetPathValue("uuid", id)
		}
		r = r.WithContext(authctx.WithPrincipal(r.Context(), p))
		w := httptest.NewRecorder()
		fn(w, r)
		return w.Code, w.Body.Bytes()
	}
	expect := func(want int, fn http.HandlerFunc, p authctx.Principal, path, id string) {
		t.Helper()
		code, body := call(fn, p, path, id)
		require.Equal(t, want, code, "%s: %s", path, body)
	}

	// --- App routes (/v1/exports): the caller's app export of the active account only.
	expect(http.StatusOK, h.GetOwn, appP, "/v1/exports/"+leadsA, leadsA)
	code, body := call(h.DownloadOwn, appP, "/v1/exports/"+leadsA+"/download", leadsA)
	require.Equal(t, http.StatusOK, code)
	require.True(t, bytes.HasPrefix(body, []byte("a,b")))
	for _, foreign := range []string{platformJob, leadsB} {
		expect(http.StatusNotFound, h.GetOwn, appP, "/v1/exports/"+foreign, foreign)
		expect(http.StatusNotFound, h.DownloadOwn, appP, "/v1/exports/"+foreign+"/download", foreign)
	}
	// In account B the same person reaches B's export and not A's.
	appB := appP
	appB.Account.ID = accountB
	expect(http.StatusOK, h.GetOwn, appB, "/v1/exports/"+leadsB, leadsB)
	expect(http.StatusNotFound, h.GetOwn, appB, "/v1/exports/"+leadsA, leadsA)
	// Without an active account nothing is reached.
	noAccount := appP
	noAccount.Account = authctx.Account{}
	expect(http.StatusNotFound, h.GetOwn, noAccount, "/v1/exports/"+leadsA, leadsA)
	// Another user of account A does not reach the caller's export.
	stranger := d.CreateUser(t)
	strangerP := appP
	strangerP.UserID, strangerP.UserInternal = stranger.UUID, stranger.ID
	expect(http.StatusNotFound, h.GetOwn, strangerP, "/v1/exports/"+leadsA, leadsA)

	// --- Platform routes: platform exports only, even for a super admin who made the app ones.
	expect(http.StatusOK, h.Get, platformP, "/v1/platform/exports/"+platformJob, platformJob)
	expect(http.StatusOK, h.Download, platformP, "/v1/platform/exports/"+platformJob+"/download", platformJob)
	for _, app := range []string{leadsA, leadsB} {
		expect(http.StatusNotFound, h.Get, platformP, "/v1/platform/exports/"+app, app)
		expect(http.StatusNotFound, h.Download, platformP, "/v1/platform/exports/"+app+"/download", app)
	}
	code, body = call(h.List, platformP, "/v1/platform/exports", "")
	require.Equal(t, http.StatusOK, code)
	var env struct {
		Data struct {
			Items []struct {
				UUID string `json:"uuid"`
			} `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &env))
	var ids []string
	for _, it := range env.Data.Items {
		ids = append(ids, it.UUID)
	}
	require.Contains(t, ids, platformJob)
	require.NotContains(t, ids, leadsA)
	require.NotContains(t, ids, leadsB)
	// A non-admin's own list holds only their platform exports too.
	code, body = call(h.List, appP, "/v1/platform/exports", "")
	require.Equal(t, http.StatusOK, code)
	require.NoError(t, json.Unmarshal(body, &env))
	require.Len(t, env.Data.Items, 1)
	require.Equal(t, platformJob, env.Data.Items[0].UUID)
}

func jsonInt(v int64) string {
	b, _ := json.Marshal(v)
	return string(b)
}
