package privacy

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"time"
)

// Rotation period bounds and default in days (analytics_settings.ip_hash_salt_rotation_days).
const (
	MinRotationDays     = 1
	MaxRotationDays     = 30
	DefaultRotationDays = 7
)

// Hasher computes privacy-safe salted hashes for client IP addresses.
type Hasher interface {
	HashIP(ip net.IP, timestamp time.Time) string
}

// RotationSchedule is the salt rotation setting with its last change (TEC-936). A new period
// applies from the UTC day after ChangedOn; on ChangedOn and before, PreviousDays stays in force,
// so every hash of one day uses the same salt however often the setting changes that day.
type RotationSchedule struct {
	Days         int
	PreviousDays int
	// ChangedOn is the UTC date of the last change; zero when it never changed.
	ChangedOn time.Time
}

// DaysOn returns the rotation period in force on t's UTC day, clamped to 1-30 (7 when unset).
func (s RotationSchedule) DaysOn(t time.Time) int {
	days := s.Days
	if !s.ChangedOn.IsZero() && s.PreviousDays > 0 {
		day := utcDay(t)
		if !day.After(utcDay(s.ChangedOn)) {
			days = s.PreviousDays
		}
	}
	return clampDays(days)
}

// ScheduleSource returns the current rotation schedule; it is called on every hash, so it should
// be cached (AnalyticsService.SaltRotationSchedule is).
type ScheduleSource func() RotationSchedule

// RotatingSaltHasher rotates salt periodically so IP addresses cannot be permanently correlated.
type RotatingSaltHasher struct {
	secretKey    []byte
	rotationDays int
	schedule     ScheduleSource
}

// NewRotatingSaltHasher creates a hasher with a fixed rotation period (out of range = 7 days).
func NewRotatingSaltHasher(secretKey string, rotationDays int) *RotatingSaltHasher {
	if secretKey == "" {
		secretKey = "analytics-default-privacy-salt-key"
	}
	return &RotatingSaltHasher{
		secretKey:    []byte(secretKey),
		rotationDays: clampDays(rotationDays),
	}
}

// WithSchedule makes the hasher read its rotation period from the analytics settings.
func (h *RotatingSaltHasher) WithSchedule(src ScheduleSource) *RotatingSaltHasher {
	h.schedule = src
	return h
}

// HashIP hashes an IP address using HMAC-SHA256 with a period-derived salt.
func (h *RotatingSaltHasher) HashIP(ip net.IP, timestamp time.Time) string {
	if ip == nil {
		return ""
	}
	if timestamp.IsZero() {
		timestamp = time.Now().UTC()
	}

	days := h.rotationDays
	if h.schedule != nil {
		days = h.schedule().DaysOn(timestamp)
	}
	// Periods are whole UTC days counted from the Unix epoch, so a period never splits a day.
	periodIndex := timestamp.Unix() / int64(days*86400)
	periodSalt := fmt.Sprintf("period-%d", periodIndex)

	mac := hmac.New(sha256.New, append(append([]byte{}, h.secretKey...), []byte(periodSalt)...))
	mac.Write([]byte(ip.String()))
	return hex.EncodeToString(mac.Sum(nil))[:32] // 32-char hex string
}

func clampDays(days int) int {
	if days < MinRotationDays || days > MaxRotationDays {
		return DefaultRotationDays
	}
	return days
}

func utcDay(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
