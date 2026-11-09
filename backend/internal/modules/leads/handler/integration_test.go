package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/leads/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/leads/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/leads/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/leads/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/providers"
	notifusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
	ioadapters "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/ioengine/adapters"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/mail"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) { pgtest.Main(m) }

type fixture struct {
	t   *testing.T
	ctx context.Context
	d   *pgtest.DB
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

func (f *fixture) member(account int64, role string) authctx.Principal {
	f.t.Helper()
	u := f.d.CreateUser(f.t)
	require.NoError(f.t, f.d.Queries.AddAccountMember(f.ctx, db.AddAccountMemberParams{AccountID: account, UserID: u.ID, Role: db.AccountRole(role)}))
	return authctx.Principal{UserID: u.UUID, UserInternal: u.ID, Email: "visitor-" + pgtest.Unique() + "@example.test",
		Account: authctx.Account{ID: account, Type: authctx.AccountTypeCompany, Role: role}}
}

// profile creates a profile of the account, linked as the member's employee profile when given.
func (f *fixture) profile(account int64, first string, member *authctx.Principal) int64 {
	f.t.Helper()
	var id int64
	f.scan(`INSERT INTO profiles(account_id) VALUES ($1) RETURNING id`, []any{account}, &id)
	f.exec(`INSERT INTO profile_translations(profile_id,locale,first_name,last_name,is_default) VALUES ($1,'tr',$2,'Test',true)`, id, first)
	if member != nil {
		f.exec(`INSERT INTO company_member_profiles(account_id,member_id,profile_id) SELECT $1,id,$3 FROM account_members WHERE account_id=$1 AND user_id=$2`,
			account, member.UserInternal, id)
	}
	return id
}

// card creates an active card (lead form off) with its card route; returns the card id and path.
func (f *fixture) card(account, profile int64) (int64, string) {
	f.t.Helper()
	var id int64
	var path string
	f.scan(`INSERT INTO nfc_cards(account_id,user_id,name,smart_code,profile_id,status)
		VALUES ($1,(SELECT user_id FROM account_members WHERE account_id=$1 AND role='owner' LIMIT 1),'Card',$2,$3,'active') RETURNING id`,
		[]any{account, "cc-" + pgtest.Unique(), profile}, &id)
	f.scan(`INSERT INTO routes(account_id,card_id,path,route_type,status,is_immutable,profile_id)
		SELECT account_id,id,'c/'||smart_code,'card_route','active',true,profile_id FROM nfc_cards WHERE id=$1 RETURNING path`, []any{id}, &path)
	return id, path
}

func (f *fixture) lead(account, profile int64, card *int64) uuid.UUID {
	f.t.Helper()
	var id uuid.UUID
	f.scan(`INSERT INTO leads(account_id,profile_id,card_id,source_type,route_path,name,email,consent_at,consent_version,kind)
		VALUES ($1,$2,$3,'profile','x','Visitor','v@example.test',NOW(),'kvkk-lead-v1','exchange') RETURNING uuid`,
		[]any{account, profile, card}, &id)
	return id
}

func call(h http.HandlerFunc, method, target string, p *authctx.Principal, body any, pathUUID string) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	r := httptest.NewRequest(method, target, &buf)
	if pathUUID != "" {
		r.SetPathValue("uuid", pathUUID)
	}
	if p != nil {
		r = r.WithContext(authctx.WithPrincipal(r.Context(), *p))
	}
	w := httptest.NewRecorder()
	h(w, r)
	return w
}

