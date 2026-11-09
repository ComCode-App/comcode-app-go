package usecase_test

// Postgres integration tests (TEST_DATABASE_URL, see internal/testutil/pgtest): aggregation,
// attribution, account isolation, role scope and date range of company analytics (TEC-574).
import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyanalytics/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyanalytics/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyanalytics/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyanalytics/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) { pgtest.Main(m) }

var today = time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)

type history struct {
	days      int32
	unlimited bool
}

func (h history) HistoryWindow(context.Context, int64, bool) (int32, bool, error) {
	return h.days, h.unlimited, nil
}

type fixture struct {
	t   *testing.T
	d   *pgtest.DB
	ctx context.Context
	svc *usecase.Service

	account                                    int64
	owner, admin, manager, idleManager, member authctx.Principal
	east, west, sales                          uuid.UUID
	p1, p2, p3                                 uuid.UUID // p1: East/Sales employee, p2: West employee, p3: unlinked
	card1, cardRoute1, smartLink, p1Link       uuid.UUID
	otherProfile, otherRoute                   uuid.UUID
}

func setup(t *testing.T) *fixture {
	t.Helper()
	d := pgtest.Open(t)
	f := &fixture{t: t, d: d, ctx: context.Background()}
	acc, err := d.Queries.CreateCompanyAccount(f.ctx, db.CreateCompanyAccountParams{Name: "Analytics Co", Slug: "analytics-" + pgtest.Unique()})
	require.NoError(t, err)
	f.account = acc.ID
	f.owner = f.actor(f.account, d.CreateUser(t), "owner")
	f.admin = f.actor(f.account, d.CreateUser(t), "admin")
	f.manager = f.actor(f.account, d.CreateUser(t), "office_manager")
	f.idleManager = f.actor(f.account, d.CreateUser(t), "office_manager")
	f.member = f.actor(f.account, d.CreateUser(t), "member")
	e1 := f.actor(f.account, d.CreateUser(t), "member")
	e2 := f.actor(f.account, d.CreateUser(t), "member")

	var eastID, westID, salesID int64
	f.scan(`INSERT INTO company_offices(account_id,name) VALUES ($1,'East') RETURNING id,uuid`, []any{f.account}, &eastID, &f.east)
	f.scan(`INSERT INTO company_offices(account_id,name) VALUES ($1,'West') RETURNING id,uuid`, []any{f.account}, &westID, &f.west)
	f.scan(`INSERT INTO company_groups(account_id,name,kind,office_id) VALUES ($1,'Sales','team',$2) RETURNING id,uuid`, []any{f.account, eastID}, &salesID, &f.sales)
	f.exec(`INSERT INTO account_member_scopes(account_id,member_id,office_id) SELECT $1,id,$3 FROM account_members WHERE account_id=$1 AND user_id=$2`, f.account, f.manager.UserInternal, eastID)
	f.exec(`INSERT INTO company_member_assignments(account_id,member_id,office_id,group_id) SELECT $1,id,$3,$4 FROM account_members WHERE account_id=$1 AND user_id=$2`, f.account, e1.UserInternal, eastID, salesID)
	f.exec(`INSERT INTO company_member_assignments(account_id,member_id,office_id) SELECT $1,id,$3 FROM account_members WHERE account_id=$1 AND user_id=$2`, f.account, e2.UserInternal, westID)

	f.p1 = f.profile(f.account, "Ayla", "Kaya", &e1)
	f.p2 = f.profile(f.account, "Burak", "Demir", &e2)
	f.p3 = f.profile(f.account, "Zeynep", "Unlinked", nil)
	f.card1, f.cardRoute1 = f.card(f.account, f.p1)
	f.smartLink = f.route(f.account, "custom_smart_link", nil)
	f.p1Link = f.route(f.account, "custom_smart_link", &f.p1)

	// Another company: its own profile, and a route of its own that targets our p1.
	other, err := d.Queries.CreateCompanyAccount(f.ctx, db.CreateCompanyAccountParams{Name: "Other Co", Slug: "other-" + pgtest.Unique()})
	require.NoError(t, err)
	f.actor(other.ID, d.CreateUser(t), "owner")
	f.otherProfile = f.profile(other.ID, "Other", "Person", nil)
	f.otherRoute = f.route(other.ID, "custom_smart_link", &f.p1)

	f.svc = usecase.New(repository.New(d.Pool), history{unlimited: true})
	f.svc.SetNow(func() time.Time { return today.Add(15 * time.Hour) })
	return f
}

