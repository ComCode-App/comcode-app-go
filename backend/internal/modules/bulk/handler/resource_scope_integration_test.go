package handler_test

// Bulk job routes reach only their own family's jobs (TEC-1300), against Postgres:
// /v1/company/bulk only app.company_profiles jobs of the account, /v1/cards/bulk only app.cards
// jobs of the account, /v1/platform/bulk only platform jobs (no account). Another family's job
// is 404 and missing from the list; each family's own jobs still work.

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/config"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/bulk/handler"
	bulkusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/bulk/usecase"
	cpmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyprofiles/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/bulkengine"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/bulkengine/adapters"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestIntegrationBulkJobRoutesStayInTheirResource(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	reg := bulkengine.NewRegistry(adapters.NewUsers(d.Queries), adapters.NewCards(d.Queries, nil))
	h := handler.New(bulkusecase.New(d.Queries, reg, nil, nil, nil, config.BulkConfig{SyncMax: 50, RollbackHours: 24}, log))

	company, err := d.Queries.CreateCompanyAccount(ctx, db.CreateCompanyAccountParams{Name: "Kapsam Co", Slug: "kapsam-" + pgtest.Unique()})
	require.NoError(t, err)
	admin := d.CreateUser(t)
	require.NoError(t, d.Queries.AddAccountMember(ctx, db.AddAccountMemberParams{AccountID: company.ID, UserID: admin.ID, Role: db.AccountRoleAdmin}))
	adminP := authctx.Principal{
		UserID: admin.UUID, UserInternal: admin.ID,
		Permissions: []string{rbac.PermAppCardsWrite, rbac.PermAppCardsBulkLock, rbac.PermAppCardsBulkUnlock, rbac.PermAppCompanyProfilesBulk},
		Account:     authctx.Account{ID: company.ID, Type: authctx.AccountTypeCompany, Role: authctx.AccountRoleAdmin},
	}
	// The same person as a platform super admin (no active account): platform routes only.
	platformP := adminP
	platformP.IsSuperAdmin = true

	job := func(resource, action string, account *int64) string {
		var id uuid.UUID
		require.NoError(t, d.Pool.QueryRow(ctx, `
			INSERT INTO bulk_jobs (resource, action, actor_id, status, account_id, rollback_until, applied_at)
			VALUES ($1, $2, $3, 'completed', $4, now() + interval '1 hour', now()) RETURNING uuid`,
			resource, action, admin.ID, account).Scan(&id))
		return id.String()
	}
	cardJob := job(adapters.ResourceAppCards, adapters.ActionCardsLock, &company.ID)
	companyJob := job(cpmodel.ResourceCompanyProfiles, "set_theme", &company.ID)
	platformJob := job("platform.users", "disable", nil)

	status := func(id string) string {
		var s string
		require.NoError(t, d.Pool.QueryRow(ctx, `SELECT status FROM bulk_jobs WHERE uuid = $1`, id).Scan(&s))
		return s
	}
	listed := func(h http.HandlerFunc, p authctx.Principal, path string) []string {
		t.Helper()
		code, data := call(t, h, p, "GET", path, "", nil)
		require.Equal(t, http.StatusOK, code, string(data))
		var page struct {
			Items []struct {
				UUID string `json:"uuid"`
			} `json:"items"`
			Total int `json:"total"`
		}
		require.NoError(t, json.Unmarshal(data, &page))
		out := make([]string, 0, len(page.Items))
		for _, it := range page.Items {
			out = append(out, it.UUID)
		}
		require.Len(t, out, page.Total)
		return out
	}
	notFound := func(h http.HandlerFunc, p authctx.Principal, method, path, id string) {
		t.Helper()
		code, data := call(t, h, p, method, path, id, nil)
		require.Equal(t, http.StatusNotFound, code, "%s %s: %s", method, path, data)
	}

	// --- Company routes: the card job and the platform job do not exist there.
	require.Equal(t, []string{companyJob}, listed(h.CompanyList, adminP, "/v1/company/bulk"))
	notFound(h.CompanyGet, adminP, "GET", "/v1/company/bulk/"+cardJob, cardJob)
	notFound(h.CompanyRollback, adminP, "POST", "/v1/company/bulk/"+cardJob+"/rollback", cardJob)
	notFound(h.CompanyGet, adminP, "GET", "/v1/company/bulk/"+platformJob, platformJob)
	code, _ := call(t, h.CompanyGet, adminP, "GET", "/v1/company/bulk/"+companyJob, companyJob, nil)
	require.Equal(t, http.StatusOK, code, "own company job")

	// --- Card routes: the company job and the platform job do not exist there.
	require.Equal(t, []string{cardJob}, listed(h.CardsList, adminP, "/v1/cards/bulk"))
	notFound(h.CardsGet, adminP, "GET", "/v1/cards/bulk/"+companyJob, companyJob)
	notFound(h.CardsRollback, adminP, "POST", "/v1/cards/bulk/"+companyJob+"/rollback", companyJob)
	notFound(h.CardsGet, adminP, "GET", "/v1/cards/bulk/"+platformJob, platformJob)
	code, _ = call(t, h.CardsGet, adminP, "GET", "/v1/cards/bulk/"+cardJob, cardJob, nil)
	require.Equal(t, http.StatusOK, code, "own card job")

	// --- Platform routes: account jobs do not exist there, even for a super admin who started them.
	platformList := listed(h.List, platformP, "/v1/platform/bulk")
	require.Contains(t, platformList, platformJob)
	require.NotContains(t, platformList, cardJob)
	require.NotContains(t, platformList, companyJob)
	notFound(h.Get, platformP, "GET", "/v1/platform/bulk/"+cardJob, cardJob)
	notFound(h.Rollback, platformP, "POST", "/v1/platform/bulk/"+cardJob+"/rollback", cardJob)
	notFound(h.Get, platformP, "GET", "/v1/platform/bulk/"+companyJob, companyJob)
	notFound(h.Rollback, platformP, "POST", "/v1/platform/bulk/"+companyJob+"/rollback", companyJob)
	code, _ = call(t, h.Get, platformP, "GET", "/v1/platform/bulk/"+platformJob, platformJob, nil)
	require.Equal(t, http.StatusOK, code, "own platform job")

	// Nothing was rolled back through a foreign route.
	require.Equal(t, "completed", status(cardJob))
	require.Equal(t, "completed", status(companyJob))

	// --- The card job still rolls back through its own route (no recorded changes: nothing to revert).
	code, data := call(t, h.CardsRollback, adminP, "POST", "/v1/cards/bulk/"+cardJob+"/rollback", cardJob, nil)
	require.Equal(t, http.StatusOK, code, string(data))
	require.Equal(t, "rolled_back", status(cardJob))

	// --- An account in another company sees none of these.
	other, err := d.Queries.CreateCompanyAccount(ctx, db.CreateCompanyAccountParams{Name: "Diger Co", Slug: "diger-" + pgtest.Unique()})
	require.NoError(t, err)
	require.NoError(t, d.Queries.AddAccountMember(ctx, db.AddAccountMemberParams{AccountID: other.ID, UserID: admin.ID, Role: db.AccountRoleAdmin}))
	otherP := adminP
	otherP.Account.ID = other.ID
	require.Empty(t, listed(h.CompanyList, otherP, "/v1/company/bulk"))
	require.Empty(t, listed(h.CardsList, otherP, "/v1/cards/bulk"))
	notFound(h.CompanyGet, otherP, "GET", "/v1/company/bulk/"+companyJob, companyJob)
	notFound(h.CardsGet, otherP, "GET", "/v1/cards/bulk/"+cardJob, cardJob)
}
