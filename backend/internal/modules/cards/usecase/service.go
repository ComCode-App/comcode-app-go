package usecase

import (
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/routecache"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/cards/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/cards/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/searchengine"
)

var (
	ErrCardNotFound       = errors.New("card not found")
	ErrCardLocked         = errors.New("card is locked")
	ErrInvalidActivation = errors.New("invalid activation code or card already activated")
	ErrInvalidProfile    = errors.New("invalid profile selected")
	ErrUnauthorizedCard   = errors.New("unauthorized to access card")
	ErrForbidden          = errors.New("account role cannot modify cards")
)

// Cards belong to the active account. Every member can read them; creating, editing, locking,
// deleting and activating need an owner or admin role (docs/TENANCY_AND_ACCESS.md).

type Service struct {
	repo        repository.Repository
	frontendURL string
	quotas      entitlement.Quotas
	cache       *routecache.Cache
	search      searchengine.Enqueuer
}

func NewService(repo repository.Repository, frontendURL string) *Service {
	return &Service{
		repo:        repo,
		frontendURL: strings.TrimRight(frontendURL, "/"),
	}
}

// SetQuotas wires plan quota checks. Without it, quota-guarded creates are denied.
func (s *Service) SetQuotas(q entitlement.Quotas) { s.quotas = q }

// SetRouteCache lets card writes drop the resolver cache entries of the card's routes (TEC-557).
func (s *Service) SetRouteCache(c *routecache.Cache) { s.cache = c }

// SetSearchIndexer keeps cards in the account-scoped Cmd+K index (TEC-951).
func (s *Service) SetSearchIndexer(e searchengine.Enqueuer) { s.search = e }

func (s *Service) indexed(ctx context.Context, card *model.CardDetail, err error) (*model.CardDetail, error) {
	if err == nil && card != nil && s.search != nil {
		s.search.EnqueueUpsert(ctx, "cards", card.UUID.String())
	}
	return card, err
}

func (s *Service) List(ctx context.Context, account authctx.Account, q *string, isActive *bool, limit, offset int32) ([]model.CardSummary, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return s.repo.ListAccountCards(ctx, account.ID, q, isActive, limit, offset)
}

func (s *Service) Get(ctx context.Context, id uuid.UUID, account authctx.Account) (*model.CardDetail, error) {
	card, err := s.repo.GetAccountCardByUUID(ctx, id, account.ID)
	if err != nil {
		return nil, err
	}
	if card == nil {
		return nil, ErrCardNotFound
	}
	return card, nil
}

func (s *Service) Create(ctx context.Context, account authctx.Account, creatorUserID int64, input model.UpsertCardInput) (*model.CardDetail, error) {
	if !account.CanWrite() {
		return nil, ErrForbidden
	}
	if err := entitlement.Guard(ctx, s.quotas, account.ID, entitlement.FeatureCardsMax); err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.Name) == "" {
		input.Name = "ComCode Akıllı NFC Kart"
	}

	if input.TargetType == "" {
		input.TargetType = "profile"
	}

	var profileID *int64
	if input.ProfileUUID != nil {
		pid, err := s.repo.GetProfileIDByUUID(ctx, *input.ProfileUUID, account.ID)
		if err != nil || pid == nil {
			return nil, ErrInvalidProfile
		}
		profileID = pid
	}

	smartCode, err := s.generateUniqueSmartCode(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to generate unique smart code: %w", err)
	}

	card, err := s.repo.CreateCard(ctx, account.ID, creatorUserID, smartCode, input, profileID)
	return s.indexed(ctx, card, err)
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, account authctx.Account, input model.UpsertCardInput) (*model.CardDetail, error) {
	if _, err := s.getWritable(ctx, id, account); err != nil {
		return nil, err
	}

	var profileID *int64
	setProfileID := false
	if input.ProfileUUID != nil {
		pid, err := s.repo.GetProfileIDByUUID(ctx, *input.ProfileUUID, account.ID)
		if err != nil || pid == nil {
			return nil, ErrInvalidProfile
		}
		profileID = pid
		setProfileID = true
	} else if input.TargetType != "" && input.TargetType != "profile" {
		profileID = nil
		setProfileID = true
	}

	card, err := s.repo.UpdateCard(ctx, id, account.ID, input, profileID, setProfileID)
	if err == nil {
		s.cache.InvalidateCard(ctx, id)
	}
	return s.indexed(ctx, card, err)
}

func (s *Service) ToggleLock(ctx context.Context, id uuid.UUID, account authctx.Account) (bool, error) {
	if _, err := s.getWritable(ctx, id, account); err != nil {
		return false, err
	}
	locked, err := s.repo.ToggleCardLock(ctx, id, account.ID)
	if err == nil {
		s.cache.InvalidateCard(ctx, id)
	}
	return locked, err
}

