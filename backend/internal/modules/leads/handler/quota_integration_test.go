package handler_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"context"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/leads/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/leads/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/leads/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/leads/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
	ioadapters "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/ioengine/adapters"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
	"github.com/stretchr/testify/require"
)

// planFor subscribes the account to a plan of its own (a copy of free) and returns a setter for
// its monthly limits, so the test can upgrade and downgrade without touching the seeded plans.
func (f *fixture) planFor(account int64) func(key string, value int32) {
	slug := "quota-" + pgtest.Unique()
	f.exec(`WITH p AS (
			INSERT INTO plans (slug, name, billing_interval, price_minor, currency, is_active)
			VALUES ($1, $1, 'month', 100, 'TRY', true) RETURNING id)
		INSERT INTO plan_limits (plan_id, feature_key, value_int, value_bool)
		SELECT p.id, l.feature_key, l.value_int, l.value_bool
		FROM p, plan_limits l JOIN plans fp ON fp.id = l.plan_id AND fp.slug = 'free'`, slug)
	f.exec(`INSERT INTO subscriptions (account_id, plan_id, status, source, starts_at)
		SELECT $1, id, 'active', 'manual', now() FROM plans WHERE slug = $2`, account, slug)
	return func(key string, value int32) {
		f.exec(`UPDATE plan_limits SET value_int = $3
			WHERE feature_key = $2 AND plan_id = (SELECT id FROM plans WHERE slug = $1)`, slug, key, value)
	}
}

type listedLead struct {
	UUID   string `json:"uuid"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Phone  string `json:"phone"`
	Locked bool   `json:"locked"`
}

// Acceptance (TEC-941): over the monthly quota leads are stored locked and masked in the list,
// the detail and the export; search does not match their hidden name; an upgrade unlocks them up
// to the new quota, a downgrade never re-locks, and a new month unlocks the rest. A quota of 0
// turns the form off.
func TestIntegrationLockedLeads(t *testing.T) {
	d := pgtest.Open(t)
	f := &fixture{t: t, ctx: context.Background(), d: d}
	owner := d.CreateUser(t)
	ownerP := authctx.Principal{UserID: owner.UUID, UserInternal: owner.ID,
		Account: authctx.Account{ID: owner.AccountID, Type: authctx.AccountTypePersonal, Role: "owner"}}
	setLimit := f.planFor(owner.AccountID)
	setLimit(entitlement.FeatureLeadsMonthlyMax, 2)

	var profile int64
	f.scan(`INSERT INTO profiles(account_id, user_id) VALUES ($1,$2) RETURNING id`, []any{owner.AccountID, owner.ID}, &profile)
	username := "lq-" + pgtest.Unique()
	f.exec(`INSERT INTO routes (path, route_type, status, target_type, profile_id, account_id, owner_user_id, presentation_config)
		VALUES ($1, 'canonical_profile', 'active', 'profile', $2, $3, $4, '{"lead_capture":{"enabled":true}}')`,
		username, profile, owner.AccountID, owner.ID)

	svc := usecase.New(repository.NewPostgres(d.Queries), nil, nil).SetQuotas(entitlement.New(d.Queries))
	h := handler.New(svc, nil)
	submit := func(name, email string) int {
		return call(h.Submit, http.MethodPost, "/v1/public/leads", nil, model.SubmitInput{
			Path: username, Name: name, Email: email, Phone: "+90 532 111 22 33",
			Consent: true, ConsentVersion: model.ConsentVersion,
		}, "").Code
	}
	list := func(q string) []listedLead {
		w := call(h.List, http.MethodGet, "/v1/leads?q="+q, &ownerP, nil, "")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var out struct {
			Data struct {
				Items []listedLead `json:"items"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
		return out.Data.Items
	}
	locked := func() (n int) {
		for _, l := range list("") {
			if l.Locked {
				n++
				require.Equal(t, "Z*** K***", l.Name)
				require.Equal(t, "z***@***", l.Email)
				require.Equal(t, "*** 33", l.Phone)
			}
		}
		return n
	}

	require.Equal(t, http.StatusCreated, submit("Ali Veli", "ali@example.test"))
	require.Equal(t, http.StatusCreated, submit("Ali Veli", "ali@example.test"))
	require.Equal(t, http.StatusCreated, submit("Zeynep Kaya", "zeynep@example.test"), "over quota is still stored")
	require.Equal(t, 1, locked())
	require.Empty(t, list("Zeynep"), "search matched a locked lead's hidden name")

	var lockedUUID string
	f.scan(`SELECT uuid::text FROM leads WHERE account_id=$1 AND locked`, []any{owner.AccountID}, &lockedUUID)
	w := call(h.Get, http.MethodGet, "/v1/leads/"+lockedUUID, &ownerP, nil, lockedUUID)
	require.Equal(t, http.StatusOK, w.Code)
	var detail struct {
		Data model.Lead `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &detail))
	require.True(t, detail.Data.Locked)
	require.Equal(t, "Z*** K***", detail.Data.Name)

	ds, err := ioadapters.NewLeads(d.Queries).Export(f.ctx, map[string]string{
		"account_id": strconvI(owner.AccountID), "viewer_user_id": strconvI(owner.ID),
	}, "tr")
	require.NoError(t, err)
	maskedRows := 0
	for _, row := range ds.Rows {
		require.NotEqual(t, "Zeynep Kaya", row["name"], "export leaked a locked lead")
		if row["locked"] == true {
			maskedRows++
			require.Equal(t, "z***@***", row["email"])
		}
	}
	require.Equal(t, 1, maskedRows)

	// Two more over the quota, then an upgrade to 3 unlocks the oldest locked one.
	require.Equal(t, http.StatusCreated, submit("Zeynep Kaya", "zeynep@example.test"))
	require.Equal(t, http.StatusCreated, submit("Zeynep Kaya", "zeynep@example.test"))
	require.Equal(t, 3, locked())
	setLimit(entitlement.FeatureLeadsMonthlyMax, 3)
	require.Equal(t, 2, locked())
	var stillLocked bool
	f.scan(`SELECT locked FROM leads WHERE uuid=$1`, []any{lockedUUID}, &stillLocked)
	require.False(t, stillLocked, "the oldest locked lead unlocks first")
	setLimit(entitlement.FeatureLeadsMonthlyMax, 1)
	require.Equal(t, 2, locked(), "a downgrade must not re-lock")

	// A new month unlocks every earlier locked lead.
	f.exec(`UPDATE leads SET created_at = created_at - interval '40 days' WHERE account_id=$1`, owner.AccountID)
	require.Equal(t, 0, locked())
	require.Len(t, list("Zeynep"), 3)

	// 0 turns the form off.
	setLimit(entitlement.FeatureLeadsMonthlyMax, 0)
	require.Equal(t, http.StatusNotFound, submit("Ali Veli", "ali@example.test"))
	cfg, err := svc.FormConfig(f.ctx, username)
	require.NoError(t, err)
	require.False(t, cfg.Enabled)

	// Export quota: the free copy allows 2 a month.
	require.NoError(t, svc.PrepareExport(f.ctx, ownerP))
	require.NoError(t, svc.PrepareExport(f.ctx, ownerP))
	err = svc.PrepareExport(f.ctx, ownerP)
	require.True(t, errors.Is(err, entitlement.ErrPlanLimitExceeded), "third export: %v", err)
}
