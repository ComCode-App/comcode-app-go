package config

import (
	"fmt"
	"strconv"
	"strings"
)

// BillingConfig controls subscription retry offsets and entitlement grace.
type BillingConfig struct {
	// RenewalEnabled gates the recurring-charge sweep. It is off by default so that deploying
	// this code never starts charging saved cards until an operator opts in.
	RenewalEnabled bool
	RetryDays      []int
	GraceDays      int
}

func loadBilling() (BillingConfig, error) {
	c := BillingConfig{
		RenewalEnabled: getBool("BILLING_RENEWAL_ENABLED", false),
		RetryDays:      []int{},
		GraceDays:      getInt("BILLING_GRACE_DAYS", 14),
	}
	raw := getEnv("BILLING_RETRY_DAYS", "1,3,7")
	previous := 0
	for _, part := range strings.Split(raw, ",") {
		day, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || day <= previous || day >= c.GraceDays {
			return c, fmt.Errorf("BILLING_RETRY_DAYS must increase and precede BILLING_GRACE_DAYS")
		}
		c.RetryDays = append(c.RetryDays, day)
		previous = day
	}
	if c.GraceDays < 1 || c.GraceDays > 365 {
		return c, fmt.Errorf("BILLING_GRACE_DAYS must be between 1 and 365")
	}
	return c, nil
}
