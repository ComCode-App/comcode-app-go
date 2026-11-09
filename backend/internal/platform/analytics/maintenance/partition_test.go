package maintenance

import (
	"reflect"
	"testing"
	"time"
)

func TestExpiredPartitions(t *testing.T) {
	cutoff := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	names := []string{"events_2026_01", "events_2026_02", "events_2026_03", "events_2026_04", "events_default", "events_x"}
	got := expiredPartitions(names, cutoff)
	// March ends after the cutoff, so it is pruned row by row instead of dropped.
	if want := []string{"events_2026_01", "events_2026_02"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got := expiredPartitions(names, time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)); len(got) != 2 {
		t.Fatalf("a month ending exactly at the cutoff is expired: %v", got)
	}
}