func (f *fixture) actor(account int64, u pgtest.User, role string) authctx.Principal {
	f.t.Helper()
	require.NoError(f.t, f.d.Queries.AddAccountMember(f.ctx, db.AddAccountMemberParams{AccountID: account, UserID: u.ID, Role: db.AccountRole(role)}))
	return authctx.Principal{UserID: u.UUID, UserInternal: u.ID,
		Account:     authctx.Account{ID: account, Type: authctx.AccountTypeCompany, Role: role},
		Permissions: []string{rbac.PermAppCompanyAnalyticsRead}}
}

func (f *fixture) exec(q string, args ...any) {
	f.t.Helper()
	_, err := f.d.Pool.Exec(f.ctx, q, args...)
	require.NoError(f.t, err)
}

func (f *fixture) scan(q string, args []any, dest ...any) {
	f.t.Helper()
	require.NoError(f.t, f.d.Pool.QueryRow(f.ctx, q, args...).Scan(dest...))
}

func (f *fixture) profile(account int64, first, last string, member *authctx.Principal) uuid.UUID {
	f.t.Helper()
	var id int64
	var public uuid.UUID
	f.scan(`INSERT INTO profiles(account_id) VALUES ($1) RETURNING id,uuid`, []any{account}, &id, &public)
	f.exec(`INSERT INTO profile_translations(profile_id,locale,first_name,last_name,is_default) VALUES ($1,'tr',$2,$3,true)`, id, first, last)
	if member != nil {
		f.exec(`INSERT INTO company_member_profiles(account_id,member_id,profile_id) SELECT $1,id,$3 FROM account_members WHERE account_id=$1 AND user_id=$2`, account, member.UserInternal, id)
	}
	return public
}

func (f *fixture) card(account int64, profile uuid.UUID) (uuid.UUID, uuid.UUID) {
	f.t.Helper()
	var id int64
	var card, route uuid.UUID
	f.scan(`INSERT INTO nfc_cards(account_id,user_id,name,smart_code,profile_id,status) VALUES ($1,(SELECT user_id FROM account_members WHERE account_id=$1 AND role='owner' LIMIT 1),'Card',$2,(SELECT id FROM profiles WHERE uuid=$3),'active') RETURNING id,uuid`,
		[]any{account, "cc-" + pgtest.Unique(), profile}, &id, &card)
	f.scan(`INSERT INTO routes(account_id,card_id,path,route_type,status,is_immutable,profile_id) SELECT account_id,id,'c/'||smart_code,'card_route','active',true,profile_id FROM nfc_cards WHERE id=$1 RETURNING uuid`,
		[]any{id}, &route)
	return card, route
}

func (f *fixture) route(account int64, routeType string, profile *uuid.UUID) uuid.UUID {
	f.t.Helper()
	var id uuid.UUID
	target := "direct_url"
	if profile != nil {
		target = "profile"
	}
	f.scan(`INSERT INTO routes(account_id,path,route_type,status,target_type,direct_url,profile_id) VALUES ($1,$2,$3,'active',$4,'https://example.test',(SELECT id FROM profiles WHERE uuid=$5)) RETURNING uuid`,
		[]any{account, "l-" + pgtest.Unique(), routeType, target, profile}, &id)
	return id
}

func (f *fixture) agg(day, subjectType string, subject uuid.UUID, eventType string, n int64, countries map[string]int64) {
	f.t.Helper()
	if countries == nil {
		countries = map[string]int64{}
	}
	b, err := json.Marshal(countries)
	require.NoError(f.t, err)
	f.exec(`INSERT INTO event_aggregates_daily(bucket_date,subject_type,subject_id,event_type,total_count,country_breakdown) VALUES ($1,$2,$3,$4,$5,$6)`,
		day, subjectType, subject.String(), eventType, n, b)
}

