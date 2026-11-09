package usecase_test

// These tests use the repository's TEST_DATABASE_URL harness, so the full verification command
// exercises them alongside the other account-scoped integration tests.
import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyassets/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyassets/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyassets/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyassets/usecase"
	omodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/model"
	orepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/repository"
	orders "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/usecase"
	products "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/products/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) { pgtest.Main(m) }

type quotas struct {
	err     error
	account int64
}

func (q *quotas) CheckQuota(_ context.Context, account int64, _ bool, _ string) error {
	q.account = account
	return q.err
}

type fixture struct {
	t                             *testing.T
	d                             *pgtest.DB
	ctx                           context.Context
	svc                           *usecase.Service
	quotas                        *quotas
	owner, manager, member, other authctx.Principal
	office, otherOffice, group    uuid.UUID
	profile, otherProfile         uuid.UUID
	card, otherCard, unassigned   uuid.UUID
	account                       int64
}

func setup(t *testing.T) *fixture {
	t.Helper()
	d := pgtest.Open(t)
	f := &fixture{t: t, d: d, ctx: context.Background(), quotas: &quotas{}}
	account, err := d.Queries.CreateCompanyAccount(f.ctx, db.CreateCompanyAccountParams{Name: "Assets", Slug: "assets-" + pgtest.Unique()})
	require.NoError(t, err)
	f.account = account.ID
	f.owner = f.actor(d.CreateUser(t), "owner")
	f.manager = f.actor(d.CreateUser(t), "office_manager")
	f.member = f.actor(d.CreateUser(t), "member")
	f.other = f.actor(d.CreateUser(t), "member")
	var officeID, otherOfficeID, groupID int64
	require.NoError(t, d.Pool.QueryRow(f.ctx, `INSERT INTO company_offices(account_id,name) VALUES ($1,'East') RETURNING id,uuid`, f.account).Scan(&officeID, &f.office))
	require.NoError(t, d.Pool.QueryRow(f.ctx, `INSERT INTO company_offices(account_id,name) VALUES ($1,'West') RETURNING id,uuid`, f.account).Scan(&otherOfficeID, &f.otherOffice))
	require.NoError(t, d.Pool.QueryRow(f.ctx, `INSERT INTO company_groups(account_id,name,kind,office_id) VALUES ($1,'Sales','team',$2) RETURNING id,uuid`, f.account, officeID).Scan(&groupID, &f.group))
	f.exec(`INSERT INTO account_member_scopes(account_id,member_id,office_id) SELECT $1,id,$3 FROM account_members WHERE account_id=$1 AND user_id=$2`, f.account, f.manager.UserInternal, officeID)
	f.exec(`INSERT INTO company_member_assignments(account_id,member_id,office_id,group_id) SELECT $1,id,$3,$4 FROM account_members WHERE account_id=$1 AND user_id=$2`, f.account, f.member.UserInternal, officeID, groupID)
	f.exec(`INSERT INTO company_member_assignments(account_id,member_id,office_id) SELECT $1,id,$3 FROM account_members WHERE account_id=$1 AND user_id=$2`, f.account, f.other.UserInternal, otherOfficeID)
	f.profile = f.link(f.member)
	f.otherProfile = f.link(f.other)
	f.card = f.newCard(&f.profile, "Alpha")
	f.otherCard = f.newCard(&f.otherProfile, "Zulu")
	f.unassigned = f.newCard(nil, "Unassigned")
	f.svc = usecase.New(repository.New(d.Pool), f.quotas, nil)
	f.svc.SetOrders(func(q *db.Queries) usecase.OrderCreator {
		return orders.NewService(orepo.NewPostgres(nil, q), products.NewPostgres(q), nil, nil, "https://comcode.test")
	}, nil)
	return f
}
func (f *fixture) actor(u pgtest.User, role string) authctx.Principal {
	f.t.Helper()
	require.NoError(f.t, f.d.Queries.AddAccountMember(f.ctx, db.AddAccountMemberParams{AccountID: f.account, UserID: u.ID, Role: db.AccountRole(role)}))
	return authctx.Principal{UserID: u.UUID, UserInternal: u.ID, Account: authctx.Account{ID: f.account, Type: "company", Role: role}, Permissions: []string{
		rbac.PermAppCardsRead, rbac.PermAppCardsWrite, rbac.PermAppOrdersRead, rbac.PermAppOrdersWrite,
	}}
}
func (f *fixture) exec(query string, args ...any) {
	f.t.Helper()
	_, err := f.d.Pool.Exec(f.ctx, query, args...)
	require.NoError(f.t, err)
}
func (f *fixture) link(p authctx.Principal) uuid.UUID {
	f.t.Helper()
	var id int64
	var public uuid.UUID
	// Creator is deliberately the admin: it must not decide employee ownership.
	require.NoError(f.t, f.d.Pool.QueryRow(f.ctx, `INSERT INTO profiles(account_id,user_id) VALUES ($1,$2) RETURNING id,uuid`, f.account, f.owner.UserInternal).Scan(&id, &public))
	f.exec(`INSERT INTO company_member_profiles(account_id,member_id,profile_id) SELECT $1,id,$3 FROM account_members WHERE account_id=$1 AND user_id=$2`, f.account, p.UserInternal, id)
	return public
}
func (f *fixture) newCard(profile *uuid.UUID, name string) uuid.UUID {
	f.t.Helper()
	var id int64
	var public uuid.UUID
	code := "cc-" + pgtest.Unique()
	require.NoError(f.t, f.d.Pool.QueryRow(f.ctx, `INSERT INTO nfc_cards(account_id,user_id,name,smart_code,profile_id,status) VALUES ($1,$2,$3,$4,(SELECT id FROM profiles WHERE account_id=$1 AND uuid=$5),'active') RETURNING id,uuid`, f.account, f.owner.UserInternal, name, code, profile).Scan(&id, &public))
	f.exec(`INSERT INTO routes(account_id,owner_user_id,card_id,path,route_type,status,is_immutable,profile_id) SELECT account_id,user_id,id,'c/'||smart_code,'card_route','active',true,profile_id FROM nfc_cards WHERE account_id=$1 AND id=$2`, f.account, id)
	return public
}
func (f *fixture) newOrder(p authctx.Principal, profile *uuid.UUID) *omodel.OrderDetail {
	f.t.Helper()
	svc := orders.NewService(orepo.NewPostgres(f.d.Pool, f.d.Queries), products.NewPostgres(f.d.Queries), nil, nil, "")
	var profileString *string
	if profile != nil {
		v := profile.String()
		profileString = &v
	}
	account := p.Account
	account.Role = "owner"
	out, err := svc.CreateForAccount(f.ctx, account, p.UserInternal, omodel.CreateOrderInput{Items: []omodel.CreateOrderItemInput{
		{ProductUUID: f.d.CreateProduct(f.t, 100).String(), Quantity: 1, ProfileUUID: profileString},
	}})
	require.NoError(f.t, err)
	return out
}

