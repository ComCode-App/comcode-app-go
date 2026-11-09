package companystructure_test

// API-level role visibility (TEC-948) against real PostgreSQL, enabled by TEST_DATABASE_URL.
import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companystructure"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companystructure/handler"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companystructure/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companystructure/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companystructure/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/jwt"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
	"github.com/google/uuid"
)

func TestMain(m *testing.M) { pgtest.Main(m) }

// identities resolves the bearer's subject to a fixture principal.
type identities map[uuid.UUID]authctx.Principal

func (i identities) LoadPrincipal(_ *http.Request, c jwt.Claims) (authctx.Principal, error) {
	return i[uuid.MustParse(c.Subject)], nil
}

type api struct {
	t      *testing.T
	mux    *http.ServeMux
	tokens *jwt.Manager
}

func (a api) do(user uuid.UUID, method, path, body string) (int, json.RawMessage) {
	a.t.Helper()
	bearer, _, err := a.tokens.IssueAccess(jwt.AccessInput{UserID: user})
	if err != nil {
		a.t.Fatal(err)
	}
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	a.mux.ServeHTTP(rec, req)
	var env struct {
		Data json.RawMessage `json:"data"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	return rec.Code, env.Data
}

// ids lists the uuid field of every item of a list endpoint.
func (a api) ids(user uuid.UUID, path, field string) []uuid.UUID {
	a.t.Helper()
	code, data := a.do(user, http.MethodGet, path, "")
	if code != http.StatusOK {
		a.t.Fatalf("GET %s: %d %s", path, code, data)
	}
	var page struct {
		Items []map[string]any `json:"items"`
		Total int              `json:"total"`
	}
	if err := json.Unmarshal(data, &page); err != nil {
		a.t.Fatal(err)
	}
	if page.Total != len(page.Items) {
		a.t.Fatalf("GET %s: total %d != %d items", path, page.Total, len(page.Items))
	}
	out := []uuid.UUID{}
	for _, v := range page.Items {
		out = append(out, uuid.MustParse(v[field].(string)))
	}
	return out
}

func sameSet(got, want []uuid.UUID) bool {
	if len(got) != len(want) {
		return false
	}
	for _, v := range want {
		if !slices.Contains(got, v) {
			return false
		}
	}
	return true
}

func TestIntegrationCompanyStructureVisibilityByRole(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	owner, admin, mgr, plain, other, pool := d.CreateUser(t), d.CreateUser(t), d.CreateUser(t), d.CreateUser(t), d.CreateUser(t), d.CreateUser(t)
	acc, err := d.Queries.CreateCompanyAccount(ctx, db.CreateCompanyAccountParams{Name: "Visibility", Slug: "visibility-" + pgtest.Unique()})
	if err != nil {
		t.Fatal(err)
	}
	roles := map[pgtest.User]db.AccountRole{
		owner: db.AccountRoleOwner, admin: db.AccountRoleAdmin, mgr: db.AccountRoleOfficeManager,
		plain: db.AccountRoleMember, other: db.AccountRoleMember, pool: db.AccountRoleMember,
	}
	who := identities{}
	for u, role := range roles {
		if err = d.Queries.AddAccountMember(ctx, db.AddAccountMemberParams{AccountID: acc.ID, UserID: u.ID, Role: role}); err != nil {
			t.Fatal(err)
		}
		// Every principal holds read+write so the account role alone decides visibility.
		who[u.UUID] = authctx.Principal{
			UserInternal: u.ID, UserID: u.UUID,
			Account:     authctx.Account{ID: acc.ID, UUID: acc.Uuid, Type: authctx.AccountTypeCompany, Role: string(role)},
			Permissions: []string{rbac.PermAppCompanyStructureRead, rbac.PermAppCompanyStructureWrite},
		}
	}
	svc := usecase.New(repository.New(d.Pool, d.Queries))
	tokens, err := jwt.NewManager("fixture-jwt-secret", time.Hour, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	companystructure.RegisterRoutes(mux, handler.New(svc, nil, nil), tokens, who)
	a := api{t: t, mux: mux, tokens: tokens}

	// Fixture: offices X, Y (managed by mgr), Z; one group per office plus a company-wide group.
	// plain sits in X/gx, other in Z, pool is unassigned; owner, admin and mgr have no office.
	op := who[owner.UUID]
	office := func(name string) uuid.UUID {
		v, err := svc.SaveOffice(ctx, op, uuid.Nil, model.OfficeInput{Name: name})
		if err != nil {
			t.Fatal(err)
		}
		return v.UUID
	}
	x, y, z := office("X"), office("Y"), office("Z")
	group := func(name string, off *uuid.UUID) uuid.UUID {
		v, err := svc.SaveGroup(ctx, op, uuid.Nil, model.GroupInput{Name: name, Kind: "team", OfficeUUID: off})
		if err != nil {
			t.Fatal(err)
		}
		return v.UUID
	}
	gx, gy, gz, global := group("GX", &x), group("GY", &y), group("GZ", &z), group("Global", nil)
	if err = svc.SetScopes(ctx, op, mgr.UUID, []uuid.UUID{x, y}); err != nil {
		t.Fatal(err)
	}
	for _, in := range []model.Assignment{{UserUUID: plain.UUID, OfficeUUID: &x, GroupUUID: &gx}, {UserUUID: other.UUID, OfficeUUID: &z, GroupUUID: &gz}} {
		if _, err = svc.SetAssignment(ctx, op, in); err != nil {
			t.Fatal(err)
		}
	}

	everyone := []uuid.UUID{owner.UUID, admin.UUID, mgr.UUID, plain.UUID, other.UUID, pool.UUID}
	cases := []struct {
		name                         string
		user                         pgtest.User
		offices, groups, assignments []uuid.UUID
	}{
		{"owner", owner, []uuid.UUID{x, y, z}, []uuid.UUID{gx, gy, gz, global}, everyone},
		{"admin", admin, []uuid.UUID{x, y, z}, []uuid.UUID{gx, gy, gz, global}, everyone},
		// Managed offices, their groups and members, the unassigned member pool and self.
		{"office manager", mgr, []uuid.UUID{x, y}, []uuid.UUID{gx, gy}, []uuid.UUID{mgr.UUID, plain.UUID, pool.UUID}},
		// Only the own office, group and placement.
		{"member", plain, []uuid.UUID{x}, []uuid.UUID{gx}, []uuid.UUID{plain.UUID}},
	}
	for _, c := range cases {
		if got := a.ids(c.user.UUID, "/v1/company/offices", "uuid"); !sameSet(got, c.offices) {
			t.Errorf("%s offices: %v, want %v", c.name, got, c.offices)
		}
		if got := a.ids(c.user.UUID, "/v1/company/groups", "uuid"); !sameSet(got, c.groups) {
			t.Errorf("%s groups: %v, want %v", c.name, got, c.groups)
		}
		if got := a.ids(c.user.UUID, "/v1/company/assignments", "user_uuid"); !sameSet(got, c.assignments) {
			t.Errorf("%s assignments: %v, want %v", c.name, got, c.assignments)
		}
		// Single-item reads follow the list rules; hidden items read as missing.
		for _, id := range []uuid.UUID{x, y, z} {
			want := http.StatusNotFound
			if slices.Contains(c.offices, id) {
				want = http.StatusOK
			}
			if code, _ := a.do(c.user.UUID, http.MethodGet, "/v1/company/offices/"+id.String(), ""); code != want {
				t.Errorf("%s office %s: %d, want %d", c.name, id, code, want)
			}
		}
		for _, id := range []uuid.UUID{gx, gy, gz, global} {
			want := http.StatusNotFound
			if slices.Contains(c.groups, id) {
				want = http.StatusOK
			}
			if code, _ := a.do(c.user.UUID, http.MethodGet, "/v1/company/groups/"+id.String(), ""); code != want {
				t.Errorf("%s group %s: %d, want %d", c.name, id, code, want)
			}
		}
		for _, id := range everyone {
			want := http.StatusNotFound
			if slices.Contains(c.assignments, id) {
				want = http.StatusOK
			}
			if code, _ := a.do(c.user.UUID, http.MethodGet, "/v1/company/members/"+id.String()+"/assignment", ""); code != want {
				t.Errorf("%s assignment %s: %d, want %d", c.name, id, code, want)
			}
		}
		// Manager grants: owners/admins read anyone's, everyone else only their own.
		for _, id := range []uuid.UUID{mgr.UUID, c.user.UUID} {
			want := http.StatusOK
			if id != c.user.UUID && c.user != owner && c.user != admin {
				want = http.StatusForbidden
			}
			if code, _ := a.do(c.user.UUID, http.MethodGet, "/v1/company/members/"+id.String()+"/scopes", ""); code != want {
				t.Errorf("%s scopes of %s: %d, want %d", c.name, id, code, want)
			}
		}
	}

	// Office managers claim unassigned members into managed offices only, and never touch others.
	put := func(user pgtest.User, target uuid.UUID, body string) int {
		code, _ := a.do(user.UUID, http.MethodPut, "/v1/company/members/"+target.String()+"/assignment", body)
		return code
	}
	officeBody := func(id uuid.UUID) string { return fmt.Sprintf(`{"office_uuid":%q}`, id) }
	for name, c := range map[string]struct {
		code int
		want int
	}{
		"member claims pool":              {put(plain, pool.UUID, officeBody(x)), http.StatusForbidden},
		"manager claims into foreign":     {put(mgr, pool.UUID, officeBody(z)), http.StatusForbidden},
		"manager moves foreign member":    {put(mgr, other.UUID, officeBody(x)), http.StatusForbidden},
		"manager claims unassigned admin": {put(mgr, admin.UUID, officeBody(x)), http.StatusForbidden},
	} {
		if c.code != c.want {
			t.Errorf("%s: %d, want %d", name, c.code, c.want)
		}
	}
	if code := put(mgr, pool.UUID, fmt.Sprintf(`{"office_uuid":%q,"group_uuid":%q}`, y, gy)); code != http.StatusOK {
		t.Fatalf("manager claims pool member: %d", code)
	}
	if got := a.ids(mgr.UUID, "/v1/company/assignments", "user_uuid"); !sameSet(got, []uuid.UUID{mgr.UUID, plain.UUID, pool.UUID}) {
		t.Errorf("claimed member must stay visible to the manager: %v", got)
	}

	// Every placement change, by owners and office managers alike, lands in the activity log.
	var n int
	var payload []byte
	if err = d.Pool.QueryRow(ctx, `SELECT count(*) FROM activity_events WHERE action=$1 AND resource_uuid=ANY($2)`,
		usecase.AssignmentActivityAction, []uuid.UUID{plain.UUID, other.UUID, pool.UUID}).Scan(&n); err != nil || n != 3 {
		t.Fatalf("assignment activity count: %d %v", n, err)
	}
	if err = d.Pool.QueryRow(ctx, `SELECT payload FROM activity_events WHERE action=$1 AND resource_uuid=$2 AND actor_user_id=$3`,
		usecase.AssignmentActivityAction, pool.UUID, mgr.ID).Scan(&payload); err != nil {
		t.Fatalf("manager claim not logged: %v", err)
	}
	var logged map[string]any
	if err = json.Unmarshal(payload, &logged); err != nil || logged["from_office_uuid"] != nil || logged["office_uuid"] != y.String() ||
		logged["group_uuid"] != gy.String() || logged["account_uuid"] != acc.Uuid.String() {
		t.Fatalf("claim payload: %s %v", payload, err)
	}
}
