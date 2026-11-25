package handler_test

// Import job routes reach only their own family's jobs (TEC-1302), against Postgres:
// /v1/platform/imports only platform jobs (no account), /v1/company/imports only the active
// company's app.company_employees jobs. Another family's, resource's or company's job is 404 on
// every job route (get, mapping, preview, confirm, rollback) and missing from the list.

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	cpmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyprofiles/model"
	importhandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/imports/handler"
	importusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/imports/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/ioengine"
	ioadapters "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/ioengine/adapters"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) { pgtest.Main(m) }

func serve(t *testing.T, h http.Handler, p authctx.Principal, method, path, id string, body any) (int, json.RawMessage) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	r := httptest.NewRequest(method, path, &buf)
	if id != "" {
		r.SetPathValue("uuid", id)
	}
	r = r.WithContext(authctx.WithPrincipal(r.Context(), p))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var env struct {
		Data json.RawMessage `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	return w.Code, env.Data
}

func TestIntegrationImportJobRoutesStayInTheirFamily(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	reg := ioengine.NewRegistry(ioadapters.NewUsers(d.Queries))
	h := importhandler.New(importusecase.New(d.Queries, nil, reg, nil, nil, nil, log))

	newCompany := func(slug string) db.Account {
		c, err := d.Queries.CreateCompanyAccount(ctx, db.CreateCompanyAccountParams{Name: slug, Slug: slug + "-" + pgtest.Unique()})
		require.NoError(t, err)
		return c
	}
	company, other := newCompany("ice"), newCompany("dis")
	admin := d.CreateUser(t)
	for _, c := range []db.Account{company, other} {
		require.NoError(t, d.Queries.AddAccountMember(ctx, db.AddAccountMemberParams{AccountID: c.ID, UserID: admin.ID, Role: db.AccountRoleOwner}))
	}
	companyP := authctx.Principal{
		UserID: admin.UUID, UserInternal: admin.ID,
		Permissions: []string{rbac.PermAppCompanyProfilesImport},
		Account:     authctx.Account{ID: company.ID, Type: authctx.AccountTypeCompany, Role: authctx.AccountRoleOwner},
	}
	// The same person as a platform super admin: platform routes only.
	platformP := companyP
	platformP.IsSuperAdmin = true

	job := func(resource string, account *int64) string {
		var id uuid.UUID
		require.NoError(t, d.Pool.QueryRow(ctx, `
			INSERT INTO import_jobs (resource, actor_id, format, status, file_key, account_id, rollback_until, applied_at)
			VALUES ($1, $2, 'csv', 'applied', 'x', $3, now() + interval '1 hour', now()) RETURNING uuid`,
			resource, admin.ID, account).Scan(&id))
		return id.String()
	}
	platformJob := job(ioadapters.ResourceUsers, nil)
	employeeJob := job(cpmodel.ResourceCompanyEmployees, &company.ID)
	otherResourceJob := job("app.other_resource", &company.ID) // same account, another resource
	otherCompanyJob := job(cpmodel.ResourceCompanyEmployees, &other.ID)

	inCompany := func(fn http.HandlerFunc) http.Handler { return importhandler.RequireCompanyImporter(fn) }
	jobRoutes := func(wrap func(http.HandlerFunc) http.Handler) []struct {
		name, method, suffix string
		h                    http.Handler
		body                 any
	} {
		return []struct {
			name, method, suffix string
			h                    http.Handler
			body                 any
		}{
			{"mapping", "PATCH", "/mapping", wrap(h.UpdateMapping), map[string]any{"mapping": map[string]string{}}},
			{"preview", "POST", "/preview", wrap(h.Preview), nil},
			{"confirm", "POST", "/confirm", wrap(h.Confirm), nil},
			{"rollback", "POST", "/rollback", wrap(h.Rollback), nil},
		}
	}
	listed := func(hh http.Handler, p authctx.Principal, path string) []string {
		t.Helper()
		code, data := serve(t, hh, p, "GET", path, "", nil)
		require.Equal(t, http.StatusOK, code, string(data))
		var page struct {
			Items []struct {
				UUID string `json:"uuid"`
			} `json:"items"`
		}
		require.NoError(t, json.Unmarshal(data, &page))
		out := []string{}
		for _, it := range page.Items {
			out = append(out, it.UUID)
		}
		return out
	}
	status := func(id string) string {
		var s string
		require.NoError(t, d.Pool.QueryRow(ctx, `SELECT status FROM import_jobs WHERE uuid = $1`, id).Scan(&s))
		return s
	}

	// --- Company routes: only this company's employee import jobs.
	require.Equal(t, []string{employeeJob}, listed(inCompany(h.CompanyList), companyP, "/v1/company/imports"))
	for _, foreign := range []string{platformJob, otherResourceJob, otherCompanyJob} {
		code, _ := serve(t, inCompany(h.CompanyGet), companyP, "GET", "/v1/company/imports/"+foreign, foreign, nil)
		require.Equal(t, http.StatusNotFound, code, "company get %s", foreign)
		for _, rt := range jobRoutes(inCompany) {
			code, data := serve(t, rt.h, companyP, rt.method, "/v1/company/imports/"+foreign+rt.suffix, foreign, rt.body)
			require.Equal(t, http.StatusNotFound, code, "company %s %s: %s", rt.name, foreign, data)
		}
	}
	code, _ := serve(t, inCompany(h.CompanyGet), companyP, "GET", "/v1/company/imports/"+employeeJob, employeeJob, nil)
	require.Equal(t, http.StatusOK, code, "own employee import job")

	// The other company, as its active account, sees its own job and none of these.
	otherP := companyP
	otherP.Account.ID = other.ID
	require.Equal(t, []string{otherCompanyJob}, listed(inCompany(h.CompanyList), otherP, "/v1/company/imports"))
	code, _ = serve(t, inCompany(h.CompanyGet), otherP, "GET", "/v1/company/imports/"+employeeJob, employeeJob, nil)
	require.Equal(t, http.StatusNotFound, code)

	// --- Platform routes: only platform jobs, even for a super admin who started the app jobs.
	platformList := listed(http.HandlerFunc(h.List), platformP, "/v1/platform/imports")
	require.Contains(t, platformList, platformJob)
	for _, app := range []string{employeeJob, otherResourceJob, otherCompanyJob} {
		require.NotContains(t, platformList, app)
		code, _ := serve(t, http.HandlerFunc(h.Get), platformP, "GET", "/v1/platform/imports/"+app, app, nil)
		require.Equal(t, http.StatusNotFound, code, "platform get %s", app)
		for _, rt := range jobRoutes(func(fn http.HandlerFunc) http.Handler { return fn }) {
			code, data := serve(t, rt.h, platformP, rt.method, "/v1/platform/imports/"+app+rt.suffix, app, rt.body)
			require.Equal(t, http.StatusNotFound, code, "platform %s %s: %s", rt.name, app, data)
		}
	}
	code, _ = serve(t, http.HandlerFunc(h.Get), platformP, "GET", "/v1/platform/imports/"+platformJob, platformJob, nil)
	require.Equal(t, http.StatusOK, code, "own platform import job")

	// Nothing changed through a foreign route; the platform job still rolls back on its own route.
	for _, app := range []string{employeeJob, otherResourceJob, otherCompanyJob} {
		require.Equal(t, "applied", status(app))
	}
	code, data := serve(t, http.HandlerFunc(h.Rollback), platformP, "POST", "/v1/platform/imports/"+platformJob+"/rollback", platformJob, nil)
	require.Equal(t, http.StatusOK, code, string(data))
	require.Equal(t, "rolled_back", status(platformJob))
}
