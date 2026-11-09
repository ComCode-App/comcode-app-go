package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/legacymigration"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
)

func TestMain(m *testing.M) { pgtest.Main(m) }

func TestCommandsDryRunAndVerificationExitError(t *testing.T) {
	d := pgtest.Open(t)
	// Use the isolated package DB, never the admin/template database.
	config := d.Pool.Config().ConnConfig.ConnString()
	t.Setenv("DATABASE_URL", config)
	dir := t.TempDir()
	input := filepath.Join(dir, "input.jsonl")
	row := legacymigration.Row{LegacyHash: "cli-" + pgtest.Unique(), TargetType: "url", TargetValue: "https://example.test/cli", ExtraJSON: json.RawMessage(`{}`)}
	data, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(input, data, 0600); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	command := func(name, report string, flags ...string) error {
		args := []string{name, "--input", input, "--report", filepath.Join(dir, report), "--frontend-url", "https://comcode.test"}
		return run(context.Background(), append(args, flags...), &stdout)
	}
	if err := command("import", "dry.csv", "--dry-run"); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := d.Pool.QueryRow(context.Background(), `SELECT count(*) FROM routes WHERE path=$1`, row.LegacyHash).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatal("CLI dry-run wrote routes")
	}
	if err := command("verify", "missing.csv"); err == nil {
		t.Fatal("verify missing mapping returned success")
	}
	if err := command("import", "import.csv"); err != nil {
		t.Fatal(err)
	}
	if err := command("verify", "ok.csv"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Pool.Exec(context.Background(), `UPDATE routes SET direct_url='https://example.test/broken' WHERE path=$1`, row.LegacyHash); err != nil {
		t.Fatal(err)
	}
	if err := command("verify", "broken.csv"); err == nil {
		t.Fatal("verify broken mapping returned success")
	}
	if err := command("import", "import.csv"); err == nil {
		t.Fatal("overwrote report")
	}
	// Blocked rows are visible in both stdout and the private operator CSV.
	if err := os.WriteFile(input, []byte(`{"legacy_hash":"leads","target_type":"url","target_value":"https://example.test/blocked"}`), 0600); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	if err := command("import", "blocked.csv"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "blocked: collides with app path\ncount=1") {
		t.Fatalf("blocked summary: %s", stdout.String())
	}
	blockedReport, err := os.ReadFile(filepath.Join(dir, "blocked.csv"))
	if err != nil || !strings.Contains(string(blockedReport), "leads,blocked: collides with app path,") || !strings.Contains(string(blockedReport), "blocked: collides with app path,count,1") {
		t.Fatalf("blocked report: %s %v", blockedReport, err)
	}

}
