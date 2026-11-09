package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/legacymigration"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/storage"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
)

// The dry-run report lists the TEC-955/TEC-956 decisions without writing anything.
func TestSyncDryRunReportsLegacyAddressRules(t *testing.T) {
	d := pgtest.Open(t)
	t.Setenv("DATABASE_URL", d.Pool.Config().ConnConfig.ConnString())
	t.Setenv("REDIS_ADDR", "")
	ctx := context.Background()
	dir := t.TempDir()
	// Pre-option-E state: promote955 imported with the legacy-<uuid> fallback.
	seed := legacymigration.SyncOptions{
		Options:  legacymigration.Options{Account: "holding", BatchID: "cli-955"},
		UsersCSV: "testdata/legacy_users_seed.csv", StateDir: filepath.Join(dir, "seed"),
		Media: &legacymigration.MediaSync{Driver: storage.NewMemory()},
	}
	if _, err := legacymigration.Sync(ctx, legacymigration.NewPostgres(d.Pool), seed, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	report := filepath.Join(dir, "dry-run.csv")
	var stdout bytes.Buffer
	args := []string{"sync", "--users-csv", "testdata/legacy_users.csv", "--state-dir", filepath.Join(dir, "state"), "--report", report}
	if err := run(ctx, append(args, "--dry-run", "--promote-own-hash"), &stdout); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(report)
	if err != nil {
		t.Fatal(err)
	}
	out := string(data)
	for _, want := range []string{
		",canonical_rule,previous_canonical,hash_claimed_by,sector\n",
		"tec955-comcode,comcode,", "LEGACY_BOUND_RESERVED:comcode",
		",ownhash955,claimable,", ",own_hash,,,construction\n",
		",promote955,claimable,", ",own_hash_promote,legacy-",
		"unmatched:bilinmeyen.png",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("dry-run report lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "blocked") || strings.Contains(out, "@example.test") {
		t.Errorf("comcode blocked or emails leaked:\n%s", out)
	}
	if !strings.Contains(stdout.String(), `"OwnHashPromoted":1`) {
		t.Errorf("summary: %s", stdout.String())
	}
	var n int
	if err = d.Pool.QueryRow(ctx, `SELECT count(*) FROM routes WHERE path IN ('comcode','ownhash955','r/legacy/promote955')`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("dry run wrote routes: %d %v", n, err)
	}
	if os.Getenv("LEGACY_SYNC_REPORT_OUT") != "" {
		_ = os.WriteFile(os.Getenv("LEGACY_SYNC_REPORT_OUT"), data, 0600)
	}
	// A real promotion refuses to run without route cache invalidation.
	err = run(ctx, append(args[:len(args)-1], filepath.Join(dir, "real.csv"), "--promote-own-hash"), &stdout)
	if err == nil || !strings.Contains(err.Error(), "REDIS_ADDR") {
		t.Fatalf("promotion without Redis: %v", err)
	}
}
