package application

import (
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/domain"
)

func TestBuildSubjectSourcesForProfile(t *testing.T) {
	rows := []domain.SourceRow{
		{SubjectType: "profile", EventType: "card.view", Count: 40},
		{SubjectType: "profile", EventType: "legacy.profile.view", Source: "legacy", Count: 5},
		{SubjectType: "profile", EventType: "vcard.download", Count: 9},
		{SubjectType: "route", EventType: "card.qr.scan", Source: "qr", Count: 6},   // canonical route QR
		{SubjectType: "card", EventType: "card.qr.scan", Source: "qr", Count: 4},    // card QR
		{SubjectType: "route", EventType: "route.scan", Source: "qr", Count: 4},     // the same card QR scans
		{SubjectType: "route", EventType: "route.scan", Source: "nfc", Count: 20},   // card taps
		{SubjectType: "route", EventType: "route.scan", Source: "legacy", Count: 7}, // legacy hash route
		{SubjectType: "route", EventType: "route.scan", Source: "link", Count: 3},   // smart link
		{SubjectType: "route", EventType: "route.scan", Source: "", Count: 2},       // canonical resolve
	}
	got := BuildSubjectSources("profile", rows)
	if got.Views != 45 {
		t.Errorf("views = %d, want 45 (card.view + legacy page hits, no QR or route scans)", got.Views)
	}
	if got.QRScans != 10 {
		t.Errorf("qr_scans = %d, want 10", got.QRScans)
	}
	want := map[string]int64{"qr": 10, "nfc": 20, "link": 3, "legacy": 12}
	if len(got.Sources) != 4 {
		t.Fatalf("sources = %+v", got.Sources)
	}
	for i, src := range []string{"qr", "nfc", "link", "legacy"} {
		if got.Sources[i].Source != src || got.Sources[i].Count != want[src] {
			t.Errorf("sources[%d] = %+v, want %s=%d", i, got.Sources[i], src, want[src])
		}
	}
	if got.TotalEntries != 45 {
		t.Errorf("total_entries = %d, want 45", got.TotalEntries)
	}
}

func TestBuildSubjectSourcesForCardRouteExcludesQRFromViews(t *testing.T) {
	rows := []domain.SourceRow{
		{SubjectType: "route", EventType: "route.scan", Source: "nfc", Count: 8},
		{SubjectType: "route", EventType: "route.scan", Source: "qr", Count: 3},
		{SubjectType: "card", EventType: "card.qr.scan", Source: "qr", Count: 3},
	}
	got := BuildSubjectSources("route", rows)
	if got.Views != 8 || got.QRScans != 3 {
		t.Fatalf("views=%d qr=%d, want 8 and 3", got.Views, got.QRScans)
	}
	if got.Sources[0].Percentage < 27 || got.Sources[0].Percentage > 28 {
		t.Errorf("qr share = %v, want 3/11", got.Sources[0].Percentage)
	}
}

func TestBuildSubjectSourcesEmpty(t *testing.T) {
	got := BuildSubjectSources("profile", nil)
	if len(got.Sources) != 4 || got.TotalEntries != 0 || got.Sources[0].Percentage != 0 {
		t.Fatalf("empty sources = %+v", got)
	}
}
