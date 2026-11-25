package handler_test

// Card bulk endpoints (TEC-596) against Postgres: the owner/admin role and the action permission
// gate them, a run only reaches the active account's cards, jobs of other accounts or other
// resources are 404, and rollback restores the lock states.

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/config"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/bulk/handler"
	bulkusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/bulk/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/bulkengine"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/bulkengine/adapters"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) { pgtest.Main(m) }

type envelope struct {
	Data json.RawMessage `json:"data"`
}

func call(t *testing.T, h http.HandlerFunc, p authctx.Principal, method, path, jobUUID string, body any) (int, json.RawMessage) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	r := httptest.NewRequest(method, path, &buf)
	if jobUUID != "" {
		r.SetPathValue("uuid", jobUUID)
	}
	r = r.WithContext(authctx.WithPrincipal(r.Context(), p))
	w := httptest.NewRecorder()
	h(w, r)
	var env envelope
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	return w.Code, env.Data
}

func TestIntegrationCardsBulkEndpoints(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	reg := bulkengine.NewRegistry(adapters.NewUsers(d.Queries), adapters.NewCards(d.Queries, nil))
	h := handler.New(bulkusecase.New(d.Queries, reg, nil, nil, nil, config.BulkConfig{SyncMax: 50, RollbackHours: 24}, log))

	company, err := d.Queries.CreateCompanyAccount(ctx, db.CreateCompanyAccountParams{Name: "Kart Co", Slug: "kart-" + pgtest.Unique()})
	require.NoError(t, err)
	perms := []string{rbac.PermAppCardsWrite, rbac.PermAppCardsBulkLock, rbac.PermAppCardsBulkUnlock}
	member := func(role string) (pgtest.User, authctx.Principal) {
		u := d.CreateUser(t)
		require.NoError(t, d.Queries.AddAccountMember(ctx, db.AddAccountMemberParams{AccountID: company.ID, UserID: u.ID, Role: db.AccountRole(role)}))
		return u, authctx.Principal{UserID: u.UUID, UserInternal: u.ID, Permissions: perms,
			Account: authctx.Account{ID: company.ID, Type: authctx.AccountTypeCompany, Role: role}}
	}
	admin, adminP := member("admin")
	_, memberP := member("member")
	cardOf := func(owner pgtest.User, account int64) uuid.UUID {
		var id uuid.UUID
		require.NoError(t, d.Pool.QueryRow(ctx, `INSERT INTO nfc_cards (user_id, account_id, name, smart_code) VALUES ($1, $2, 'Kart', $3) RETURNING uuid`,
			owner.ID, account, "cc-"+pgtest.Unique()).Scan(&id))
		return id
	}
	c1, c2 := cardOf(admin, company.ID), cardOf(admin, company.ID)
	outsider := d.CreateUser(t)
	foreign := cardOf(outsider, outsider.AccountID)
	outsiderP := authctx.Principal{UserID: outsider.UUID, UserInternal: outsider.ID, Permissions: perms,
		Account: authctx.Account{ID: outsider.AccountID, Type: authctx.AccountTypePersonal, Role: authctx.AccountRoleOwner}}
	isLocked := func(id uuid.UUID) bool {
		var v bool
		require.NoError(t, d.Pool.QueryRow(ctx, `SELECT is_locked FROM nfc_cards WHERE uuid = $1`, id).Scan(&v))
		return v
	}
	lockAll := map[string]any{"action": "lock", "target": map[string]any{"scope": "query"}}

	// A plain member, or an admin without the action permission, is refused.
	code, _ := call(t, h.ExecuteCards, memberP, "POST", "/v1/cards/bulk", "", lockAll)
	require.Equal(t, http.StatusForbidden, code)
	noPerm := adminP
	noPerm.Permissions = []string{rbac.PermAppCardsWrite}
	code, _ = call(t, h.ExecuteCards, noPerm, "POST", "/v1/cards/bulk", "", lockAll)
	require.Equal(t, http.StatusForbidden, code)
	code, _ = call(t, h.ExecuteCards, adminP, "POST", "/v1/cards/bulk", "", map[string]any{"action": "disable", "target": map[string]any{"scope": "query"}})
	require.Equal(t, http.StatusBadRequest, code)
	require.False(t, isLocked(c1))

	// The admin locks every card of the company; the outsider's card is not reached.
	code, data := call(t, h.ExecuteCards, adminP, "POST", "/v1/cards/bulk", "", lockAll)
	require.Equal(t, http.StatusOK, code, string(data))
	var sync struct {
		Summary bulkengine.BulkSummary  `json:"summary"`
		Job     bulkusecase.BulkJobView `json:"job"`
	}
	require.NoError(t, json.Unmarshal(data, &sync))
	require.Equal(t, 2, sync.Summary.Succeeded)
	require.True(t, isLocked(c1) && isLocked(c2))
	require.False(t, isLocked(foreign))

	// Ids from another account fail per item.
	code, data = call(t, h.ExecuteCards, adminP, "POST", "/v1/cards/bulk", "",
		map[string]any{"action": "unlock", "target": map[string]any{"scope": "ids", "ids": []string{foreign.String(), c2.String()}}})
	require.Equal(t, http.StatusOK, code, string(data))
	var partial struct {
		Summary bulkengine.BulkSummary `json:"summary"`
	}
	require.NoError(t, json.Unmarshal(data, &partial))
	require.Equal(t, 1, partial.Summary.Succeeded)
	require.Equal(t, 1, partial.Summary.Failed)
	require.False(t, isLocked(c2))

	// Jobs are listed and read only in their account.
	jobID := sync.Job.UUID.String()
	code, _ = call(t, h.CardsGet, adminP, "GET", "/v1/cards/bulk/"+jobID, jobID, nil)
	require.Equal(t, http.StatusOK, code)
	code, _ = call(t, h.CardsGet, outsiderP, "GET", "/v1/cards/bulk/"+jobID, jobID, nil)
	require.Equal(t, http.StatusNotFound, code)
	code, _ = call(t, h.CardsGet, memberP, "GET", "/v1/cards/bulk/"+jobID, jobID, nil)
	require.Equal(t, http.StatusForbidden, code)
	code, data = call(t, h.CardsList, adminP, "GET", "/v1/cards/bulk", "", nil)
	require.Equal(t, http.StatusOK, code)
	var page struct {
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal(data, &page))
	require.Equal(t, 2, page.Total)
	code, data = call(t, h.CardsList, outsiderP, "GET", "/v1/cards/bulk", "", nil)
	require.Equal(t, http.StatusOK, code)
	require.NoError(t, json.Unmarshal(data, &page))
	require.Equal(t, 0, page.Total)

	// Rollback of the lock job brings c1 back to unlocked (c2 was unlocked meanwhile anyway).
	code, _ = call(t, h.CardsRollback, outsiderP, "POST", "/v1/cards/bulk/"+jobID+"/rollback", jobID, nil)
	require.Equal(t, http.StatusNotFound, code)
	code, data = call(t, h.CardsRollback, adminP, "POST", "/v1/cards/bulk/"+jobID+"/rollback", jobID, nil)
	require.Equal(t, http.StatusOK, code, string(data))
	require.False(t, isLocked(c1))
	require.False(t, isLocked(c2))
}
