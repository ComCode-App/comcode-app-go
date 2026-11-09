package handler

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/auth/model"
)

func TestSessionMetaClientHeaders(t *testing.T) {
	r := httptest.NewRequest("POST", "/v1/auth/login", nil)
	r.Header.Set("X-Device-Name", "Ignored for web")
	if m := sessionMeta(r); m.ClientType != model.ClientWeb || m.DeviceName != "" || m.AppVersion != "" {
		t.Fatalf("web meta = %+v", m)
	}

	for _, v := range []string{"mobile", "comcode-mobile", " Mobile "} {
		r := httptest.NewRequest("POST", "/v1/auth/login", nil)
		r.Header.Set("X-Client", v)
		r.Header.Set("X-Device-Name", "  iPhone\t15\x00 Pro  ")
		r.Header.Set("X-App-Version", "1.0.3")
		m := sessionMeta(r)
		if m.ClientType != model.ClientMobile || m.DeviceName != "iPhone 15 Pro" || m.AppVersion != "1.0.3" {
			t.Fatalf("X-Client %q: meta = %+v", v, m)
		}
	}

	r = httptest.NewRequest("POST", "/v1/auth/login", nil)
	r.Header.Set("X-Client", "desktop")
	if m := sessionMeta(r); m.ClientType != model.ClientWeb {
		t.Fatalf("unknown client must be web: %+v", m)
	}
}

func TestClientLabelCapsLength(t *testing.T) {
	if got := clientLabel(strings.Repeat("ş", 300), maxDeviceNameLen); len([]rune(got)) != maxDeviceNameLen {
		t.Fatalf("len = %d", len([]rune(got)))
	}
	if got := clientLabel("\u202e\x07", maxAppVersionLen); got != "" {
		t.Fatalf("control/format chars kept: %q", got)
	}
}
