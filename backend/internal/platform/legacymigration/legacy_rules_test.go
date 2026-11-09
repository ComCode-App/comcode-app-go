package legacymigration

import (
	"bytes"
	"context"
	"encoding/csv"
	"strings"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	profilerepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/repository"
	routerepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/repository"
	routeusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/storage"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
	"github.com/google/uuid"
)

const legacyUsersFixture = "../../../cmd/migrate-legacy/testdata/legacy_users.csv"

type fakeRouteCache struct{ profiles []uuid.UUID }

func (c *fakeRouteCache) InvalidateProfile(_ context.Context, id uuid.UUID) {
	c.profiles = append(c.profiles, id)
}

type fakeNotifier struct{ claims []HashClaim }

func (n *fakeNotifier) HashClaimed(_ context.Context, c HashClaim) error {
	n.claims = append(n.claims, c)
	return nil
}

// reportRows keys the sync CSV by legacy user ID.
func reportRows(t *testing.T, report []byte) map[string]map[string]string {
	t.Helper()
	records, err := csv.NewReader(bytes.NewReader(report)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]map[string]string{}
	for _, rec := range records[1:] {
		row := map[string]string{}
		for i, col := range records[0] {
			row[col] = rec[i]
		}
		rows[rec[0]] = row
	}
	return rows
}

func TestSectorIconKey(t *testing.T) {
	for in, want := range map[string]string{"teknoloji.png": "teknoloji", "uploads/comps/İnşaat.PNG": "insaat", "Sağlık_Hizmetleri.jpg": "saglik-hizmetleri", "9fb0ef66.png": "9fb0ef66"} {
		if got := sectorIconKey(in); got != want {
			t.Errorf("sectorIconKey(%q) = %q, want %q", in, got, want)
		}
	}
	got, err := Transform(LegacyUser{"user_id": "1", "uniqid_id": "abcde", "ikon2": "/teknoloji.png", "firmalogo": "logo.png", "logo4": "AR Teknoloji,Teknoloji"}, nil)
	if err != nil || got.SectorIcon != "teknoloji.png" || len(got.SectorIcons) != 2 || got.SectorIcons[1] != "logo.png" {
		t.Fatalf("sector icons %q %q %v", got.SectorIcon, got.SectorIcons, err)
	}
}