func TestCardsScopeAndFilters(t *testing.T) {
	f := setup(t)
	for _, tc := range []struct {
		name  string
		actor authctx.Principal
		count int64
	}{
		{name: "owner sees company", actor: f.owner, count: 3},
		{name: "office manager sees granted office", actor: f.manager, count: 1},
		{name: "member sees linked profile despite admin creator", actor: f.member, count: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			items, total, err := f.svc.Cards(f.ctx, tc.actor, model.Filter{})
			require.NoError(t, err)
			require.Equal(t, tc.count, total)
			require.Len(t, items, int(total))
		})
	}
	items, total, err := f.svc.Cards(f.ctx, f.owner, model.Filter{OfficeUUID: &f.office, GroupUUID: &f.group, Q: "Alpha", Status: "active", Limit: 1})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, f.card, items[0].UUID)
	require.Equal(t, f.member.UserID, *items[0].MemberUUID)
	require.Equal(t, f.profile, *items[0].ProfileUUID)
	require.True(t, items[0].IsImmutable)
	require.NotEmpty(t, items[0].RoutePath)
	items, total, err = f.svc.Cards(f.ctx, f.owner, model.Filter{Sort: "name", Limit: 1, Offset: 1})
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	require.Equal(t, "Unassigned", items[0].Name)
	_, total, err = f.svc.Cards(f.ctx, f.manager, model.Filter{OfficeUUID: &f.otherOffice})
	require.NoError(t, err)
	require.Zero(t, total)
	_, _, err = f.svc.Cards(f.ctx, f.owner, model.Filter{Sort: "name;drop table users"})
	require.Error(t, err)
	f.exec(`DELETE FROM account_member_scopes WHERE account_id=$1`, f.account)
	_, total, err = f.svc.Cards(f.ctx, f.manager, model.Filter{})
	require.NoError(t, err)
	require.Zero(t, total)
}

