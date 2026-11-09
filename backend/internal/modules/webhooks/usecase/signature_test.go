package usecase

import (
	"testing"
	"time"
)

func TestSignAndVerify(t *testing.T) {
	body := []byte(`{"id":"1","event":"lead.created"}`)
	at := time.Unix(1_800_000_000, 0)
	header := Sign("whsec_abc", at, body)
	if err := Verify("whsec_abc", header, body, at.Add(2*time.Minute), 5*time.Minute); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
	if err := Verify("whsec_other", header, body, at, time.Minute); err == nil {
		t.Fatal("wrong secret accepted")
	}
	if err := Verify("whsec_abc", header, []byte(`{"tampered":true}`), at, time.Minute); err == nil {
		t.Fatal("tampered body accepted")
	}
	if err := Verify("whsec_abc", header, body, at.Add(time.Hour), 5*time.Minute); err == nil {
		t.Fatal("stale timestamp accepted")
	}
	if err := Verify("whsec_abc", "v1=deadbeef", body, at, time.Minute); err == nil {
		t.Fatal("header without timestamp accepted")
	}
}

func TestValidateURL(t *testing.T) {
	ok := []string{"https://example.com/hooks", "https://hooks.example.com:8443/a?b=c", "https://8.8.8.8/x"}
	bad := []string{"", "http://example.com/hooks", "ftp://example.com", "https://localhost/x", "https://10.0.0.5/x",
		"https://127.0.0.1/x", "https://169.254.169.254/latest", "https://[fd00::1]/x", "https://user:pw@example.com/x",
		"https://example.com/x#frag", "https://api.internal/x", "https://100.64.1.1/x", "https://0.1.2.3/x",
		"https://198.18.0.1/x", "https://240.0.0.1/x", "https://[2002:c0a8:0101::1]/x", "https://[64:ff9b::a00:1]/x"}
	for _, u := range ok {
		if _, err := ValidateURL(u, false); err != nil {
			t.Errorf("%q: unexpected %v", u, err)
		}
	}
	for _, u := range bad {
		if _, err := ValidateURL(u, false); err == nil {
			t.Errorf("%q: expected error", u)
		}
	}
	if _, err := ValidateURL("http://localhost:9000/hook", true); err != nil {
		t.Errorf("insecure mode must accept local http: %v", err)
	}
}

func TestBackoff(t *testing.T) {
	want := []time.Duration{30 * time.Second, time.Minute, 2 * time.Minute, 4 * time.Minute, 8 * time.Minute, 16 * time.Minute, 32 * time.Minute}
	for i, w := range want {
		if got := Backoff(int32(i + 1)); got != w {
			t.Errorf("Backoff(%d) = %v, want %v", i+1, got, w)
		}
	}
	if got := Backoff(20); got != 6*time.Hour {
		t.Errorf("cap: %v", got)
	}
}
