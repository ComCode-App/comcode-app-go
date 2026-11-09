package legacymigration

import (
	"errors"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/publicpath"
	"io"
	"strings"
	"testing"
)

func TestNormalizedReader(t *testing.T) {
	cases := []struct {
		name, format, data string
		valid              bool
	}{
		{"jsonl", "jsonl", `{"legacy_hash":"AbC","target_type":"url","target_value":"https://example.test","extra_json":{"id":1}}`, true},
		{"CSV multiline", "csv", "legacy_hash,target_type,target_value,owner_email,owner_name,created_at,extra_json\nabc,url,https://example.test,,\"Two\nLines\",,{}\n", true},
		{"bad header", "csv", "hash,type\na,b", false},
		{"extra column", "csv", "legacy_hash,target_type,target_value,owner_email,owner_name,created_at,extra_json\na,url,https://example.test,,,,{},extra", false},
		{"traversal", "jsonl", `{"legacy_hash":"../login","target_type":"unknown"}`, false},
		{"encoded segment", "jsonl", `{"legacy_hash":"%2Flogin","target_type":"unknown"}`, false},
		{"invalid extra", "jsonl", `{"legacy_hash":"abc","target_type":"unknown","extra_json":[]}`, false},
		{"invalid date", "jsonl", `{"legacy_hash":"abc","target_type":"unknown","created_at":"yesterday"}`, false},
		{"unknown field", "jsonl", `{"legacy_hash":"abc","target_type":"unknown","unexpected":true}`, false},
		{"trailing JSON", "jsonl", `{"legacy_hash":"abc","target_type":"unknown"} {}`, false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			reader, err := NewReader(strings.NewReader(tt.data), tt.format)
			if err == nil {
				_, err = reader.Next()
			}
			if (err == nil) != tt.valid {
				t.Fatalf("valid=%t, err=%v", tt.valid, err)
			}
			if err == nil {
				if _, err := reader.Next(); !errors.Is(err, io.EOF) {
					t.Fatalf("want EOF: %v", err)
				}
			}
		})
	}
}

func TestRoutingDocumentReservedPaths(t *testing.T) {
	for _, path := range []string{"checkout", "cart", "products", "plans", "subscriptions", "ready"} {
		if !publicpath.IsReserved(path) {
			t.Errorf("routing document reserved path %q not protected", path)
		}
	}
}