func (f *fixture) lead(account int64, at string, profile, card *uuid.UUID) {
	f.t.Helper()
	f.exec(`INSERT INTO leads(account_id,profile_id,card_id,source_type,route_path,name,email,consent_at,consent_version,created_at)
		VALUES ($1,(SELECT id FROM profiles WHERE uuid=$2),(SELECT id FROM nfc_cards WHERE uuid=$3),'profile','x','Lead','lead@example.test',now(),'v1',$4)`,
		account, profile, card, at)
}

// seed writes the shared event set: 2026-10-10 and 2026-10-12 inside the default range,
// 2026-08-01 outside it.
func (f *fixture) seed() {
	f.agg("2026-10-10", "profile", f.p1, "card.view", 10, map[string]int64{"TR": 7, "DE": 3})
	f.agg("2026-10-12", "profile", f.p1, "card.view", 5, map[string]int64{"TR": 5})
	f.agg("2026-10-10", "profile", f.p1, "vcard.download", 2, nil)
	f.agg("2026-10-10", "profile", f.p1, "contact.phone.click", 3, nil)
	f.agg("2026-10-10", "route", f.cardRoute1, "route.scan", 4, map[string]int64{"TR": 4})
	f.agg("2026-10-10", "card", f.card1, "card.qr.scan", 1, nil)
	f.agg("2026-10-12", "route", f.p1Link, "route.scan", 6, nil)
	f.agg("2026-10-10", "profile", f.p2, "card.view", 8, map[string]int64{"US": 8})
	f.agg("2026-10-10", "profile", f.p2, "social.link.click", 1, nil)
	f.agg("2026-10-12", "profile", f.p3, "card.view", 1, nil)
	f.agg("2026-10-12", "route", f.smartLink, "route.scan", 9, map[string]int64{"FR": 9})
	f.agg("2026-08-01", "profile", f.p1, "card.view", 100, nil)
	// Another company: never counted, not even its route that targets our profile.
	f.agg("2026-10-10", "profile", f.otherProfile, "card.view", 1000, map[string]int64{"TR": 1000})
	f.agg("2026-10-10", "route", f.otherRoute, "route.scan", 500, nil)

	f.lead(f.account, "2026-10-10T09:00:00Z", &f.p1, nil)
	f.lead(f.account, "2026-10-12T09:00:00Z", nil, &f.card1)
	f.lead(f.account, "2026-08-01T09:00:00Z", &f.p1, nil)
	var otherAccount int64
	f.scan(`SELECT account_id FROM profiles WHERE uuid=$1`, []any{f.otherProfile}, &otherAccount)
	f.lead(otherAccount, "2026-10-10T09:00:00Z", &f.otherProfile, nil)
}

func defaultFilter() model.Filter {
	return model.Filter{From: today.AddDate(0, 0, -29), To: today, Sort: "-total_events", Limit: 20}
}

func byProfile(items []model.Employee) map[uuid.UUID]model.Employee {
	out := map[uuid.UUID]model.Employee{}
	for _, e := range items {
		out[e.ProfileUUID] = e
	}
	return out
}

