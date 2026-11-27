package usecase_test

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companybranding/model"
	pmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/model"
	profilesrepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/repository"
	profilesusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
	"github.com/google/uuid"
)

func personal(u pgtest.User) authctx.Principal {
	return authctx.Principal{
		UserID: u.UUID, UserInternal: u.ID,
		Permissions: []string{rbac.PermAppProfilesRead, rbac.PermAppProfilesWrite},
		Account:     authctx.Account{ID: u.AccountID, Type: authctx.AccountTypePersonal, Role: authctx.AccountRoleOwner},
	}
}

// TEC-1326: a person's open profiles (personal account + their employee profile) are shown
// together; a closed one is replaced by the first open one and never leaks; a company can keep
// employee profiles open; other people's profiles never join the group.
func TestIntegrationProfileSwipeGroup(t *testing.T) {
	e := newEnv(t)
	repo := profilesrepo.NewPostgres(e.d.Pool, e.d.Queries)
	profiles := profilesusecase.NewService(repo, nil)
	profiles.SetQuotas(allowQuotas{})
	profiles.SetPublicPages(repo)
	profiles.SetProfileGroups(repo)
	profiles.SetAccountThemes(e.svc)

	acme, acmeOwner := e.company("Acme")
	person, stranger, colleague := e.d.CreateUser(t), e.d.CreateUser(t), e.d.CreateUser(t)
	e.member(acme, person, db.AccountRoleMember)
	e.member(acme, colleague, db.AccountRoleMember)
	owner := principal(acme, acmeOwner, authctx.AccountRoleOwner)
	asMember := principal(acme, person, authctx.AccountRoleMember)
	asColleague := principal(acme, colleague, authctx.AccountRoleMember)
	mine := personal(person)

	create := func(p authctx.Principal, first, company string) (uuid.UUID, string) {
		t.Helper()
		prof, err := profiles.Create(e.ctx, p, pmodel.UpsertProfileInput{PrimaryLocale: "tr", Translations: []pmodel.UpsertTranslationInput{
			{Locale: "tr", FirstName: first, LastName: "Swipe" + pgtest.Unique(), CompanyName: company, IsDefault: true},
		}})
		if err != nil {
			t.Fatal(err)
		}
		page, err := profiles.GetPublicPage(e.ctx, p, prof.UUID)
		if err != nil {
			t.Fatal(err)
		}
		return prof.UUID, page.Username
	}
	a, aName := create(mine, "Alpha", "Olex Films")
	b, bName := create(mine, "Bravo", "Swiss Garage")
	c, cName := create(owner, "Charlie", "Acme")
	_, dName := create(personal(stranger), "Delta", "")
	unlinked, _ := create(owner, "Echo", "")
	if _, err := e.d.Pool.Exec(e.ctx, `
		INSERT INTO company_member_profiles (account_id, member_id, profile_id)
		SELECT $1, am.id, p.id FROM account_members am, profiles p
		WHERE am.account_id = $1 AND am.user_id = $2 AND p.uuid = $3`, acme.ID, person.ID, c); err != nil {
		t.Fatal(err)
	}

	siblings := func(pp *pmodel.PublicProfile) []string {
		out := []string{}
		for _, s := range pp.Siblings {
			out = append(out, s.Username)
		}
		slices.Sort(out)
		return out
	}
	want := func(names ...string) []string { slices.Sort(names); return names }

	// All open: the three profiles of the person, never the stranger's.
	pp, err := profiles.PublicProfile(e.ctx, aName)
	if err != nil || pp.ProfileUUID != a {
		t.Fatalf("open profile: %+v %v", pp, err)
	}
	if got := siblings(pp); !slices.Equal(got, want(aName, bName, cName)) {
		t.Fatalf("siblings %v", got)
	}
	labels := map[string]string{}
	for _, s := range pp.Siblings {
		labels[s.Username] = s.ShortName
	}
	if labels[aName] != "Olex Films" || labels[bName] != "Swiss Garage" {
		t.Fatalf("short names %v", labels)
	}
	if pp, _ := profiles.PublicProfile(e.ctx, dName); len(pp.Siblings) != 0 {
		t.Fatalf("stranger has siblings: %+v", pp.Siblings)
	}

	// Close B: its link shows another open profile and none of B's data.
	closed := false
	if _, err := profiles.SetOpen(e.ctx, mine, b, pmodel.SetOpenInput{IsOpen: &closed}); err != nil {
		t.Fatal(err)
	}
	pp, err = profiles.PublicProfile(e.ctx, bName)
	if err != nil || pp.ProfileUUID == b || pp.NotShared {
		t.Fatalf("closed profile shown: %+v %v", pp, err)
	}
	raw, _ := json.Marshal(pp)
	if strings.Contains(string(raw), "Bravo") || strings.Contains(string(raw), b.String()) || strings.Contains(string(raw), "Swiss Garage") {
		t.Fatalf("closed profile leaks: %s", raw)
	}
	if got := siblings(pp); !slices.Equal(got, want(aName, cName)) {
		t.Fatalf("siblings without B: %v", got)
	}
	// The vCard of a closed profile's card is the shown profile's.
	vc, _, _, err := profiles.PublicVCard(e.ctx, bName, "tr")
	if err != nil || strings.Contains(vc, "Bravo") || (!strings.Contains(vc, "Alpha") && !strings.Contains(vc, "Charlie")) {
		t.Fatalf("closed vcard: %v %q", err, vc)
	}

	// The employee switches their own company profile; a colleague cannot, an owner can.
	if _, err := profiles.SetOpen(e.ctx, asColleague, c, pmodel.SetOpenInput{IsOpen: &closed}); !errors.Is(err, profilesusecase.ErrForbidden) {
		t.Fatalf("colleague closes: %v", err)
	}
	if _, err := profiles.SetOpen(e.ctx, asMember, unlinked, pmodel.SetOpenInput{IsOpen: &closed}); !errors.Is(err, profilesusecase.ErrForbidden) {
		t.Fatalf("member closes another profile: %v", err)
	}
	if st, err := profiles.SetOpen(e.ctx, asMember, c, pmodel.SetOpenInput{IsOpen: &closed}); err != nil || st.IsOpen || !st.CanToggleOpen {
		t.Fatalf("member closes own: %+v %v", st, err)
	}
	// Profiles of another account are not found.
	if _, err := profiles.SetOpen(e.ctx, mine, c, pmodel.SetOpenInput{IsOpen: &closed}); !errors.Is(err, profilesusecase.ErrNotFound) {
		t.Fatalf("cross-account toggle: %v", err)
	}
	list, _, err := profiles.List(e.ctx, asMember, 50, 0, "", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range list {
		if it.CanToggleOpen != (it.UUID == c) {
			t.Fatalf("member list can_toggle_open: %+v", it)
		}
	}
	pp, _ = profiles.PublicProfile(e.ctx, aName)
	if len(pp.Siblings) != 0 {
		t.Fatalf("single open profile has siblings: %+v", pp.Siblings)
	}

	// Company lock: employee profiles stay open and cannot be closed.
	alwaysOpen := true
	if _, err := e.svc.UpdateBranding(e.ctx, owner, model.UpdateBrandingInput{ProfilesAlwaysOpen: &alwaysOpen}); err != nil {
		t.Fatal(err)
	}
	if st, err := profiles.OpenState(e.ctx, asMember, c); err != nil || !st.IsOpen || !st.OpenLocked {
		t.Fatalf("locked state: %+v %v", st, err)
	}
	if _, err := profiles.SetOpen(e.ctx, asMember, c, pmodel.SetOpenInput{IsOpen: &closed}); !errors.Is(err, profilesusecase.ErrOpenLocked) {
		t.Fatalf("close while locked: %v", err)
	}
	pp, _ = profiles.PublicProfile(e.ctx, cName)
	if pp.ProfileUUID != c || !slices.Equal(siblings(pp), want(aName, cName)) {
		t.Fatalf("locked profile: %+v", pp)
	}
	// A branding save without the field keeps the lock.
	if _, err := e.svc.UpdateBranding(e.ctx, owner, model.UpdateBrandingInput{}); err != nil {
		t.Fatal(err)
	}
	if b, _ := e.svc.GetBranding(e.ctx, owner); !b.ProfilesAlwaysOpen {
		t.Fatal("lock dropped by a save without the field")
	}

	// Nothing open: "not shared", no data, no vCard.
	alwaysOpen = false
	if _, err := e.svc.UpdateBranding(e.ctx, owner, model.UpdateBrandingInput{ProfilesAlwaysOpen: &alwaysOpen}); err != nil {
		t.Fatal(err)
	}
	if _, err := profiles.SetOpen(e.ctx, mine, a, pmodel.SetOpenInput{IsOpen: &closed}); err != nil {
		t.Fatal(err)
	}
	pp, err = profiles.PublicProfile(e.ctx, aName)
	if err != nil || !pp.NotShared || pp.ProfileUUID != uuid.Nil || len(pp.Translations) != 0 || len(pp.Siblings) != 0 {
		t.Fatalf("nothing open: %+v %v", pp, err)
	}
	if _, _, _, err := profiles.PublicVCard(e.ctx, aName, "tr"); !errors.Is(err, profilesusecase.ErrNotFound) {
		t.Fatalf("vcard with nothing open: %v", err)
	}
	// A company profile without a linked member is shown on its own while open, hidden when closed.
	if _, err := profiles.SetOpen(e.ctx, owner, unlinked, pmodel.SetOpenInput{IsOpen: &closed}); err != nil {
		t.Fatal(err)
	}
	page, _ := profiles.GetPublicPage(e.ctx, owner, unlinked)
	if pp, _ := profiles.PublicProfile(e.ctx, page.Username); !pp.NotShared {
		t.Fatalf("closed unlinked profile: %+v", pp)
	}
}
