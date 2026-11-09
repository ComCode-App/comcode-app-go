package usecase

import (
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
)

func TestCalendarPeriodEnd(t *testing.T) {
	for _, tt := range []struct {
		start, want string
		interval    db.BillingInterval
	}{
		{start: "2027-01-31T12:15:00Z", want: "2027-02-28T12:15:00Z", interval: db.BillingIntervalMonth},
		{start: "2028-01-31T12:15:00Z", want: "2028-02-29T12:15:00Z", interval: db.BillingIntervalMonth},
		{start: "2028-02-29T12:15:00Z", want: "2029-02-28T12:15:00Z", interval: db.BillingIntervalYear},
	} {
		t.Run(tt.start, func(t *testing.T) {
			start, err := time.Parse(time.RFC3339, tt.start)
			if err != nil {
				t.Fatal(err)
			}
			if got := advancePeriod(start, tt.interval).Format(time.RFC3339); got != tt.want {
				t.Fatalf("%s want %s", got, tt.want)
			}
		})
	}
}
func TestProrationRoundingAndOverflow(t *testing.T) {
	for _, tt := range []struct {
		difference       int64
		remaining, total time.Duration
		want             int64
	}{
		{difference: 10001, remaining: time.Hour, total: 3 * time.Hour, want: 3334},
		{difference: 100000000000, remaining: 15 * 24 * time.Hour, total: 30 * 24 * time.Hour, want: 50000000000},
		{difference: 100, remaining: 0, total: time.Hour, want: 0},
	} {
		if got := proratedDifference(tt.difference, tt.remaining, tt.total); got != tt.want {
			t.Fatalf("%d want %d", got, tt.want)
		}
	}
}