func TestOwnerTotalsAndAttribution(t *testing.T) {
	f := setup(t)
	f.seed()
	s, err := f.svc.Summary(f.ctx, f.owner, defaultFilter())
	require.NoError(t, err)
	require.Equal(t, model.Metrics{
		TotalEvents: 10 + 5 + 2 + 3 + 4 + 1 + 6 + 8 + 1 + 1 + 9, ProfileViews: 10 + 5 + 8 + 1,
		CardTaps: 4, QRScans: 1, ContactSaves: 2, LinkClicks: 6 + 9, ContactClicks: 3 + 1, Leads: 2,
	}, s.Totals)
	require.Equal(t, "2026-09-20", s.From)
	require.Equal(t, "2026-10-19", s.To)
	require.Nil(t, s.HistoryDays)
	require.EqualValues(t, 3, s.Profiles)
	require.EqualValues(t, 3, s.ActiveProfiles)
	require.Equal(t, model.EventCount{EventType: "card.view", Count: 24}, s.TopEvents[0])

	items, total, err := f.svc.Employees(f.ctx, f.owner, defaultFilter())
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	rows := byProfile(items)
	p1 := rows[f.p1]
	// p1 gets its profile page, its card (route + QR), the smart link targeting it, and both leads.
	require.Equal(t, model.Metrics{TotalEvents: 31, ProfileViews: 15, CardTaps: 4, QRScans: 1, ContactSaves: 2, LinkClicks: 6, ContactClicks: 3, Leads: 2}, p1.Metrics)
	require.Equal(t, "Ayla Kaya", p1.ProfileName)
	require.Equal(t, "East", p1.OfficeName)
	require.Equal(t, "Sales", p1.GroupName)
	require.NotNil(t, p1.MemberUUID)
	require.InDelta(t, 31.0/50.0*100, p1.SharePercent, 0.0001)
	require.EqualValues(t, 9, rows[f.p2].Metrics.TotalEvents)
	require.Equal(t, "West", rows[f.p2].OfficeName)
	require.Nil(t, rows[f.p3].MemberUUID, "unlinked profiles are compared too")
	require.Equal(t, f.p1, items[0].ProfileUUID, "default sort is -total_events")
}

func TestAccountIsolation(t *testing.T) {
	f := setup(t)
	f.seed()
	// A principal of A cannot reach B by passing B's id with a role it does not hold there.
	stranger := f.owner
	var otherAccount int64
	f.scan(`SELECT account_id FROM profiles WHERE uuid=$1`, []any{f.otherProfile}, &otherAccount)
	stranger.Account.ID = otherAccount
	_, err := f.svc.Summary(f.ctx, stranger, defaultFilter())
	require.ErrorIs(t, err, model.ErrForbidden)

	b, err := f.svc.Breakdown(f.ctx, f.owner, model.Filter{From: today.AddDate(0, 0, -29), To: today, Dimension: "country", TopN: 10})
	require.NoError(t, err)
	require.EqualValues(t, 7+5+4+8+3+9, b.Total, "other company's TR=1000 is excluded")
	require.Equal(t, "TR", b.Items[0].Key)
	require.EqualValues(t, 16, b.Items[0].Count)
	require.InDelta(t, 16.0/36.0*100, b.Items[0].Percentage, 0.0001)

	limited, err := f.svc.Breakdown(f.ctx, f.owner, model.Filter{From: today.AddDate(0, 0, -29), To: today, Dimension: "country", TopN: 2})
	require.NoError(t, err)
	require.Len(t, limited.Items, 2)
	require.EqualValues(t, 36, limited.Total, "the total is over every value, not only the top N")

	views, err := f.svc.Breakdown(f.ctx, f.owner, model.Filter{From: today.AddDate(0, 0, -29), To: today, Dimension: "country", EventType: "route.scan", TopN: 10})
	require.NoError(t, err)
	require.EqualValues(t, 4+9, views.Total)
}