func TestCrossAccountAndLiveMembership(t *testing.T) {
	f := setup(t)
	g := setup(t)
	g.newOrder(g.owner, &g.profile)
	_, orderTotal, orderErr := f.svc.Orders(f.ctx, f.owner, model.Filter{})
	require.NoError(t, orderErr)
	require.Zero(t, orderTotal)
	require.ErrorIs(t, f.svc.AssignCard(f.ctx, f.owner, g.card, f.member.UserID), model.ErrNotFound)
	require.ErrorIs(t, f.svc.AssignCard(f.ctx, f.owner, f.card, g.member.UserID), model.ErrNotFound)
	require.ErrorIs(t, f.svc.SetCardActive(f.ctx, f.owner, g.card, false), model.ErrNotFound)
	_, total, err := f.svc.Cards(f.ctx, f.owner, model.Filter{OfficeUUID: &g.office})
	require.NoError(t, err)
	require.Zero(t, total)
	personal := f.owner
	personal.Account.Type = "personal"
	_, _, err = f.svc.Cards(f.ctx, personal, model.Filter{})
	require.ErrorIs(t, err, model.ErrForbidden)
	// A stale principal with an owner role cannot survive live demotion.
	f.actor(f.d.CreateUser(t), "owner")
	f.exec(`UPDATE account_members SET role='member' WHERE account_id=$1 AND user_id=$2`, f.account, f.owner.UserInternal)
	require.ErrorIs(t, f.svc.SetCardActive(f.ctx, f.owner, f.card, false), model.ErrForbidden)
	missing := f.owner
	missing.Permissions = nil
	_, _, err = f.svc.Orders(f.ctx, missing, model.Filter{})
	require.ErrorIs(t, err, model.ErrForbidden)
}

func TestAssignCardScopesAndRouteConsistency(t *testing.T) {
	f := setup(t)
	require.ErrorIs(t, f.svc.AssignCard(f.ctx, f.member, f.card, f.other.UserID), model.ErrForbidden)
	require.ErrorIs(t, f.svc.AssignCard(f.ctx, f.manager, f.card, f.other.UserID), model.ErrNotFound)
	require.ErrorIs(t, f.svc.AssignCard(f.ctx, f.manager, f.otherCard, f.member.UserID), model.ErrNotFound)
	require.ErrorIs(t, f.svc.AssignCard(f.ctx, f.manager, f.unassigned, f.member.UserID), model.ErrNotFound)
	require.NoError(t, f.svc.AssignCard(f.ctx, f.manager, f.card, f.member.UserID))
	// Admin may claim an unassigned company card and move a card across offices.
	require.NoError(t, f.svc.AssignCard(f.ctx, f.owner, f.unassigned, f.member.UserID))
	require.NoError(t, f.svc.AssignCard(f.ctx, f.owner, f.card, f.other.UserID))
	var cardProfile, routeProfile uuid.UUID
	var immutable bool
	var path, code, target string
	require.NoError(t, f.d.Pool.QueryRow(f.ctx, `SELECT p.uuid,rp.uuid,r.is_immutable,r.path,c.smart_code,r.target_type FROM nfc_cards c JOIN profiles p ON p.id=c.profile_id JOIN routes r ON r.card_id=c.id JOIN profiles rp ON rp.id=r.profile_id WHERE c.account_id=$1 AND c.uuid=$2`, f.account, f.card).Scan(&cardProfile, &routeProfile, &immutable, &path, &code, &target))
	require.Equal(t, f.otherProfile, cardProfile)
	require.Equal(t, cardProfile, routeProfile)
	require.True(t, immutable)
	require.Equal(t, "c/"+code, path)
	require.Equal(t, "profile", target)
	_, total, err := f.svc.Cards(f.ctx, f.member, model.Filter{})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
}

