package geoip_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/geoip"
)

func TestMockProvider_And_MemoryCache(t *testing.T) {
	mock := geoip.NewMock(geoip.GeoLocation{
		CountryCode: "US",
		CountryName: "United States",
		City:        "Ashburn",
		Timezone:    "America/New_York",
	})

	ipTR := net.ParseIP("212.252.128.1")
	mock.Set(ipTR.String(), geoip.GeoLocation{
		CountryCode: "TR",
		CountryName: "Turkey",
		City:        "Istanbul",
		Timezone:    "Europe/Istanbul",
	})

	ctx := context.Background()

	t.Run("mock resolution", func(t *testing.T) {
		locTR, err := mock.Lookup(ipTR)
		if err != nil {
			t.Fatalf("lookup failed: %v", err)
		}
		if locTR.CountryCode != "TR" || locTR.City != "Istanbul" {
			t.Fatalf("unexpected location: %+v", locTR)
		}

		ipUS := net.ParseIP("8.8.8.8")
		locUS, err := mock.Lookup(ipUS)
		if err != nil {
			t.Fatalf("lookup failed: %v", err)
		}
		if locUS.CountryCode != "US" {
			t.Fatalf("expected US default, got %s", locUS.CountryCode)
		}
	})

	t.Run("memory cache behavior", func(t *testing.T) {
		cache := geoip.NewMemoryCache(10, 50*time.Millisecond)
		loc := &geoip.GeoLocation{CountryCode: "DE", City: "Frankfurt"}

		cache.Set("1.2.3.4", loc, 50*time.Millisecond)

		cached, found := cache.Get("1.2.3.4")
		if !found || cached.CountryCode != "DE" {
			t.Fatalf("expected cache hit, got %v", found)
		}

		time.Sleep(60 * time.Millisecond)
		_, foundAfterExpiry := cache.Get("1.2.3.4")
		if foundAfterExpiry {
			t.Fatal("expected cache entry to expire")
		}
	})

	t.Run("noop provider", func(t *testing.T) {
		noop := geoip.NewNoop()
		loc, err := noop.Lookup(ipTR)
		if err != nil {
			t.Fatalf("expected no error from noop, got: %v", err)
		}
		if loc.CountryCode != "" {
			t.Fatalf("expected empty country code from noop, got: %s", loc.CountryCode)
		}
	})
	t.Run("real maxmind file resolution if present", func(t *testing.T) {
		paths := []string{
			"data/geoip/GeoLite2-City.mmdb",
			"../../data/geoip/GeoLite2-City.mmdb",
			"../../../data/geoip/GeoLite2-City.mmdb",
			"../../../backend/data/geoip/GeoLite2-City.mmdb",
		}
		var mm geoip.GeoIPDatabase
		for _, p := range paths {
			prov, err := geoip.NewMaxMind(p)
			if err == nil {
				mm = prov
				break
			}
		}
		if mm == nil {
			t.Skip("skipping real maxmind test, file not found locally")
		}
		defer func() { _ = mm.Close() }()

		loc, err := mm.Lookup(net.ParseIP("8.8.8.8"))
		if err != nil {
			t.Fatalf("real maxmind lookup failed: %v", err)
		}
		if loc.CountryCode == "" {
			t.Fatalf("expected country code for 8.8.8.8, got empty")
		}
		t.Logf("Successfully resolved 8.8.8.8 -> Country: %s (%s), City: %s, Timezone: %s",
			loc.CountryCode, loc.CountryName, loc.City, loc.Timezone)
	})
	_ = ctx
}
