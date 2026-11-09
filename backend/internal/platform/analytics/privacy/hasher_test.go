package privacy_test

import (
	"net"
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/privacy"
)

func TestRotatingSaltHasher(t *testing.T) {
	hasher := privacy.NewRotatingSaltHasher("test-secret-key", 7)

	ip1 := net.ParseIP("192.168.1.100")
	ip2 := net.ParseIP("192.168.1.101")
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)

	hash1 := hasher.HashIP(ip1, now)
	hash1Repeat := hasher.HashIP(ip1, now.Add(2*time.Hour))
	hash2 := hasher.HashIP(ip2, now)

	if hash1 == "" {
		t.Fatal("expected non-empty hash")
	}
	if hash1 != hash1Repeat {
		t.Fatalf("expected deterministic hash within same period: %s vs %s", hash1, hash1Repeat)
	}
	if hash1 == hash2 {
		t.Fatalf("expected different hashes for different IPs: %s vs %s", hash1, hash2)
	}

	// Next rotation period (e.g., 8 days later)
	nextPeriod := now.Add(8 * 24 * time.Hour)
	hash1NextPeriod := hasher.HashIP(ip1, nextPeriod)

	if hash1 == hash1NextPeriod {
		t.Fatalf("expected different hash after salt rotation: %s vs %s", hash1, hash1NextPeriod)
	}
}

func TestRotatingSaltHasherKeepsHashesWithinADayWhenThePeriodChanges(t *testing.T) {
	ip := net.ParseIP("203.0.113.7")
	changeDay := time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC)
	morning := changeDay.Add(8 * time.Hour)
	evening := changeDay.Add(22 * time.Hour)

	schedule := privacy.RotationSchedule{Days: 7}
	hasher := privacy.NewRotatingSaltHasher("k", 99).WithSchedule(func() privacy.RotationSchedule { return schedule })
	fixed7 := privacy.NewRotatingSaltHasher("k", 7)

	before := hasher.HashIP(ip, morning)
	if before != fixed7.HashIP(ip, morning) {
		t.Fatal("a 7-day schedule must hash like the fixed 7-day hasher")
	}

	// The admin switches to 1 day at noon, then to 3 days in the evening: today keeps the 7-day salt.
	schedule = privacy.RotationSchedule{Days: 1, PreviousDays: 7, ChangedOn: changeDay}
	schedule = privacy.RotationSchedule{Days: 3, PreviousDays: schedule.DaysOn(changeDay), ChangedOn: changeDay}
	if got := hasher.HashIP(ip, evening); got != before {
		t.Fatalf("hash changed within the day of the period change: %s vs %s", got, before)
	}
	if got := schedule.DaysOn(evening); got != 7 {
		t.Fatalf("DaysOn(change day) = %d, want 7", got)
	}

	// From the next UTC day the new period applies.
	nextDay := changeDay.AddDate(0, 0, 1)
	if got := schedule.DaysOn(nextDay.Add(time.Minute)); got != 3 {
		t.Fatalf("DaysOn(next day) = %d, want 3", got)
	}
	fixed3 := privacy.NewRotatingSaltHasher("k", 3)
	if a, b := hasher.HashIP(ip, nextDay.Add(time.Hour)), hasher.HashIP(ip, nextDay.Add(23*time.Hour)); a != b || a != fixed3.HashIP(ip, nextDay) {
		t.Fatal("the next day must hash consistently with the new 3-day period")
	}
}

func TestRotationScheduleClampsOutOfRangePeriods(t *testing.T) {
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	for _, days := range []int{0, -1, 31, 365} {
		if got := (privacy.RotationSchedule{Days: days}).DaysOn(now); got != privacy.DefaultRotationDays {
			t.Errorf("DaysOn with %d days = %d, want %d", days, got, privacy.DefaultRotationDays)
		}
	}
	if got := (privacy.RotationSchedule{Days: 1}).DaysOn(now); got != 1 {
		t.Errorf("DaysOn with 1 day = %d, want 1", got)
	}
	ip := net.ParseIP("198.51.100.1")
	daily := privacy.NewRotatingSaltHasher("k", 1)
	if daily.HashIP(ip, now) == daily.HashIP(ip, now.AddDate(0, 0, 1)) {
		t.Fatal("a 1-day period must rotate the salt every UTC day")
	}
}
