package main

import (
	"bytes"
	"io"
	"strings"
	"testing"

	profilesusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/usecase"
)

func TestParseOptionsDefaultsToDryRun(t *testing.T) {
	o, err := parseOptions(nil, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if o.apply || o.inline || o.batchSize != profilesusecase.DefaultBackfillBatchSize || o.afterID != 0 || o.limit != 0 {
		t.Fatalf("unexpected defaults: %+v", o)
	}
	o, err = parseOptions([]string{"-apply", "-inline", "-batch-size", "50", "-after-id", "42", "-limit", "10"}, io.Discard)
	if err != nil || !o.apply || !o.inline || o.batchSize != 50 || o.afterID != 42 || o.limit != 10 {
		t.Fatalf("parse: %+v %v", o, err)
	}
}

func TestParseOptionsRejectsBadInput(t *testing.T) {
	for _, args := range [][]string{
		{"-batch-size", "0"},
		{"-batch-size", "5001"},
		{"-after-id", "-1"},
		{"-limit", "-1"},
		{"-inline"},
		{"extra"},
	} {
		if _, err := parseOptions(args, io.Discard); err == nil {
			t.Errorf("%v: expected an error", args)
		}
	}
}

func TestPrintSummaryHasOnlyCounts(t *testing.T) {
	var buf bytes.Buffer
	printSummary(&buf, "dry-run", profilesusecase.BackfillSummary{DeletedProfiles: 3, Candidates: 5, Deletable: 4, Skipped: 1, LastID: 9})
	out := buf.String()
	for _, want := range []string{"deleted_profiles=3", "candidate_objects=5 deletable=4 skipped_referenced=1", "nothing was deleted", "last_id=9"} {
		if !strings.Contains(out, want) {
			t.Errorf("summary misses %q:\n%s", want, out)
		}
	}
}
