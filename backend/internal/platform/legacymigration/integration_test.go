package legacymigration

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	profileshandler "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/handler"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	profilerepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/model"
	routerepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
)

func TestMain(m *testing.M) { pgtest.Main(m) }

func fixture(t *testing.T) *Reader {
	t.Helper()
	f, err := os.Open("../../../cmd/migrate-legacy/testdata/cards.csv")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := f.Close(); err != nil {
			t.Error(err)
		}
	})
	reader, err := NewReader(f, "csv")
	if err != nil {
		t.Fatal(err)
	}
	return reader
}

func options() Options {
	return Options{Account: "holding", BatchID: "test-560", BatchSize: 1, FrontendURL: "https://comcode.test"}
}

func TestImportDryRunIdempotencyAndBlockedAppPaths(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	repo := NewPostgres(d.Pool)
	opts := options()
	var report, sample bytes.Buffer
	count := func() int {
		var n int
		if err := d.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM routes)+(SELECT count(*) FROM accounts)`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	before := count()
	opts.DryRun = true
	summary, err := Import(ctx, repo, fixture(t), opts, &report, &sample)
	if err != nil || summary.Created != 1 || summary.Blocked != 1 || count() != before {
		t.Fatalf("dry run: %+v %v", summary, err)
	}
	if !strings.Contains(report.String(), BlockedAppPath+",count,1") || !strings.Contains(sample.String(), "Abc560") {
		t.Fatalf("missing blocked section/count or samples: %s", report.String())
	}
	opts.DryRun = false
	report.Reset()
	summary, err = Import(ctx, repo, fixture(t), opts, &report, &sample)
	if err != nil || summary.Created != 1 || summary.Blocked != 1 {
		t.Fatalf("import: %+v %v", summary, err)
	}
	var stored []byte
	var immutable bool
	var timestamp string
	if err := d.Pool.QueryRow(ctx, `SELECT is_immutable,legacy_metadata,created_at::text FROM routes WHERE path='abc560'`).Scan(&immutable, &stored, &timestamp); err != nil {
		t.Fatal(err)
	}
	if !immutable || !strings.HasPrefix(timestamp, "2020-01-02") {
		t.Fatal("lost immutability/source timestamp")
	}
	before = count()
	opts.BatchID = "different-batch"
	summary, err = Import(ctx, repo, fixture(t), opts, &report, &sample)
	if err != nil || summary.Skipped != 1 || summary.Blocked != 1 || count() != before {
		t.Fatalf("rerun: %+v %v", summary, err)
	}
	var after []byte
	if err := d.Pool.QueryRow(ctx, `SELECT legacy_metadata FROM routes WHERE path='abc560'`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stored, after) {
		t.Fatal("rerun changed metadata")
	}
	for _, hash := range []string{"login", "leads", "orders", "account", "checkout", "ready", "LOGIN"} {
		row := Row{LegacyHash: hash, TargetType: "url", TargetValue: "https://example.test/blocked"}
		for _, dry := range []bool{true, false} {
			opts.DryRun = dry
			results, err := repo.Batch(ctx, []Row{row, row}, opts)
			if err != nil || len(results) != 2 || results[0].Status != BlockedAppPath || results[1].Status != BlockedAppPath {
				t.Fatalf("blocked %s: %+v %v", hash, results, err)
			}
		}
	}
	var aliases int
	if err := d.Pool.QueryRow(ctx, `SELECT count(*) FROM routes WHERE path IN ('r/legacy/login','r/legacy/leads','r/legacy/orders','r/legacy/account','r/legacy/checkout','r/legacy/ready')`).Scan(&aliases); err != nil {
		t.Fatal(err)
	}
	if aliases != 0 || count() != before {
		t.Fatal("blocked rows changed the database")
	}
	resolver := usecase.NewService(routerepo.NewPostgres(db.New(d.Pool)), opts.FrontendURL, nil)
	resolution, err := resolver.Resolve(ctx, "abc560")
	if err != nil || len(resolution.LegacyMetadata) != 0 {
		t.Fatalf("public provenance: %+v %v", resolution, err)
	}
	report.Reset()
	if n, err := Verify(ctx, repo, resolver, fixture(t), opts, &report); err == nil || n != 1 || !strings.Contains(report.String(), BlockedAppPath) {
		t.Fatalf("blocked verification: %d %v %s", n, err, report.String())
	}
	if _, err := d.Pool.Exec(ctx, `UPDATE routes SET direct_url='https://example.test/BROKEN' WHERE path='abc560'`); err != nil {
		t.Fatal(err)
	}
	report.Reset()
	if n, err := Verify(ctx, repo, resolver, fixture(t), opts, &report); err == nil || n != 2 || !strings.Contains(report.String(), "target_mismatch") {
		t.Fatalf("broken verification: %d %v %s", n, err, report.String())
	}
}

func TestOwnerMappingProfileVCardSmartLinkAndUsernameCollision(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	u := d.CreateUser(t)
	var email string
	if err := d.Pool.QueryRow(ctx, `SELECT email FROM users WHERE id=$1`, u.ID).Scan(&email); err != nil {
		t.Fatal(err)
	}
	var profileID int64
	var profileUUID uuid.UUID
	if err := d.Pool.QueryRow(ctx, `INSERT INTO profiles (account_id,user_id) VALUES ($1,$2) RETURNING id,uuid`, u.AccountID, u.ID).Scan(&profileID, &profileUUID); err != nil {
		t.Fatal(err)
	}
	username := "collision-" + pgtest.Unique()
	rr := routerepo.NewPostgres(d.Queries)
	if _, err := rr.CreateRoute(ctx, model.CreateReservedRouteInput{Path: username, RouteType: model.RouteTypeCanonicalProfile, Status: model.RouteStatusActive, TargetType: model.TargetTypeProfile, ProfileID: &profileID, AccountID: &u.AccountID, OwnerUserID: &u.ID}); err != nil {
		t.Fatal(err)
	}
	linkPath := "s/test-" + pgtest.Unique()
	if _, err := rr.CreateRoute(ctx, model.CreateReservedRouteInput{Path: linkPath, RouteType: model.RouteTypeCustomSmartLink, Status: model.RouteStatusActive, TargetType: model.TargetTypeDirectURL, DirectURL: "https://example.test/link", AccountID: &u.AccountID}); err != nil {
		t.Fatal(err)
	}
	opts := options()
	opts.MatchOwners = true
	rows := []Row{
		{LegacyHash: username, TargetType: "url", TargetValue: "https://example.test/collision", OwnerEmail: strings.ToUpper(email)},
		{LegacyHash: "prof-" + pgtest.Unique(), TargetType: "profile", TargetValue: profileUUID.String(), OwnerEmail: email},
		{LegacyHash: "vcf-" + pgtest.Unique(), TargetType: "vcard", TargetValue: profileUUID.String(), OwnerEmail: email},
		{LegacyHash: "smart-" + pgtest.Unique(), TargetType: "smart_link", TargetValue: linkPath, OwnerEmail: email},
	}
	repo := NewPostgres(d.Pool)
	for i := range rows {
		if err := rows[i].validate(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := repo.Batch(ctx, rows[1:], opts); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		var data, report bytes.Buffer
		if err := json.NewEncoder(&data).Encode(row); err != nil {
			t.Fatal(err)
		}
		reader, err := NewReader(&data, "jsonl")
		if err != nil {
			t.Fatal(err)
		}
		resolver := usecase.NewService(rr, opts.FrontendURL, nil)
		if n, err := Verify(ctx, repo, resolver, reader, opts, &report); err != nil || n != 0 {
			t.Fatalf("target verify: %d %v %s", n, err, report.String())
		}
	}
	results, err := repo.Batch(ctx, rows[:1], opts)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(results[0].Warning, "COLLISION") {
		t.Fatal("missing username collision warning")
	}
	var accountID, ownerID int64
	if err := d.Pool.QueryRow(ctx, `SELECT account_id,owner_user_id FROM routes WHERE path=$1`, fallbackPrefix+username).Scan(&accountID, &ownerID); err != nil {
		t.Fatal(err)
	}
	if accountID != u.AccountID || ownerID != u.ID {
		t.Fatal("owner mapping failed")
	}
	resolver := usecase.NewService(rr, opts.FrontendURL, nil)
	r, err := resolver.Resolve(ctx, username)
	if err != nil || r.TargetURL != "https://example.test/collision" {
		t.Fatalf("username precedence: %+v %v", r, err)
	}
	// Exercise the exact backend call used by the existing [username] page.
	profileHandler := profileshandler.New(nil)
	profileHandler.SetLegacyResolver(resolver)
	request := httptest.NewRequest(http.MethodGet, "/v1/public/p/"+username, nil)
	request.SetPathValue("username", username)
	response := httptest.NewRecorder()
	profileHandler.PublicProfile(response, request)
	if response.Code != http.StatusTemporaryRedirect || response.Header().Get("Location") != "https://example.test/collision" || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("public page collision: %d %v %s", response.Code, response.Header(), response.Body.String())
	}
	if len(r.LegacyMetadata) != 0 {
		t.Fatal("public resolver exposed metadata")
	}
	if taken, err := rr.CheckPathExists(ctx, username, nil); err != nil || !taken {
		t.Fatalf("alias identity: %t %v", taken, err)
	}
	if own, err := profilerepo.NewPostgres(d.Pool, d.Queries).PathOwner(ctx, username); err != nil || own == nil || own.RouteType != model.RouteTypeLegacyRoute {
		t.Fatalf("alias owner: %+v %v", own, err)
	}
	// A canonical username shadowed by a printed hash cannot be used as a migration target.
	// The validator must flag this dependency before cutover.
	for _, row := range rows[1:3] {
		var body bytes.Buffer
		if err := json.NewEncoder(&body).Encode(row); err != nil {
			t.Fatal(err)
		}
		reader, err := NewReader(&body, "jsonl")
		if err != nil {
			t.Fatal(err)
		}
		var report bytes.Buffer
		if n, err := Verify(ctx, repo, resolver, reader, opts, &report); err == nil || n != 1 {
			t.Fatalf("shadowed profile target: %d %v", n, err)
		}
	}
}

func TestBatchRollbackUnknownAndChangedProvenance(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	repo := NewPostgres(d.Pool)
	opts := options()
	hash := "rollback-" + pgtest.Unique()
	rows := []Row{{LegacyHash: hash, TargetType: "url", TargetValue: "https://example.test/a"}, {LegacyHash: "bad-" + pgtest.Unique(), TargetType: "url", TargetValue: "javascript:alert(1)"}}
	for i := range rows {
		if err := rows[i].validate(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := repo.Batch(ctx, rows, opts); err == nil {
		t.Fatal("accepted unsafe URL")
	}
	var count int
	if err := d.Pool.QueryRow(ctx, `SELECT count(*) FROM routes WHERE path=$1`, hash).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("failed batch partially committed")
	}
	if _, err := repo.Batch(ctx, rows[:1], opts); err != nil {
		t.Fatal(err)
	}
	rows[0].TargetValue = "https://example.test/b"
	if _, err := repo.Batch(ctx, rows[:1], opts); err == nil {
		t.Fatal("overwrote immutable input")
	}
	unknown := Row{LegacyHash: "unknown-" + pgtest.Unique(), TargetType: "unknown", ExtraJSON: json.RawMessage(`{}`)}
	if _, err := repo.Batch(ctx, []Row{unknown}, opts); err != nil {
		t.Fatal(err)
	}
	var data bytes.Buffer
	if err := json.NewEncoder(&data).Encode(unknown); err != nil {
		t.Fatal(err)
	}
	reader, err := NewReader(&data, "jsonl")
	if err != nil {
		t.Fatal(err)
	}
	var report bytes.Buffer
	resolver := usecase.NewService(routerepo.NewPostgres(d.Queries), opts.FrontendURL, nil)
	if n, err := Verify(ctx, repo, resolver, reader, opts, &report); err == nil || n != 1 {
		t.Fatalf("unknown target verified: %d %v", n, err)
	}
	records, err := csv.NewReader(&report).ReadAll()
	if err != nil || len(records) != 2 || records[0][3] != "status" {
		t.Fatalf("report: %v %v", records, err)
	}
}
