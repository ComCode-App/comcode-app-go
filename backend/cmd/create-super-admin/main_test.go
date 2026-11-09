package main

import (
	"errors"
	"io"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/password"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestParseOptionsRequiresEmailAndPassword(t *testing.T) {
	cases := map[string][]string{
		"no args":       nil,
		"email only":    {"-email", "ops@x.test"},
		"password only": {"-password", "Str0ngPassword"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := parseOptions(args, env(nil), io.Discard); err == nil {
				t.Fatal("expected an error: there must be no default email or password")
			}
		})
	}
}

func TestParseOptionsRejectsWeakPassword(t *testing.T) {
	for _, pw := range []string{"short1A", "alllowercase1", "NoDigitsHere", "12345678"} {
		_, err := parseOptions([]string{"-email", "ops@x.test", "-password", pw}, env(nil), io.Discard)
		if !errors.Is(err, password.ErrInvalidPassword) {
			t.Fatalf("%q: got %v, want ErrInvalidPassword", pw, err)
		}
	}
	_, err := parseOptions([]string{"-email", "ops@x.test"}, env(map[string]string{"SA_PASSWORD": "weak"}), io.Discard)
	if !errors.Is(err, password.ErrInvalidPassword) {
		t.Fatalf("weak SA_PASSWORD: got %v", err)
	}
}

func TestParseOptionsRejectsInvalidEmail(t *testing.T) {
	if _, err := parseOptions([]string{"-email", "admin", "-password", "Str0ngPassword"}, env(nil), io.Discard); err == nil {
		t.Fatal("expected an error for an email without @")
	}
}

func TestParseOptionsFlagsAndEnv(t *testing.T) {
	o, err := parseOptions([]string{"-email", " Ops@X.test "}, env(map[string]string{"SA_PASSWORD": "Str0ngPassword"}), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if o.email != "ops@x.test" || o.password != "Str0ngPassword" || o.name != "Platform" || o.surname != "Admin" {
		t.Fatalf("unexpected options: %+v", o)
	}

	o, err = parseOptions(nil, env(map[string]string{"SA_EMAIL": "env@x.test", "SA_PASSWORD": "Str0ngPassword"}), io.Discard)
	if err != nil || o.email != "env@x.test" {
		t.Fatalf("env fallback: %+v, %v", o, err)
	}

	// Flags win over the environment.
	o, err = parseOptions([]string{"-email", "flag@x.test", "-password", "FlagPassw0rd"},
		env(map[string]string{"SA_EMAIL": "env@x.test", "SA_PASSWORD": "EnvPassw0rd"}), io.Discard)
	if err != nil || o.email != "flag@x.test" || o.password != "FlagPassw0rd" {
		t.Fatalf("flag precedence: %+v, %v", o, err)
	}
}