func listTotal(t *testing.T, w *httptest.ResponseRecorder) int {
	t.Helper()
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var out struct {
		Data struct {
			Total int `json:"total"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	return out.Data.Total
}

// Acceptance (TEC-940): owner/admin read every lead of the account, a member only the leads of
// their own profile or card; another member's lead answers 404. Export follows the same rule.
func TestIntegrationLeadReadScope(t *testing.T) {
	d := pgtest.Open(t)
	f := &fixture{t: t, ctx: context.Background(), d: d}
	company, err := d.Queries.CreateCompanyAccount(f.ctx, db.CreateCompanyAccountParams{Name: "Leads Co", Slug: "leads-" + pgtest.Unique()})
	require.NoError(t, err)
	owner := f.member(company.ID, "owner")
	admin := f.member(company.ID, "admin")
	m1 := f.member(company.ID, "member")
	m2 := f.member(company.ID, "member")
	manager := f.member(company.ID, "office_manager")
	other, err := d.Queries.CreateCompanyAccount(f.ctx, db.CreateCompanyAccountParams{Name: "Other Co", Slug: "other-" + pgtest.Unique()})
	require.NoError(t, err)
	outsider := f.member(other.ID, "owner")

	p1, p2, unlinked := f.profile(company.ID, "Bir", &m1), f.profile(company.ID, "Iki", &m2), f.profile(company.ID, "Bos", nil)
	m1Card, _ := f.card(company.ID, p1)
	l1 := f.lead(company.ID, p1, nil)
	l2 := f.lead(company.ID, p2, nil)
	// Came through m1's card while it still showed another profile: the card is m1's now.
	lCard := f.lead(company.ID, unlinked, &m1Card)
	lNone := f.lead(company.ID, unlinked, nil)

	h := handler.New(usecase.New(repository.NewPostgres(d.Queries), nil, nil).SetQuotas(entitlement.New(d.Queries)), nil)
	get := func(p authctx.Principal, id uuid.UUID) int {
		return call(h.Get, http.MethodGet, "/v1/leads/"+id.String(), &p, nil, id.String()).Code
	}
	list := func(p authctx.Principal) int {
		return listTotal(t, call(h.List, http.MethodGet, "/v1/leads", &p, nil, ""))
	}

	require.Equal(t, http.StatusNotFound, get(m2, l1), "member read another member's lead")
	require.Equal(t, http.StatusNotFound, get(m1, l2), "member read another member's lead")
	require.Equal(t, http.StatusNotFound, get(m1, lNone), "member read an unassigned lead")
	require.Equal(t, http.StatusOK, get(m1, l1))
	require.Equal(t, http.StatusOK, get(m1, lCard))
	require.Equal(t, http.StatusOK, get(m2, l2))
	for _, p := range []authctx.Principal{owner, admin} {
		for _, id := range []uuid.UUID{l1, l2, lCard, lNone} {
			require.Equal(t, http.StatusOK, get(p, id))
		}
		require.Equal(t, 4, list(p))
	}
	require.Equal(t, 2, list(m1))
	require.Equal(t, 1, list(m2))
	require.Equal(t, 0, list(manager), "office managers get no account-wide lead access")

	// A principal whose token claims another account is scoped by that account and its membership.
	forged := outsider
	forged.Account.ID = company.ID
	forged.Account.Role = "owner"
	require.Equal(t, http.StatusNotFound, get(forged, l1), "non-member read a lead")
	require.Equal(t, 0, list(forged))

	adapter := ioadapters.NewLeads(d.Queries)
	export := func(p authctx.Principal) int {
		ds, err := adapter.Export(f.ctx, map[string]string{
			"account_id": strconvI(company.ID), "viewer_user_id": strconvI(p.UserInternal),
		}, "tr")
		require.NoError(t, err)
		return len(ds.Rows)
	}
	require.Equal(t, 4, export(owner))
	require.Equal(t, 2, export(m1))
	require.Equal(t, 1, export(m2))
	_, err = adapter.Export(f.ctx, map[string]string{"account_id": strconvI(company.ID)}, "tr")
	require.Error(t, err, "export without a viewer must fail closed")
}

type capturedMail struct {
	mu   sync.Mutex
	msgs []mail.Message
}

func (c *capturedMail) Send(_ context.Context, m mail.Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.msgs = append(c.msgs, m)
	return nil
}

type fakeVCards struct{}

func (fakeVCards) PublicVCard(_ context.Context, username, _ string) (string, string, string, error) {
	return "BEGIN:VCARD\r\nFN:" + username + "\r\nEND:VCARD\r\n", username + ".vcf", "", nil
}

// Acceptance (TEC-940): an exchange through a card whose lead form is off is the card's; the
// visitor gets a copy with the owner's vCard; its delete link deletes the exchange (and the copy)
// exactly once.
func TestIntegrationExchangeCopyAndDeleteLink(t *testing.T) {
	d := pgtest.Open(t)
	f := &fixture{t: t, ctx: context.Background(), d: d}
	owner := d.CreateUser(t)
	var profile int64
	f.scan(`INSERT INTO profiles(account_id, user_id) VALUES ($1,$2) RETURNING id`, []any{owner.AccountID, owner.ID}, &profile)
	f.exec(`INSERT INTO profile_translations(profile_id,locale,first_name,last_name,is_default) VALUES ($1,'tr','Ahmet','Kaya',true)`, profile)
	username := "ex-" + pgtest.Unique()
	f.exec(`INSERT INTO routes (path, route_type, status, target_type, profile_id, account_id, owner_user_id, presentation_config)
		VALUES ($1, 'canonical_profile', 'active', 'profile', $2, $3, $4, '{"contact_exchange":{"enabled":true}}')`,
		username, profile, owner.AccountID, owner.ID)
	cardID, cardPath := f.card(owner.AccountID, profile)

	sent := &capturedMail{}
	files := &usecase.CopyAttachments{VCards: fakeVCards{}}
	notif := notifusecase.New(d.Queries, nil, []providers.Provider{providers.EmailProvider{Mail: sent, Attachments: files}}, nil)
	svc := usecase.New(repository.NewPostgres(d.Queries), notif, nil).WithVisitorCopy("integration-secret", "https://comcode.test").
		SetQuotas(entitlement.New(d.Queries))
	h := handler.New(svc, nil)

	w := call(h.Exchange, http.MethodPost, "/v1/public/exchanges", nil, model.ExchangeInput{
		Path: username, Via: cardPath, Name: "Ayşe Yılmaz", Email: "ayse-" + pgtest.Unique() + "@example.test",
		Company: "Acme", Locale: "tr", Consent: true, ConsentVersion: model.ExchangeConsentVersion,
	}, "")
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var created struct {
		Data model.SubmitResult `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	lead := created.Data.UUID

	var source string
	var gotCard *int64
	f.scan(`SELECT source_type, card_id FROM leads WHERE uuid=$1`, []any{lead}, &source, &gotCard)
	require.Equal(t, model.SourceCard, source, "card with its form off must still attribute")
	require.NotNil(t, gotCard)
	require.Equal(t, cardID, *gotCard)

	require.Len(t, sent.msgs, 1)
	msg := sent.msgs[0]
	require.Contains(t, msg.Subject, "Ahmet Kaya")
	require.Contains(t, msg.Body, "Şirket: Acme")
	require.Len(t, msg.Attachments, 1)
	require.Contains(t, string(msg.Attachments[0].Data), "FN:"+username)
	var copies int
	f.scan(`SELECT COUNT(*) FROM notifications WHERE source_event=$1 AND status='sent'`, []any{model.ExchangeCopyEvent(lead)}, &copies)
	require.Equal(t, 1, copies)

	i := strings.Index(msg.Body, "https://comcode.test/kvkk/exchange?token=")
	require.GreaterOrEqual(t, i, 0, msg.Body)
	link, err := url.Parse(strings.Fields(msg.Body[i:])[0])
	require.NoError(t, err)
	token := link.Query().Get("token")

	del := func(tok string) int {
		return call(h.DeleteExchange, http.MethodPost, "/v1/public/exchanges/delete", nil, model.DeleteExchangeInput{Token: tok}, "").Code
	}
	require.Equal(t, http.StatusBadRequest, del(token+"x"))
	require.Equal(t, http.StatusOK, del(token))
	var left int
	f.scan(`SELECT (SELECT COUNT(*) FROM leads WHERE uuid=$1) + (SELECT COUNT(*) FROM notifications WHERE source_event=$2)`,
		[]any{lead, model.ExchangeCopyEvent(lead)}, &left)
	require.Zero(t, left, "exchange or its copy survived")
	require.Equal(t, http.StatusNotFound, del(token), "the link must work once")
}

func strconvI(v int64) string { return strconv.FormatInt(v, 10) }
