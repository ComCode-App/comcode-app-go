package config

import (
	"strings"
	"testing"
)

func TestParseReviewAccounts(t *testing.T) {
	got, err := parseReviewAccounts(" Review@Example.com:246810 , other@x.io : 913572 ")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got["review@example.com"] != "246810" || got["other@x.io"] != "913572" {
		t.Fatalf("got %v", got)
	}
	for _, blank := range []string{"", "   "} {
		m, err := parseReviewAccounts(blank)
		if err != nil || len(m) != 0 {
			t.Fatalf("blank %q must disable: %v %v", blank, m, err)
		}
	}
}

// A malformed or weak value is refused, and the error never echoes the code.
func TestParseReviewAccountsRejectsMalformed(t *testing.T) {
	cases := map[string]string{
		"no code":         "review@example.com",
		"empty code":      "review@example.com:",
		"short code":      "review@example.com:24681",
		"long code":       "review@example.com:2468103",
		"letters":         "review@example.com:24a810",
		"no email":        ":246810",
		"bad email":       "review.example.com:246810",
		"no domain dot":   "review@localhost:246810",
		"empty entry":     "review@example.com:246810,,other@x.io:913572",
		"trailing comma":  "review@example.com:246810,",
		"duplicate email": "review@example.com:246810,REVIEW@example.com:913572",
		"same digit":      "a@x.io:777777",
		"zeros":           "a@x.io:000000",
		"ascending":       "a@x.io:123456",
		"descending":      "a@x.io:654321",
		"wrapping run":    "a@x.io:890123",
		"pair repeat":     "a@x.io:121212",
		"triple repeat":   "a@x.io:123123",
		"common":          "a@x.io:112233",
		"good then bad":   "a@x.io:246810,b@x.io:111111",
	}
	for name, raw := range cases {
		m, err := parseReviewAccounts(raw)
		if err == nil {
			t.Errorf("%s: %q accepted as %v", name, raw, m)
			continue
		}
		_, code, _ := strings.Cut(strings.Split(raw, ",")[0], ":")
		if code = strings.TrimSpace(code); len(code) >= 4 && strings.Contains(err.Error(), code) {
			t.Errorf("%s: error leaks the code: %v", name, err)
		}
	}
}

func TestLoadReviewAccounts(t *testing.T) {
	for _, env := range []string{EnvDevelopment, EnvProduction} {
		setEnv(t, env, nil)
		t.Setenv("AUTH_REVIEW_ACCOUNTS", "")
		cfg, err := Load()
		if err != nil || len(cfg.Auth.ReviewAccounts) != 0 {
			t.Fatalf("%s blank: %v %v", env, cfg.Auth.ReviewAccounts, err)
		}

		t.Setenv("AUTH_REVIEW_ACCOUNTS", "Review@Example.com:246810")
		cfg, err = Load()
		if err != nil || cfg.Auth.ReviewAccounts["review@example.com"] != "246810" {
			t.Fatalf("%s valid: %v %v", env, cfg.Auth.ReviewAccounts, err)
		}

		// Never half-applied: a bad value stops boot in every environment.
		t.Setenv("AUTH_REVIEW_ACCOUNTS", "review@example.com:123456")
		if _, err := Load(); err == nil || !strings.Contains(err.Error(), "AUTH_REVIEW_ACCOUNTS") {
			t.Fatalf("%s weak code must be refused, got %v", env, err)
		}
	}
}
