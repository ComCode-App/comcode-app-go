package usecase

import (
	"context"
	"log/slog"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/completion"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/model"
)

// Verification status values (profileverifications module) without importing it.
const (
	verificationNone = "none"
)

// Identity-change reasons recorded when a verified profile loses its badge (TEC-549).
const (
	reasonDisplayNameChanged  = "display_name_changed"
	reasonOrganizationChanged = "organization_changed"
)

// Verifications is the verified profile badge (profileverifications module, TEC-549).
type Verifications interface {
	// Status is none | pending | verified | rejected for a profile of the account.
	Status(ctx context.Context, accountID int64, profileUUID uuid.UUID) (string, error)
	// Verified reports whether a profile holds the badge (visitor API).
	Verified(ctx context.Context, profileID int64) (bool, error)
	// RevokeForIdentityChange drops the badge of a verified profile and re-opens review; it is a
	// no-op for profiles that are not verified.
	RevokeForIdentityChange(ctx context.Context, accountID int64, profileUUID uuid.UUID, actorUserID int64, reasons []string) error
}

// SetVerifications wires the verified badge. Without it every profile reads as "none".
func (s *Service) SetVerifications(v Verifications) { s.verifications = v }

// completionRateWriter is implemented by the Postgres repository; avatar and cover uploads use it
// to keep the stored completion_rate in step with the media they change.
type completionRateWriter interface {
	SetCompletionRate(ctx context.Context, accountID int64, profileUUID uuid.UUID, rate int) error
}

// decorate fills the derived fields of a profile response: the completion score and suggestions
// (profiles/completion, the single source of the formula) and the verification status.
func (s *Service) decorate(ctx context.Context, accountID int64, d *model.ProfileDetail) {
	if d == nil {
		return
	}
	res := completion.Evaluate(completion.FromDetail(d))
	d.CompletionRate = res.Rate
	d.CompletionSuggestions = res.Suggestions
	d.VerificationStatus = verificationNone
	if s.verifications != nil {
		status, err := s.verifications.Status(ctx, accountID, d.UUID)
		if err != nil {
			slog.Default().Warn("profile_verification_status_failed", "profile_uuid", d.UUID.String(), "error", err)
			return
		}
		d.VerificationStatus = status
	}
}

// syncCompletionRate stores the score of d when it drifted (media uploads change the avatar
// outside the upsert payload).
func (s *Service) syncCompletionRate(ctx context.Context, accountID int64, d *model.ProfileDetail, stored int) {
	if d == nil || d.CompletionRate == stored {
		return
	}
	if w, ok := s.repo.(completionRateWriter); ok {
		if err := w.SetCompletionRate(ctx, accountID, d.UUID, d.CompletionRate); err != nil {
			slog.Default().Warn("profile_completion_rate_sync_failed", "profile_uuid", d.UUID.String(), "error", err)
		}
	}
}

// identityChanges compares the display names and organizations of the stored profile with an
// update payload. Adding a translation with the same name or organization is not a change; any
// new, edited or removed value is. Title, bio and contacts never revoke the badge.
func identityChanges(before *model.ProfileDetail, in model.UpsertProfileInput) []string {
	var oldNames, oldOrgs, newNames, newOrgs []string
	for _, t := range before.Translations {
		oldNames = appendNorm(oldNames, t.FirstName+" "+t.MiddleName+" "+t.LastName)
		oldOrgs = appendNorm(oldOrgs, t.CompanyName)
	}
	for _, t := range in.Translations {
		newNames = appendNorm(newNames, t.FirstName+" "+t.MiddleName+" "+t.LastName)
		newOrgs = appendNorm(newOrgs, t.CompanyName)
	}
	var reasons []string
	if !sameSet(oldNames, newNames) {
		reasons = append(reasons, reasonDisplayNameChanged)
	}
	if !sameSet(oldOrgs, newOrgs) {
		reasons = append(reasons, reasonOrganizationChanged)
	}
	return reasons
}

// appendNorm adds v case- and space-normalized, skipping blanks and duplicates. Turkish dotted
// and dotless i fold together, so "YILMAZ" and "Yılmaz" are the same name.
func appendNorm(set []string, v string) []string {
	v = strings.Map(func(r rune) rune {
		switch r {
		case 'ı':
			return 'i'
		case '\u0307': // combining dot left by lower-casing İ
			return -1
		}
		return r
	}, strings.ToLower(strings.Join(strings.Fields(v), " ")))
	if v == "" || slices.Contains(set, v) {
		return set
	}
	return append(set, v)
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for _, v := range a {
		if !slices.Contains(b, v) {
			return false
		}
	}
	return true
}
