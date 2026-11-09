package domain_test

import (
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/domain"
)

func TestEntrySource(t *testing.T) {
	scan := func(routeType string, extra map[string]any) domain.Event {
		props := map[string]any{"route_type": routeType}
		for k, v := range extra {
			props[k] = v
		}
		return domain.Event{Type: "route.scan", Properties: props}
	}
	cases := []struct {
		name string
		ev   domain.Event
		want string
	}{
		{"qr scan", domain.Event{Type: "card.qr.scan"}, "qr"},
		{"card route tap", scan("card_route", nil), "nfc"},
		{"legacy hash route", scan("legacy_route", nil), "legacy"},
		{"smart link", scan("custom_smart_link", nil), "link"},
		{"qr-marked card route", scan("card_route", map[string]any{"entry_source": "qr"}), "qr"},
		{"canonical route", scan("canonical_profile", nil), ""},
		{"imported legacy hit", domain.Event{Type: "legacy.profile.view"}, "legacy"},
		{"page view", domain.Event{Type: "card.view"}, ""},
	}
	for _, c := range cases {
		if got := domain.EntrySource(c.ev); got != c.want {
			t.Errorf("%s: EntrySource = %q, want %q", c.name, got, c.want)
		}
	}
}
