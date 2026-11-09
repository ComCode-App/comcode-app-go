package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestOwnersCLIValidation(t *testing.T) {
	for _, args := range [][]string{
		{"owners"},
		{"owners", "--users-csv", "users.csv"},
		{"owners", "--report", "report.csv", "extra"},
		{"owners", "rollback", "--unexpected"},
	} {
		var out bytes.Buffer
		if err := run(context.Background(), args, &out); err == nil {
			t.Fatalf("accepted args %v", args)
		}
	}
	t.Setenv("DATABASE_URL", "")
	var out bytes.Buffer
	err := run(context.Background(), []string{"owners", "rollback", "--all"}, &out)
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("rollback dispatch: %v", err)
	}
}