func TestCardActiveAndLifecycleLocks(t *testing.T) {
	f := setup(t)
	require.ErrorIs(t, f.svc.SetCardActive(f.ctx, f.member, f.card, false), model.ErrForbidden)
	require.ErrorIs(t, f.svc.SetCardActive(f.ctx, f.manager, f.otherCard, false), model.ErrNotFound)
	require.NoError(t, f.svc.SetCardActive(f.ctx, f.manager, f.card, false))
	active := false
	items, total, err := f.svc.Cards(f.ctx, f.owner, model.Filter{IsActive: &active})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, f.card, items[0].UUID)
	f.quotas.err = entitlement.ErrPlanLimitExceeded
	require.ErrorIs(t, f.svc.SetCardActive(f.ctx, f.manager, f.card, true), entitlement.ErrPlanLimitExceeded)
	f.quotas.err = nil
	require.NoError(t, f.svc.SetCardActive(f.ctx, f.manager, f.card, true))
	require.Equal(t, f.account, f.quotas.account)
	for _, state := range []string{"reserved", "retired"} {
		t.Run(state, func(t *testing.T) {
			f.exec(`UPDATE routes SET status=$3 WHERE account_id=$1 AND card_id=(SELECT id FROM nfc_cards WHERE uuid=$2)`, f.account, f.card, state)
			require.ErrorIs(t, f.svc.SetCardActive(f.ctx, f.owner, f.card, true), model.ErrConflict)
			require.ErrorIs(t, f.svc.AssignCard(f.ctx, f.owner, f.card, f.member.UserID), model.ErrConflict)
		})
	}
	f.exec(`UPDATE routes SET status='active' WHERE account_id=$1 AND card_id=(SELECT id FROM nfc_cards WHERE uuid=$2)`, f.account, f.card)
	ord := f.newOrder(f.owner, &f.profile)
	f.exec(`UPDATE nfc_cards SET order_id=$3 WHERE account_id=$1 AND uuid=$2`, f.account, f.card, ord.ID)
	for _, state := range []string{"in_production", "cancelled", "refunded"} {
		t.Run(state, func(t *testing.T) {
			f.exec(`UPDATE orders SET status=$3 WHERE account_id=$1 AND id=$2`, f.account, ord.ID, state)
			require.ErrorIs(t, f.svc.AssignCard(f.ctx, f.owner, f.card, f.member.UserID), model.ErrConflict)
			require.ErrorIs(t, f.svc.SetCardActive(f.ctx, f.owner, f.card, false), model.ErrConflict)
		})
	}
}

func TestOrdersScopeAndPaging(t *testing.T) {
	f := setup(t)
	own := f.newOrder(f.member, nil)
	recipient := f.newOrder(f.owner, &f.profile)
	f.newOrder(f.other, nil)
	personal := f.member
	u := f.d.CreateUser(t)
	personal.UserInternal = u.ID
	personal.Account = authctx.Account{ID: u.AccountID, Role: "owner"}
	f.newOrder(personal, nil)
	for _, tc := range []struct {
		name  string
		p     authctx.Principal
		count int64
	}{
		{name: "owner company only", p: f.owner, count: 3},
		{name: "office manager", p: f.manager, count: 2},
		{name: "own placer or recipient", p: f.member, count: 2},
		{name: "other office", p: f.other, count: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			items, total, err := f.svc.Orders(f.ctx, tc.p, model.Filter{Limit: 1})
			require.NoError(t, err)
			require.Equal(t, tc.count, total)
			require.Len(t, items, 1)
		})
	}
	items, total, err := f.svc.Orders(f.ctx, f.manager, model.Filter{Q: own.OrderNumber, OfficeUUID: &f.office, GroupUUID: &f.group, Status: "pending", Sort: "order_number"})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, own.UUID, items[0].UUID.String())
	_, total, err = f.svc.Orders(f.ctx, f.manager, model.Filter{Q: recipient.OrderNumber, OfficeUUID: &f.otherOffice})
	require.NoError(t, err)
	require.Zero(t, total)
	_, total, err = f.svc.Orders(f.ctx, f.owner, model.Filter{Offset: 100})
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
}

