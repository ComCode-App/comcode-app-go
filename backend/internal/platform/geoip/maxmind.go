package geoip

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	geoip2 "github.com/oschwald/geoip2-golang"
)

// MaxMindProvider reads MaxMind MMDB city databases.
type MaxMindProvider struct {
	mu     sync.RWMutex
	reader *geoip2.Reader
	path   string
	ver    string
}

// NewMaxMind creates a new MaxMind provider from an MMDB file path.
func NewMaxMind(path string) (*MaxMindProvider, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("maxmind db not found at %q: %w", path, err)
	}

	reader, err := geoip2.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open maxmind db: %w", err)
	}

	ver := fmt.Sprintf("MaxMind-%s", time.Now().Format("2006.01.02"))
	if meta := reader.Metadata(); meta.BuildEpoch > 0 {
		ver = time.Unix(int64(meta.BuildEpoch), 0).UTC().Format("2006-01-02 15:04:05 UTC")
	}

	return &MaxMindProvider{
		reader: reader,
		path:   path,
		ver:    ver,
	}, nil
}

// Lookup queries the MMDB reader for city/country info.
func (p *MaxMindProvider) Lookup(ip net.IP) (*GeoLocation, error) {
	if p == nil {
		return nil, errors.New("nil maxmind provider")
	}

	p.mu.RLock()
	reader := p.reader
	p.mu.RUnlock()

	if reader == nil {
		return nil, errors.New("maxmind reader is closed")
	}

	if ip == nil {
		return nil, errors.New("nil ip")
	}

	record, err := reader.City(ip)
	if err != nil {
		return nil, err
	}

	loc := &GeoLocation{
		CountryCode: record.Country.IsoCode,
		CountryName: record.Country.Names["en"],
		City:        record.City.Names["en"],
		Timezone:    record.Location.TimeZone,
	}

	if record.Location.Latitude != 0 || record.Location.Longitude != 0 {
		lat := record.Location.Latitude
		lon := record.Location.Longitude
		loc.Latitude = &lat
		loc.Longitude = &lon
	}

	return loc, nil
}

// Resolve satisfies the GeoIPResolver interface.
func (p *MaxMindProvider) Resolve(ctx context.Context, ip net.IP) (*GeoLocation, error) {
	if p == nil {
		return nil, errors.New("nil maxmind provider")
	}
	return p.Lookup(ip)
}

// Version returns database build epoch/version string.
func (p *MaxMindProvider) Version() string {
	if p == nil {
		return ""
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.ver
}

// Close safely releases the MMDB reader handle.
func (p *MaxMindProvider) Close() error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.reader != nil {
		err := p.reader.Close()
		p.reader = nil
		return err
	}
	return nil
}

// NoopProvider returns empty geolocation when geoip is disabled.
type NoopProvider struct{}

func NewNoop() *NoopProvider {
	return &NoopProvider{}
}

func (n *NoopProvider) Lookup(ip net.IP) (*GeoLocation, error) {
	return &GeoLocation{}, nil
}

func (n *NoopProvider) Resolve(ctx context.Context, ip net.IP) (*GeoLocation, error) {
	return n.Lookup(ip)
}

func (n *NoopProvider) Version() string {
	return "noop"
}

func (n *NoopProvider) Close() error {
	return nil
}

// MockProvider provides configurable IP to location mapping for tests.
type MockProvider struct {
	mu        sync.RWMutex
	locations map[string]GeoLocation
	def       GeoLocation
}

func NewMock(defaultLoc GeoLocation) *MockProvider {
	return &MockProvider{
		locations: make(map[string]GeoLocation),
		def:       defaultLoc,
	}
}

func (m *MockProvider) Set(ipStr string, loc GeoLocation) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.locations[ipStr] = loc
}

func (m *MockProvider) Lookup(ip net.IP) (*GeoLocation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if loc, ok := m.locations[ip.String()]; ok {
		return &loc, nil
	}
	return &m.def, nil
}

func (m *MockProvider) Resolve(ctx context.Context, ip net.IP) (*GeoLocation, error) {
	return m.Lookup(ip)
}

func (m *MockProvider) Version() string {
	return "mock-1.0.0"
}

func (m *MockProvider) Close() error {
	return nil
}
