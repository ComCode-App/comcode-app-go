package logredact

import (
	"errors"
	"strings"
	"testing"
)

func TestStringRemovesPersonalDataAndSecrets(t *testing.T) {
	cases := []struct {
		in      string
		mustNot []string
		want    string
	}{
		{in: "user ali.veli+x@example.com.tr not found", mustNot: []string{"ali.veli", "example.com"}, want: "[email]"},
		{in: "token eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.abc_DEF-1 expired", mustNot: []string{"eyJ"}, want: "[token]"},
		{in: "header Authorization: Bearer abc.def-123", mustNot: []string{"abc.def-123"}, want: "Bearer [token]"},
		{in: `body {"password":"hunter2","email":"a@b.co"}`, mustNot: []string{"hunter2", "a@b.co"}, want: redacted},
		{in: "refresh_token=r3fr3sh&x=1", mustNot: []string{"r3fr3sh"}, want: "refresh_token=" + redacted},
		{in: "sms to +90 532 123 45 67 failed", mustNot: []string{"532 123"}, want: "[phone]"},
		{in: "sms to 0532 123 45 67 failed", mustNot: []string{"123 45 67"}, want: "[phone]"},
		// Expo push tokens (TEC-1187), as Expo quotes them in error messages.
		{in: `"ExponentPushToken[xXyY-123_abc]" is not a registered push notification recipient`, mustNot: []string{"xXyY-123_abc"}, want: "[push-token]"},
		{in: "to=ExpoPushToken[abcdef] failed", mustNot: []string{"abcdef"}, want: "[push-token]"},
		// Apple .p8 / GitHub App keys (TEC-1123): multi-line, one-line with literal \n, unterminated.
		{in: "bad key -----BEGIN PRIVATE KEY-----\nMIGTAgEAMBMGByqGSM49\nAgEG-----END PRIVATE KEY----- end", mustNot: []string{"MIGT", "AgEG"}, want: "[private-key] end"},
		{in: `body {"private_key":"-----BEGIN PRIVATE KEY-----\nMIGTAgEAMBMG\n-----END PRIVATE KEY-----\n"}`, mustNot: []string{"MIGT"}, want: "[private-key]"},
		{in: "truncated -----BEGIN EC PRIVATE KEY-----\nMIGTAgEAMBMGByqG", mustNot: []string{"MIGT"}, want: "[private-key]"},
	}
	for _, tc := range cases {
		got := String(tc.in)
		for _, bad := range tc.mustNot {
			if strings.Contains(got, bad) {
				t.Errorf("String(%q) = %q still contains %q", tc.in, got, bad)
			}
		}
		if !strings.Contains(got, tc.want) {
			t.Errorf("String(%q) = %q, want it to contain %q", tc.in, got, tc.want)
		}
	}
}

func TestStringKeepsOrdinaryText(t *testing.T) {
	in := "auth: invalid credentials (status 401, user 3f2a1c9e-0000-4000-8000-000000000001)"
	if got := String(in); got != in {
		t.Fatalf("String changed plain text: %q", got)
	}
	if Error(nil) != "" {
		t.Fatal("Error(nil) should be empty")
	}
	if got := Error(errors.New("no user a@b.io")); strings.Contains(got, "a@b.io") {
		t.Fatalf("Error leaked email: %q", got)
	}
}