// TEC-955 + TEC-956 option E against the fixture in cmd/migrate-legacy/testdata.
func TestSyncLegacyAddressRules(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	repo := NewPostgres(d.Pool)
	// A panel user took claimed955 as a username before that card was synced.
	claimer := d.CreateUser(t)
	var claimerProfile uuid.UUID
	if err := d.Pool.QueryRow(ctx, `WITH p AS (INSERT INTO profiles(account_id,user_id) VALUES($1,$2) RETURNING id,uuid)
		INSERT INTO routes(path,route_type,status,target_type,profile_id,account_id,owner_user_id) SELECT 'claimed955','canonical_profile','active','profile',p.id,$1,$2 FROM p RETURNING (SELECT uuid FROM p)`, claimer.AccountID, claimer.ID).Scan(&claimerProfile); err != nil {
		t.Fatal(err)
	}
	// Pre-option-E state: promote955 was imported with the legacy-<uuid> fallback.
	base := SyncOptions{Options: options(), StateDir: t.TempDir()}
	seed := base
	seed.UsersCSV = "../../../cmd/migrate-legacy/testdata/legacy_users_seed.csv"
	seed.Media = &MediaSync{Driver: storage.NewMemory()}
	if _, err := Sync(ctx, repo, seed, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	promoteFallback := fallbackUsername(uuid.NewSHA1(uuid.NameSpaceURL, []byte("comcode:legacy:user:tec955-promote")).String())

	run := func(dry, promote bool, cache *fakeRouteCache, notifier *fakeNotifier) (SyncSummary, map[string]map[string]string, string) {
		t.Helper()
		opts := base
		opts.UsersCSV = legacyUsersFixture
		opts.DryRun, opts.PromoteOwnHash = dry, promote
		opts.Media = &MediaSync{Driver: storage.NewMemory()}
		if cache != nil {
			opts.RouteCache = cache
		}
		if notifier != nil {
			opts.Notifier = notifier
		}
		var report bytes.Buffer
		summary, err := Sync(ctx, repo, opts, &report)
		if err != nil {
			t.Fatal(err)
		}
		return summary, reportRows(t, report.Bytes()), report.String()
	}

	notifier := &fakeNotifier{}
	summary, rows, raw := run(true, false, nil, notifier)
	if strings.Contains(raw, "@example.test") {
		t.Fatal("sync report must not carry owner emails")
	}
	if len(notifier.claims) != 0 {
		t.Fatal("dry run notified a username owner")
	}
	check := func(id, col, want string) {
		t.Helper()
		if got := rows[id][col]; got != want {
			t.Errorf("%s %s = %q, want %q", id, col, got, want)
		}
	}
	contains := func(id, col, want string) {
		t.Helper()
		if !strings.Contains(rows[id][col], want) {
			t.Errorf("%s %s = %q, want it to contain %q", id, col, rows[id][col], want)
		}
	}
	check("tec955-comcode", "status", "created")
	contains("tec955-comcode", "canonical_username", "legacy-")
	contains("tec955-comcode", "warning", "LEGACY_BOUND_RESERVED:comcode")
	check("tec955-comcode", "sector", "technology")
	check("tec955-own", "canonical_rule", CanonicalOwnHash)
	check("tec955-own", "canonical_username", "ownhash955")
	check("tec955-own", "username_status", "claimable")
	check("tec955-own", "sector", "construction")
	check("tec955-promote", "status", "updated")
	check("tec955-promote", "canonical_rule", CanonicalOwnHashPending)
	check("tec955-promote", "canonical_username", promoteFallback)
	contains("tec955-promote", "warning", "USERNAME_OWN_HASH_PENDING:promote955")
	check("tec955-promote", "sector", "")
	check("tec955-claimed", "hash_claimed_by", "canonical_profile:active:"+claimerProfile.String())
	check("tec955-claimed", "sector", "unmatched:bilinmeyen.png")
	check("tec955-invalid", "username_status", "conflict")
	check("tec955-invalid", "sector", "technology")
	// Another card's printed hash is taken even before that card is imported.
	check("tec955-taken", "username_status", "conflict")
	contains("tec955-taken", "warning", "USERNAME_CONFLICT:ownhash955")
	if summary.OwnHash != 1 || summary.OwnHashPending != 1 || summary.HashClaims != 1 || summary.SectorMatched != 3 || summary.SectorUnmatched != 1 || summary.Blocked != 0 {
		t.Fatalf("dry-run summary %+v", summary)
	}

	// The operator's preview of the promotion changes nothing either.
	summary, rows, _ = run(true, true, nil, nil)
	check("tec955-promote", "canonical_rule", CanonicalOwnHashPromote)
	check("tec955-promote", "canonical_username", "promote955")
	check("tec955-promote", "previous_canonical", promoteFallback)
	check("tec955-promote", "username_status", "claimable")
	var n int
	if err := d.Pool.QueryRow(ctx, `SELECT count(*) FROM routes WHERE path IN ('comcode','ownhash955','promote955','r/legacy/promote955')`).Scan(&n); err != nil || n != 1 || summary.OwnHashPromoted != 1 {
		t.Fatalf("dry run wrote routes: %d %v %+v", n, err, summary)
	}

	cache := &fakeRouteCache{}
	if _, err := Sync(ctx, repo, SyncOptions{Options: options(), UsersCSV: legacyUsersFixture, StateDir: t.TempDir(), PromoteOwnHash: true}, &bytes.Buffer{}); err == nil {
		t.Fatal("promotion without route cache invalidation accepted")
	}
	summary, _, _ = run(false, true, cache, notifier)
	if summary.OwnHashPromoted != 1 || summary.Creates != 5 {
		t.Fatalf("apply summary %+v", summary)
	}
	promoteUUID := uuid.NewSHA1(uuid.NameSpaceURL, []byte("comcode:legacy:user:tec955-promote"))
	if len(cache.profiles) != 1 || cache.profiles[0] != promoteUUID {
		t.Fatalf("route cache invalidations %v", cache.profiles)
	}
	if len(notifier.claims) != 1 || notifier.claims[0].ProfileUUID != claimerProfile.String() || notifier.claims[0].OwnerUserID != claimer.ID || !notifier.claims[0].Notify() {
		t.Fatalf("hash claim notifications %+v", notifier.claims)
	}

	routes := map[string]string{}
	pathRows, err := d.Pool.Query(ctx, `SELECT path,route_type||':'||status FROM routes WHERE path IN ('comcode','ownhash955','r/legacy/ownhash955','promote955','r/legacy/promote955','claimed955','r/legacy/claimed955',$1)`, promoteFallback)
	if err != nil {
		t.Fatal(err)
	}
	for pathRows.Next() {
		var path, kind string
		if err = pathRows.Scan(&path, &kind); err != nil {
			t.Fatal(err)
		}
		routes[path] = kind
	}
	pathRows.Close()
	for path, want := range map[string]string{
		"comcode": "legacy_route:active", "ownhash955": "canonical_profile:active", "r/legacy/ownhash955": "legacy_route:active",
		"promote955": "canonical_profile:active", "r/legacy/promote955": "legacy_route:active", promoteFallback: "canonical_profile:retired",
		"claimed955": "canonical_profile:active", "r/legacy/claimed955": "legacy_route:active",
	} {
		if routes[path] != want {
			t.Errorf("route %s = %q, want %q", path, routes[path], want)
		}
	}
	if err = d.Pool.QueryRow(ctx, `SELECT count(*) FROM route_history WHERE old_path=$1 AND new_path='promote955' AND reason='username_change'`, promoteFallback).Scan(&n); err != nil || n != 1 {
		t.Fatalf("promotion history %d %v", n, err)
	}
	// Migration 000066 allows only the one move to the alias; the identity is otherwise frozen.
	for _, path := range []string{"elsewhere955", "r/legacy/r/legacy/ownhash955"} {
		if _, err = d.Pool.Exec(ctx, `UPDATE routes SET path=$1 WHERE path='r/legacy/ownhash955'`, path); err == nil {
			t.Fatalf("immutable legacy route moved to %s", path)
		}
	}
	sectorOf := func(hash string) string {
		t.Helper()
		var slug string
		if err := d.Pool.QueryRow(ctx, `SELECT COALESCE(sec.slug,'') FROM legacy_sync_users s JOIN profiles p ON p.id=s.profile_id LEFT JOIN sectors sec ON sec.id=p.sector_id WHERE s.legacy_hash=$1`, hash).Scan(&slug); err != nil {
			t.Fatal(err)
		}
		return slug
	}
	for hash, want := range map[string]string{"comcode": "technology", "ownhash955": "construction", "invalid955": "technology", "claimed955": "", "promote955": ""} {
		if got := sectorOf(hash); got != want {
			t.Errorf("sector of %s = %q, want %q", hash, got, want)
		}
	}

	// Visitors: /comcode opens its card, own-hash usernames render in place, a claimed hash
	// serves the printed card.
	resolver := routeusecase.NewService(routerepo.NewPostgres(db.New(d.Pool)), "https://comcode.test", nil)
	comcodeUUID := uuid.NewSHA1(uuid.NameSpaceURL, []byte("comcode:legacy:user:tec955-comcode")).String()
	for path, want := range map[string]string{
		"comcode": "https://comcode.test/" + fallbackUsername(comcodeUUID), "ownhash955": "", "promote955": "",
		"claimed955": "https://comcode.test/claimed-card-955",
	} {
		res, err := resolver.ResolveLegacy(ctx, path)
		if err != nil || (want == "") != (res == nil) || (res != nil && res.TargetURL != want) {
			t.Errorf("resolve /%s = %+v %v, want %q", path, res, err, want)
		}
	}
	// Printed hashes stay taken for every username claim.
	owners := profilerepo.NewPostgres(d.Pool, db.New(d.Pool))
	for _, path := range []string{"comcode", "ownhash955", "promote955", "claimed955", "taken955"} {
		if owner, err := owners.PathOwner(ctx, path); err != nil || owner == nil {
			t.Errorf("username %s claimable: %+v %v", path, owner, err)
		}
	}

	// Re-runs are stable: no second promotion; a cleared sector link is restored, an owner's
	// different sector is kept.
	if _, err = d.Pool.Exec(ctx, `UPDATE profiles SET sector_id=NULL WHERE id=(SELECT profile_id FROM legacy_sync_users WHERE legacy_hash='ownhash955')`); err != nil {
		t.Fatal(err)
	}
	if _, err = d.Pool.Exec(ctx, `UPDATE profiles SET sector_id=(SELECT id FROM sectors WHERE slug='media') WHERE id=(SELECT profile_id FROM legacy_sync_users WHERE legacy_hash='comcode')`); err != nil {
		t.Fatal(err)
	}
	summary, rows, _ = run(false, true, cache, notifier)
	if summary.Skips != 6 || summary.OwnHashPromoted != 0 || len(cache.profiles) != 1 {
		t.Fatalf("re-run summary %+v cache %v", summary, cache.profiles)
	}
	check("tec955-promote", "canonical_username", "promote955")
	check("tec955-promote", "username_status", "claimable")
	check("tec955-promote", "canonical_rule", "")
	if sectorOf("ownhash955") != "construction" || sectorOf("comcode") != "media" {
		t.Fatalf("sector re-link: %q %q", sectorOf("ownhash955"), sectorOf("comcode"))
	}
}