func TestReorderCreatesPendingCompanyOrder(t *testing.T) {
	f := setup(t)
	product := f.d.CreateProduct(t, 100)
	input := model.ReorderInput{MemberUUIDs: []uuid.UUID{f.member.UserID, f.other.UserID}, ProductUUID: product}
	require.ErrorIs(t, func() error { _, err := f.svc.Reorder(f.ctx, f.member, input); return err }(), model.ErrForbidden)
	require.ErrorIs(t, func() error { _, err := f.svc.Reorder(f.ctx, f.manager, input); return err }(), model.ErrForbidden)
	published := false
	f.svc.SetOrders(func(q *db.Queries) usecase.OrderCreator {
		return orders.NewService(orepo.NewPostgres(nil, q), products.NewPostgres(q), nil, nil, "")
	}, func(ctx context.Context, account authctx.Account, id uuid.UUID) {
		// Separate connection observes a fully committed order and all its items.
		var count int
		require.NoError(t, f.d.Pool.QueryRow(ctx, `SELECT count(*) FROM order_items oi JOIN orders o ON o.id=oi.order_id WHERE o.account_id=$1 AND o.uuid=$2`, account.ID, id).Scan(&count))
		require.Equal(t, 2, count)
		published = true
	})
	out, err := f.svc.Reorder(f.ctx, f.owner, input)
	require.NoError(t, err)
	require.True(t, published)
	require.Equal(t, f.account, out.AccountID)
	require.Equal(t, "pending", out.Status)
	require.Equal(t, "unpaid", out.PaymentStatus)
	require.Len(t, out.Items, 2)
	require.Equal(t, 240.0, out.GrandTotal)
	for _, item := range out.Items {
		require.Equal(t, "unprovisioned", item.ProvisioningStatus)
		require.Nil(t, item.CardUUID)
		require.NotNil(t, item.ProfileUUID)
	}
	// A rejected target never leaves a header or partially created items behind.
	input.MemberUUIDs = []uuid.UUID{f.member.UserID, uuid.New()}
	_, err = f.svc.Reorder(f.ctx, f.owner, input)
	require.ErrorIs(t, err, model.ErrNotFound)
	var count int
	require.NoError(t, f.d.Pool.QueryRow(f.ctx, `SELECT count(*) FROM orders WHERE account_id=$1`, f.account).Scan(&count))
	require.Equal(t, 1, count)
	input.MemberUUIDs = []uuid.UUID{f.member.UserID, f.member.UserID}
	_, err = f.svc.Reorder(f.ctx, f.owner, input)
	require.ErrorIs(t, err, model.ErrInvalid)
}

