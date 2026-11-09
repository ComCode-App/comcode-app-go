package reqid

import (
	"context"
	"strings"
	"testing"
)

func TestValid(t *testing.T) {
	cases := map[string]bool{
		"":                                     false,
		"abc-123":                              true,
		"0b6f2a52-7f4e-4f7d-9c1c-1f2e3d4c5b6a": true,
		"trace:abc.def_1":                      true,
		"has space":                            false,
		"new\nline":                            false,
		"<script>":                             false,
		strings.Repeat("a", MaxLen):            true,
		strings.Repeat("a", MaxLen+1):          false,
	}
	for in, want := range cases {
		if got := Valid(in); got != want {
			t.Errorf("Valid(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestResolveAndContext(t *testing.T) {
	if got := Resolve("keep-me"); got != "keep-me" {
		t.Fatalf("Resolve kept = %q", got)
	}
	gen := Resolve("bad id")
	if gen == "bad id" || !Valid(gen) {
		t.Fatalf("Resolve generated = %q", gen)
	}
	ctx := With(context.Background(), "rid-1")
	if From(ctx) != "rid-1" {
		t.Fatal("From did not return stored id")
	}
	if From(With(context.Background(), "")) != "" {
		t.Fatal("empty id should not be stored")
	}
}