func TestRoleScope(t *testing.T) {
	f := setup(t)
	f.seed()

	admin, err := f.svc.Summary(f.ctx, f.admin, defaultFilter())
	require.NoError(t, err)
	require.EqualValues(t, 50, admin.Totals.TotalEvents)

	// The office manager sees East only: p1, not p2, the unlinked p3 or the URL smart link.
	s, err := f.svc.Summary(f.ctx, f.manager, defaultFilter())
	require.NoError(t, err)
	require.EqualValues(t, 31, s.Totals.TotalEvents)
	require.EqualValues(t, 2, s.Totals.Leads)
	require.EqualValues(t, 1, s.Profiles)
	items, total, err := f.svc.Employees(f.ctx, f.manager, defaultFilter())
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, f.p1, items[0].ProfileUUID)
	require.InDelta(t, 100, items[0].SharePercent, 0.0001)
	west := defaultFilter()
	west.OfficeUUID = &f.west
	items, total, err = f.svc.Employees(f.ctx, f.manager, west)
	require.NoError(t, err)
	require.Zero(t, total, "an office filter cannot widen the grant")
	require.Empty(t, items)

	idle, err := f.svc.Summary(f.ctx, f.idleManager, defaultFilter())
	require.NoError(t, err)
	require.Zero(t, idle.Totals.TotalEvents, "an office manager without grants sees nothing")

	_, err = f.svc.Summary(f.ctx, f.member, defaultFilter())
	require.ErrorIs(t, err, model.ErrForbidden)
	_, _, err = f.svc.Employees(f.ctx, f.member, defaultFilter())
	require.ErrorIs(t, err, model.ErrForbidden)

	noPerm := f.owner
	noPerm.Permissions = nil
	_, err = f.svc.Summary(f.ctx, noPerm, defaultFilter())
	require.ErrorIs(t, err, model.ErrForbidden)

	u := f.d.CreateUser(t)
	personal := authctx.Principal{UserID: u.UUID, UserInternal: u.ID,
		Account:     authctx.Account{ID: u.AccountID, Type: authctx.AccountTypePersonal, Role: "owner"},
		Permissions: []string{rbac.PermAppCompanyAnalyticsRead}}
	_, err = f.svc.Summary(f.ctx, personal, defaultFilter())
	require.ErrorIs(t, err, model.ErrForbidden)
	personal.Account.Type = authctx.AccountTypeCompany // a forged type still fails on the DB check
	_, err = f.svc.Summary(f.ctx, personal, defaultFilter())
	require.ErrorIs(t, err, model.ErrForbidden)

	// Demotion takes effect at once: the role is reloaded from the database.
	f.exec(`UPDATE account_members SET role='member' WHERE account_id=$1 AND user_id=$2`, f.account, f.admin.UserInternal)
	_, err = f.svc.Summary(f.ctx, f.admin, defaultFilter())
	require.ErrorIs(t, err, model.ErrForbidden)
}

func TestOfficeAndGroupFilters(t *testing.T) {
	f := setup(t)
	f.seed()
	east := defaultFilter()
	east.OfficeUUID = &f.east
	s, err := f.svc.Summary(f.ctx, f.owner, east)
	require.NoError(t, err)
	require.EqualValues(t, 31, s.Totals.TotalEvents, "a filter drops unattributed subjects")
	group := defaultFilter()
	group.GroupUUID = &f.sales
	items, total, err := f.svc.Employees(f.ctx, f.owner, group)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, f.p1, items[0].ProfileUUID)
	west := defaultFilter()
	west.OfficeUUID = &f.west
	s, err = f.svc.Summary(f.ctx, f.owner, west)
	require.NoError(t, err)
	require.EqualValues(t, 9, s.Totals.TotalEvents)
}

func TestDateRangeHistoryAndSeries(t *testing.T) {
	f := setup(t)
	f.seed()
	wide := model.Filter{From: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), To: today, Sort: "-total_events", Limit: 20}
	s, err := f.svc.Summary(f.ctx, f.owner, wide)
	require.NoError(t, err)
	require.EqualValues(t, 150, s.Totals.TotalEvents, "the August event is inside a wide range")
	require.EqualValues(t, 3, s.Totals.Leads)

	single := model.Filter{From: time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)}
	s, err = f.svc.Summary(f.ctx, f.owner, single)
	require.NoError(t, err)
	require.EqualValues(t, 5+6+1+9, s.Totals.TotalEvents, "from and to are inclusive days")
	require.EqualValues(t, 1, s.Totals.Leads)

	// The plan's history window clamps from, as in per-subject analytics.
	f.svc = usecase.New(repository.New(f.d.Pool), history{days: 30})
	f.svc.SetNow(func() time.Time { return today.Add(15 * time.Hour) })
	s, err = f.svc.Summary(f.ctx, f.owner, wide)
	require.NoError(t, err)
	require.Equal(t, "2026-09-19", s.From)
	require.EqualValues(t, 30, *s.HistoryDays)
	require.EqualValues(t, 50, s.Totals.TotalEvents)

	ts, err := f.svc.TimeSeries(f.ctx, f.owner, defaultFilter())
	require.NoError(t, err)
	require.Len(t, ts.Points, 30, "zero-filled, one point per day")
	byDate := map[string]model.Point{}
	var sum int64
	for _, p := range ts.Points {
		byDate[p.Date] = p
		sum += p.TotalEvents
	}
	require.EqualValues(t, 50, sum)
	require.EqualValues(t, 10+8, byDate["2026-10-10"].ProfileViews)
	require.EqualValues(t, 1, byDate["2026-10-10"].Leads)
	require.EqualValues(t, 15, byDate["2026-10-12"].LinkClicks)
	require.Zero(t, byDate["2026-10-11"].TotalEvents)

	mgr, err := f.svc.TimeSeries(f.ctx, f.manager, defaultFilter())
	require.NoError(t, err)
	sum = 0
	for _, p := range mgr.Points {
		sum += p.TotalEvents
	}
	require.EqualValues(t, 31, sum)
}