func TestCompanyAssetHTTPContract(t *testing.T) {
	f := setup(t)
	h := handler.New(f.svc)
	for _, tc := range []struct {
		name, method, path, body string
		principal                authctx.Principal
		serve                    http.HandlerFunc
		status                   int
	}{
		{name: "card list", method: "GET", path: "/v1/company/cards?limit=1", principal: f.owner, serve: h.Cards, status: 200},
		{name: "unknown sort", method: "GET", path: "/v1/company/cards?sort=unknown", principal: f.owner, serve: h.Cards, status: 400},
		{name: "multiple sort", method: "GET", path: "/v1/company/orders?sort=status,created_at", principal: f.owner, serve: h.Orders, status: 400},
		{name: "banned alias", method: "GET", path: "/v1/company/cards?search=x", principal: f.owner, serve: h.Cards, status: 400},
		{name: "bad office", method: "GET", path: "/v1/company/orders?office_uuid=bad", principal: f.owner, serve: h.Orders, status: 400},
		{name: "bad active", method: "GET", path: "/v1/company/cards?is_active=bad", principal: f.owner, serve: h.Cards, status: 400},
		{name: "missing active", method: "PUT", path: "/v1/company/cards/uuid/active", body: "{}", principal: f.owner, serve: h.SetCardActive, status: 400},
		{name: "trailing body", method: "PUT", path: "/v1/company/cards/uuid/active", body: `{"is_active":false} {}`, principal: f.owner, serve: h.SetCardActive, status: 400},
		{name: "unknown field", method: "PUT", path: "/v1/company/cards/uuid/assignment", body: `{"member_uuid":"` + f.member.UserID.String() + `","account_id":1}`, principal: f.owner, serve: h.AssignCard, status: 400},
		{name: "member cannot disable", method: "PUT", path: "/v1/company/cards/uuid/active", body: `{"is_active":false}`, principal: f.member, serve: h.SetCardActive, status: 403},
		{name: "reorder empty selection", method: "POST", path: "/v1/company/orders/reorder", body: `{}`, principal: f.owner, serve: h.Reorder, status: 400},
		{name: "card metadata", method: "GET", path: "/v1/company/cards/meta", principal: f.member, serve: h.CardsMeta, status: 200},
		{name: "order metadata", method: "GET", path: "/v1/company/orders/meta", principal: f.manager, serve: h.OrdersMeta, status: 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			r.SetPathValue("uuid", f.card.String())
			r = r.WithContext(authctx.WithPrincipal(r.Context(), tc.principal))
			w := httptest.NewRecorder()
			tc.serve(w, r)
			require.Equal(t, tc.status, w.Code, w.Body.String())
			var envelope struct {
				Success bool `json:"success"`
				Error   struct {
					Code    string `json:"code"`
					Details []struct {
						Field string `json:"field"`
					} `json:"details"`
				} `json:"error"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope))
			require.Equal(t, tc.status == 200, envelope.Success)
			if tc.name == "unknown sort" || tc.name == "multiple sort" {
				require.Equal(t, "VALIDATION_ERROR", envelope.Error.Code)
				require.Equal(t, "sort", envelope.Error.Details[0].Field)
			}
		})
	}
}

func TestReorderRollsBackItemFailure(t *testing.T) {
	f := setup(t)
	f.exec(`ALTER TABLE order_items ADD CONSTRAINT tec572b_reject_sales CHECK (recipient_department <> 'Sales') NOT VALID`)
	t.Cleanup(func() { f.exec(`ALTER TABLE order_items DROP CONSTRAINT tec572b_reject_sales`) })
	_, err := f.svc.Reorder(f.ctx, f.owner, model.ReorderInput{
		MemberUUIDs: []uuid.UUID{f.other.UserID, f.member.UserID}, ProductUUID: f.d.CreateProduct(t, 100),
	})
	require.Error(t, err)
	var count int
	require.NoError(t, f.d.Pool.QueryRow(f.ctx, `SELECT count(*) FROM orders WHERE account_id=$1`, f.account).Scan(&count))
	require.Zero(t, count)
}

func TestActivatedCompanyCardKeepsPurchaseOrderLifecycleGuard(t *testing.T) {
	f := setup(t)
	purchaser := f.d.CreateUser(t)
	personal := authctx.Principal{
		UserInternal: purchaser.ID,
		Account:      authctx.Account{ID: purchaser.AccountID, Role: "owner"},
	}
	original := f.newOrder(personal, nil)
	// Activation changes the card account, while its original purchase order stays personal.
	f.exec(`UPDATE nfc_cards SET order_id=$3 WHERE account_id=$1 AND uuid=$2`, f.account, f.card, original.ID)
	require.NoError(t, f.svc.SetCardActive(f.ctx, f.owner, f.card, false))
	require.NoError(t, f.svc.SetCardActive(f.ctx, f.owner, f.card, true))
	require.NoError(t, f.svc.AssignCard(f.ctx, f.owner, f.card, f.other.UserID))
	_, total, err := f.svc.Orders(f.ctx, f.owner, model.Filter{})
	require.NoError(t, err)
	require.Zero(t, total) // The private purchase order is never listed in the company.
	f.exec(`UPDATE orders SET status='in_production' WHERE account_id=$1 AND id=$2`, purchaser.AccountID, original.ID)
	require.ErrorIs(t, f.svc.SetCardActive(f.ctx, f.owner, f.card, false), model.ErrConflict)
	require.ErrorIs(t, f.svc.AssignCard(f.ctx, f.owner, f.card, f.member.UserID), model.ErrConflict)
}
