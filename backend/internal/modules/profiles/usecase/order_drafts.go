package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/model"
	"github.com/google/uuid"
)

// OrderDraftInput is one recipient of a company bulk order (TEC-1317).
type OrderDraftInput struct {
	Name       string
	Title      string
	Department string
	Email      string
	Phone      string
}

// CreateOrderDraft creates a recipient's profile in a company account for a bulk order awaiting
// approval. Plan quota is checked when the order is approved, not here. The public page starts
// closed ("locked") and opens once the order is paid (PublishOrderDrafts).
func (s *Service) CreateOrderDraft(ctx context.Context, accountID, creatorUserID int64, creatorUUID uuid.UUID, in OrderDraftInput) (uuid.UUID, error) {
	first, last := splitName(in.Name)
	if first == "" && last == "" {
		return uuid.Nil, fmt.Errorf("%w: recipient name is required", ErrInvalidRequest)
	}
	up := model.UpsertProfileInput{
		PrimaryLocale: "tr",
		Translations: []model.UpsertTranslationInput{{
			Locale: "tr", FirstName: first, LastName: last,
			Title: strings.TrimSpace(in.Title), Department: strings.TrimSpace(in.Department), IsDefault: true,
		}},
	}
	if v := strings.TrimSpace(in.Email); v != "" {
		up.ContactDetails = append(up.ContactDetails, model.CreateContactDetailInput{Category: "email", Type: "work", Value: v, IsPrimary: true})
	}
	if v := strings.TrimSpace(in.Phone); v != "" {
		up.ContactDetails = append(up.ContactDetails, model.CreateContactDetailInput{Category: "phone", Type: "work", Value: v, IsPrimary: true, DisplayOrder: 1})
	}
	detail, err := s.repo.Create(ctx, accountID, creatorUserID, creatorUUID, up, calculateCompletionRate(up))
	if err != nil {
		return uuid.Nil, err
	}
	if s.pages != nil {
		// Best effort like Create: without a route the page stays closed anyway.
		_ = s.ensureUsernameStatus(ctx, accountID, detail, "locked")
	}
	s.indexProfile(ctx, detail.UUID)
	return detail.UUID, nil
}

// DeleteOrderDraft deletes a draft profile an order created, when the order is rejected.
func (s *Service) DeleteOrderDraft(ctx context.Context, accountID int64, profileUUID uuid.UUID) error {
	detail, err := s.repo.GetByUUID(ctx, accountID, profileUUID)
	if err != nil {
		return mapRepoErr(err)
	}
	return s.deleteProfile(ctx, accountID, detail)
}

// InvalidateProfiles drops cached public pages after their routes changed outside this service.
func (s *Service) InvalidateProfiles(ctx context.Context, ids []uuid.UUID) {
	for _, id := range ids {
		s.routeCache.InvalidateProfile(ctx, id)
	}
}

func splitName(name string) (string, string) {
	parts := strings.Fields(name)
	switch len(parts) {
	case 0:
		return "", ""
	case 1:
		return parts[0], ""
	default:
		return strings.Join(parts[:len(parts)-1], " "), parts[len(parts)-1]
	}
}