func TestEmployeesSortSearchPaging(t *testing.T) {
	f := setup(t)
	f.seed()
	fl := defaultFilter()
	fl.Sort = "name"
	items, _, err := f.svc.Employees(f.ctx, f.owner, fl)
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{f.p1, f.p2, f.p3}, []uuid.UUID{items[0].ProfileUUID, items[1].ProfileUUID, items[2].ProfileUUID})
	fl.Sort = "-profile_views"
	fl.Limit, fl.Offset = 1, 1
	items, total, err := f.svc.Employees(f.ctx, f.owner, fl)
	require.NoError(t, err)
	require.EqualValues(t, 3, total)
	require.Len(t, items, 1)
	require.Equal(t, f.p2, items[0].ProfileUUID)
	fl = defaultFilter()
	fl.Q = "burak"
	items, total, err = f.svc.Employees(f.ctx, f.owner, fl)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, f.p2, items[0].ProfileUUID)
}

func TestHandlerStatusAndValidation(t *testing.T) {
	f := setup(t)
	f.seed()
	h := handler.New(f.svc)
	call := func(p authctx.Principal, fn http.HandlerFunc, target string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, target, nil)
		r = r.WithContext(authctx.WithPrincipal(r.Context(), p))
		w := httptest.NewRecorder()
		fn(w, r)
		return w
	}
	require.Equal(t, http.StatusOK, call(f.owner, h.Summary, "/v1/company/analytics/summary?from=2026-10-01&to=2026-10-19").Code)
	require.Equal(t, http.StatusOK, call(f.manager, h.Employees, "/v1/company/analytics/employees?sort=-leads").Code)
	require.Equal(t, http.StatusOK, call(f.owner, h.Breakdown, "/v1/company/analytics/breakdown?dimension=city&limit=5").Code)
	require.Equal(t, http.StatusOK, call(f.owner, h.TimeSeries, "/v1/company/analytics/timeseries").Code)
	require.Equal(t, http.StatusForbidden, call(f.member, h.Summary, "/v1/company/analytics/summary").Code)
	for _, tc := range []struct {
		fn     http.HandlerFunc
		target string
	}{
		{h.Summary, "/v1/company/analytics/summary?from=yesterday"},
		{h.Summary, "/v1/company/analytics/summary?from=2026-10-19&to=2026-10-01"},
		{h.Summary, "/v1/company/analytics/summary?from=2024-01-01&to=2026-10-19"},
		{h.TimeSeries, "/v1/company/analytics/timeseries?office_uuid=nope"},
		{h.Employees, "/v1/company/analytics/employees?sort=password"},
		{h.Employees, "/v1/company/analytics/employees?page=2"},
		{h.Breakdown, "/v1/company/analytics/breakdown?dimension=ip"},
		{h.Breakdown, "/v1/company/analytics/breakdown?limit=500"},
	} {
		require.Equal(t, http.StatusBadRequest, call(f.owner, tc.fn, tc.target).Code, tc.target)
	}

	f.svc = usecase.New(repository.New(f.d.Pool), history{days: 30})
	f.svc.SetNow(func() time.Time { return today })
	w := call(f.owner, handler.New(f.svc).Summary, "/v1/company/analytics/summary")
	require.Equal(t, "30", w.Header().Get(handler.HistoryHeader))
}