// getWritable loads a card of the account (ErrCardNotFound otherwise) and checks the write role.
func (s *Service) getWritable(ctx context.Context, id uuid.UUID, account authctx.Account) (*model.CardDetail, error) {
	card, err := s.Get(ctx, id, account)
	if err != nil {
		return nil, err
	}
	if !account.CanWrite() {
		return nil, ErrForbidden
	}
	return card, nil
}

// Activate claims an unactivated card into the caller's active account and activates its route.
func (s *Service) Activate(ctx context.Context, account authctx.Account, userID int64, input model.ActivateCardInput) (*model.CardDetail, error) {
	if !account.CanWrite() {
		return nil, ErrForbidden
	}
	code := strings.TrimSpace(strings.ToUpper(input.ActivationCode))
	if code == "" {
		return nil, ErrInvalidActivation
	}

	existing, err := s.repo.GetCardByActivationCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, ErrInvalidActivation
	}
	if err := entitlement.Guard(ctx, s.quotas, account.ID, entitlement.FeatureCardsMax); err != nil {
		return nil, err
	}

	var profileID *int64
	if input.ProfileUUID != nil {
		pid, err := s.repo.GetProfileIDByUUID(ctx, *input.ProfileUUID, account.ID)
		if err != nil || pid == nil {
			return nil, ErrInvalidProfile
		}
		profileID = pid
	}

	var name *string
	if strings.TrimSpace(input.Name) != "" {
		n := strings.TrimSpace(input.Name)
		name = &n
	}

	card, err := s.repo.ActivateCardByCode(ctx, code, account.ID, userID, name, profileID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Raced with another activation of the same code.
			return nil, ErrInvalidActivation
		}
		return nil, err
	}
	s.cache.InvalidateCard(ctx, card.UUID)
	// Activation moves the card into this account; reindex so its account_id follows.
	return s.indexed(ctx, card, nil)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID, account authctx.Account) error {
	if _, err := s.getWritable(ctx, id, account); err != nil {
		return err
	}
	if err := s.repo.DeleteCard(ctx, id, account.ID); err != nil {
		return err
	}
	s.cache.InvalidateCard(ctx, id)
	if s.search != nil {
		s.search.EnqueueDelete(ctx, "cards", id.String())
	}
	return nil
}

func (s *Service) ResolveSmartLink(ctx context.Context, code string) (*model.SmartLinkResolution, error) {
	cleanCode := strings.TrimSpace(code)
	card, err := s.repo.GetCardBySmartCode(ctx, cleanCode)
	if err != nil {
		return nil, err
	}
	if card == nil {
		return nil, ErrCardNotFound
	}

	res := &model.SmartLinkResolution{
		SmartCode:          card.SmartCode,
		CardName:           card.Name,
		TargetType:         card.TargetType,
		DirectURL:          card.DirectURL,
		CustomData:         card.CustomData,
		IsLocked:           card.IsLocked,
		IsActive:           card.IsActive,
		LeadCaptureEnabled: card.LeadCaptureEnabled,
		RequiresPasscode:   card.HasPasscode,
	}

	if card.ProfileUUID != nil {
		res.ProfileUUID = card.ProfileUUID
		res.ProfileAvatarURL = card.ProfileAvatarURL
	}

	switch card.TargetType {
	case "direct_url":
		res.TargetURL = card.DirectURL
	case "profile":
		if card.ProfileUUID != nil {
			res.TargetURL = fmt.Sprintf("%s/profiles/%s", s.frontendURL, card.ProfileUUID.String())
		}
	case "vcard_download":
		if card.ProfileUUID != nil {
			res.TargetURL = fmt.Sprintf("%s/v1/profiles/%s/vcard", s.frontendURL, card.ProfileUUID.String())
		}
	case "social_hub":
		// Custom data holds the links
		if card.ProfileUUID != nil {
			res.TargetURL = fmt.Sprintf("%s/profiles/%s", s.frontendURL, card.ProfileUUID.String())
		}
	default:
		if card.ProfileUUID != nil {
			res.TargetURL = fmt.Sprintf("%s/profiles/%s", s.frontendURL, card.ProfileUUID.String())
		} else if card.DirectURL != "" {
			res.TargetURL = card.DirectURL
		}
	}

	return res, nil
}

func (s *Service) generateUniqueSmartCode(ctx context.Context) (string, error) {
	const charset = "abcdefghjkmnpqrstuvwxyz23456789"
	for attempts := 0; attempts < 10; attempts++ {
		code := "cc-"
		for i := 0; i < 6; i++ {
			n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
			if err != nil {
				return "", err
			}
			code += string(charset[n.Int64()])
		}

		exists, err := s.repo.CheckSmartCodeExists(ctx, code, nil)
		if err != nil {
			return "", err
		}
		if !exists {
			return code, nil
		}
	}
	return "", errors.New("failed to generate a unique smart code after multiple attempts")
}
