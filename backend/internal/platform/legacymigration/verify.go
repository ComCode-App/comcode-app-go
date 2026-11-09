package legacymigration

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"io"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/model"
)

// Verify exercises Service.Resolve, the very same method used by the public HTTP handler.
// It intentionally has no collector or Redis cache: validation is read-only and sees current DB state.
func Verify(ctx context.Context, repo Repository, resolver Resolver, reader *Reader, opts Options, report io.Writer) (int, error) {
	w := csv.NewWriter(report)
	if err := w.Write([]string{"hash", "expected", "actual", "status"}); err != nil {
		return 0, err
	}
	defer w.Flush()
	mismatches := 0
	for {
		row, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return mismatches, err
		}
		expected, expectErr := repo.Expected(ctx, row, opts)
		actual, status := "", "ok"
		res, resolveErr := resolver.Resolve(ctx, row.LegacyHash)
		if res != nil {
			actual = res.TargetURL
		}
		switch {
		case expectErr != nil:
			status = "invalid_expected: " + expectErr.Error()
		case resolveErr != nil:
			status = "resolver_error: " + resolveErr.Error()
		case res == nil || res.RouteType != model.RouteTypeLegacyRoute || !res.IsActive || res.IsLocked || res.RequiresPasscode:
			status = "inactive_or_not_legacy"
		case !matchesTarget(row, res):
			status = "target_type_or_profile_mismatch"
		case expected == "" || actual != expected:
			status = "target_mismatch"
		}
		if status != "ok" {
			mismatches++
		}
		if err := w.Write([]string{row.LegacyHash, expected, actual, status}); err != nil {
			return mismatches, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return mismatches, err
	}
	if mismatches > 0 {
		return mismatches, fmt.Errorf("verification failed: %d mismatches", mismatches)
	}
	return 0, nil
}

func matchesTarget(row Row, res *model.RouteResolution) bool {
	switch row.TargetType {
	case "url":
		return res.TargetType == model.TargetTypeDirectURL
	case "profile", "vcard":
		kind := model.TargetTypeProfile
		if row.TargetType == "vcard" {
			kind = model.TargetTypeVCardDownload
		}
		id, err := uuid.Parse(row.TargetValue)
		return err == nil && res.TargetType == kind && res.ProfileUUID != nil && *res.ProfileUUID == id
	default:
		return true
	}
}
