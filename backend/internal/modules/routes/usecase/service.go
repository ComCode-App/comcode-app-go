package usecase

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/activity"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/domain"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/ingestion"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/publicpath"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/routecache"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/storage"
	"github.com/google/uuid"
)

var (
	ErrRouteNotFound        = errors.New("route not found")
	ErrRoutePathReserved    = errors.New("route path is reserved by system")
	ErrRoutePathConflict    = errors.New("route path already exists")
	ErrRouteImmutable       = errors.New("route is immutable and cannot be altered or deleted")
	ErrInvalidRoutePath     = errors.New("invalid route path format")
	ErrInvalidProfileTarget = errors.New("invalid profile target specified")
	ErrUnauthorizedRoute    = errors.New("unauthorized to manage route")
	ErrRouteFieldForbidden  = errors.New("route field cannot be changed by owner")
	ErrRouteNotLockable     = errors.New("only active or locked routes can be toggled")
	ErrForbidden            = errors.New("account role cannot modify routes")
)

// reserved first-level paths live in platform/publicpath (shared with profile usernames).
func isReservedKeyword(segment string) bool { return publicpath.IsReserved(segment) }

var pathRegex = regexp.MustCompile(`^[a-z0-9]+([a-z0-9_\-\.\/]*[a-z0-9]+)?$`)

type Service struct {
	qrStore     repository.QRStore
	qrStorage   storage.Driver
	repo        repository.Repository
	frontendURL string
	collector   *ingestion.Collector
	quotas      entitlement.Quotas
	features    entitlement.Features
	cache       *routecache.Cache
	activity    *activity.Recorder
	webhooks    Webhooks
	scanEmits   scanDebounce
}

// scanDebounce rate-limits card.scanned per route: a bot or link preview hitting one route in a
// burst produces one webhook, not hundreds. Per process; analytics still counts every hit.
type scanDebounce struct {
	mu   sync.Mutex
	last map[uuid.UUID]time.Time
}

const scanWebhookWindow = 10 * time.Second

func (d *scanDebounce) allow(id uuid.UUID, now time.Time) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.last == nil {
		d.last = make(map[uuid.UUID]time.Time)
	}
	if t, ok := d.last[id]; ok && now.Sub(t) < scanWebhookWindow {
		return false
	}
	if len(d.last) > 10000 {
		for k, t := range d.last {
			if now.Sub(t) >= scanWebhookWindow {
				delete(d.last, k)
			}
		}
	}
	d.last[id] = now
	return true
}

// Webhooks fans a domain event out to the account's webhook endpoints (TEC-603).
type Webhooks interface {
	Dispatch(ctx context.Context, accountID int64, eventName string, data map[string]any)
}

// SetWebhooks enables card.scanned webhooks.
func (s *Service) SetWebhooks(w Webhooks) { s.webhooks = w }

// emitScanned sends card.scanned for a resolved route (card, smart link or legacy hash) of an
// account. Visitor details stay in analytics; the webhook carries what was scanned and how.
func (s *Service) emitScanned(ctx context.Context, route *model.RouteDetail, source string) {
	if s.webhooks == nil || route == nil || route.AccountID == nil {
		return
	}
	if !s.scanEmits.allow(route.UUID, time.Now()) {
		return
	}
	if source == "" {
		source = "link"
	}
	data := map[string]any{
		"route_uuid": route.UUID.String(), "path": "/" + route.Path, "route_type": route.RouteType,
		"target_type": route.TargetType, "source": source, "scanned_at": time.Now().UTC().Format(time.RFC3339),
	}
	if route.CardUUID != nil {
		data["card_uuid"], data["card_name"] = route.CardUUID.String(), route.CardName
	}
	if route.ProfileUUID != nil {
		data["profile_uuid"], data["profile_username"] = route.ProfileUUID.String(), route.ProfileUsername
	}
	s.webhooks.Dispatch(ctx, *route.AccountID, "card.scanned", data)
}

func NewService(repo repository.Repository, frontendURL string, collector *ingestion.Collector) *Service {
	return &Service{
		repo:        repo,
		frontendURL: strings.TrimRight(frontendURL, "/"),
		collector:   collector,
	}
}

// SetQuotas wires plan quota checks. Without it, quota-guarded creates are denied.
func (s *Service) SetQuotas(q entitlement.Quotas) { s.quotas = q }

// SetFeatures wires the design.premium check for smart link themes (TEC-947). Without it
// premium themes cannot be picked (fail-closed).
func (s *Service) SetFeatures(f entitlement.Features) { s.features = f }

// SetCache enables the Redis resolver cache (TEC-557). Without it every resolve reads Postgres.
func (s *Service) SetCache(c *routecache.Cache) { s.cache = c }

// SetActivity records path changes in the activity log (TEC-558).
func (s *Service) SetActivity(r *activity.Recorder) { s.activity = r }

func (s *Service) NormalizePath(rawPath string) string {
	clean := strings.TrimSpace(rawPath)
	clean = strings.Trim(clean, "/")
	return strings.ToLower(clean)
}

func (s *Service) IsReservedPath(path string) bool {
	normalized := s.NormalizePath(path)
	if !strings.Contains(normalized, "/") {
		return isReservedKeyword(normalized)
	}

	parts := strings.Split(normalized, "/")
	// If it starts with standard sub-namespaces like s/ or c/, check the sub-slug
	if parts[0] == "s" || parts[0] == "c" || parts[0] == "r" || parts[0] == "u" {
		if len(parts) > 1 {
			return isReservedKeyword(parts[1])
		}
		return true
	}

	// For any other path, check root prefix
	return isReservedKeyword(parts[0])
}

func (s *Service) ValidatePathFormat(path string) error {
	normalized := s.NormalizePath(path)
	if len(normalized) < 2 || len(normalized) > 120 {
		return fmt.Errorf("%w: path length must be between 2 and 120 characters", ErrInvalidRoutePath)
	}
	if !pathRegex.MatchString(normalized) {
		return fmt.Errorf("%w: path contains invalid characters", ErrInvalidRoutePath)
	}
	if strings.Contains(normalized, "//") {
		return fmt.Errorf("%w: path cannot contain consecutive slashes", ErrInvalidRoutePath)
	}
	return nil
}

func (s *Service) Resolve(ctx context.Context, rawPath string, httpContexts ...ingestion.HTTPContext) (*model.RouteResolution, error) {
	norm := s.NormalizePath(rawPath)
	if norm == "" {
		return nil, ErrRouteNotFound
	}

	route, err := s.lookupPath(ctx, norm)
	if err != nil {
		return nil, err
	}
	if route == nil {
		// Fallback: If single slug provided (e.g. "code"), try checking "c/{slug}" and "s/{slug}"
		if !strings.Contains(norm, "/") {
			if r, _ := s.lookupPath(ctx, "c/"+norm); r != nil {
				route = r
			} else if r, _ := s.lookupPath(ctx, "s/"+norm); r != nil {
				route = r
			}
		}
	}

	if route == nil {
		return nil, ErrRouteNotFound
	}

	return s.resolve(ctx, route, httpContexts...)
}

// ResolveLegacy supplies legacy redirects to the existing public-profile endpoint.
// Ordinary profiles return nil without ingesting a route scan.
func (s *Service) ResolveLegacy(ctx context.Context, rawPath string, httpContexts ...ingestion.HTTPContext) (*model.RouteResolution, error) {
	norm := s.NormalizePath(rawPath)
	if norm == "" || strings.Contains(norm, "/") {
		return nil, nil
	}
	route, err := s.lookupPath(ctx, norm)
	if err != nil {
		return nil, err
	}
	if route == nil || route.RouteType != model.RouteTypeLegacyRoute {
		return nil, nil
	}
	// A card whose username is its own printed hash renders in place (TEC-956 option E);
	// redirecting would send /{hash} to itself.
	if strings.EqualFold(route.ProfileUsername, norm) {
		return nil, nil
	}
	return s.resolve(ctx, route, httpContexts...)
}

func (s *Service) resolve(ctx context.Context, route *model.RouteDetail, httpContexts ...ingestion.HTTPContext) (*model.RouteResolution, error) {
	if len(httpContexts) > 0 && httpContexts[0].Source == "qr" {
		s.recordQRScan(ctx, route, httpContexts[0])
	}

	// Ingest analytics event asynchronously into Analytics Event Store
	if s.collector != nil {
		var hCtx ingestion.HTTPContext
		if len(httpContexts) > 0 {
			hCtx = httpContexts[0]
		}

		props := map[string]any{
			"path":        route.Path,
			"route_type":  route.RouteType,
			"target_type": route.TargetType,
			"direct_url":  route.DirectURL,
			"is_locked":   route.Status == model.RouteStatusLocked || route.CardIsLocked,
			"is_active":   route.Status == model.RouteStatusActive && (route.CardID == nil || route.CardIsActive),
			"source":      "resolver",
		}
		if route.CardUUID != nil {
			props["card_uuid"] = route.CardUUID.String()
			props["card_name"] = route.CardName
		}
		if route.ProfileUUID != nil {
			props["profile_uuid"] = route.ProfileUUID.String()
		}
		if hCtx.Source == "qr" {
			// Already counted as a QR scan; keeps it out of the NFC/link source counts (TEC-953).
			props[domain.EntrySourceProperty] = domain.SourceQR
		}

		go func(rDetail *model.RouteDetail, cProps map[string]any, hc ingestion.HTTPContext) {
			s.emitScanned(context.Background(), rDetail, hc.Source)
			_, _ = s.collector.Ingest(context.Background(), ingestion.IngestRequest{
				Type:          "route.scan",
				SchemaVersion: 1,
				Subject: domain.EntityReference{
					Type: "route",
					ID:   rDetail.UUID.String(),
				},
				Properties: cProps,
			}, hc, true, "async")
		}(route, props, hCtx)
	}

	return s.buildResolution(route, false), nil
}

// buildResolution is the visitor view of a route. A smart link passcode hides where the link
// goes until the visitor unlocks it (POST /v1/s/{slug}/unlock); unlocked skips that.
func (s *Service) buildResolution(route *model.RouteDetail, unlocked bool) *model.RouteResolution {
	linkPasscode := model.PasscodeHash(route.PresentationConfig) != ""
	resolution := &model.RouteResolution{
		RouteUUID:          route.UUID,
		Path:               route.Path,
		RouteType:          route.RouteType,
		Status:             route.Status,
		TargetType:         route.TargetType,
		DirectURL:          route.DirectURL,
		ProfileUUID:        route.ProfileUUID,
		ProfileAvatarURL:   route.ProfileAvatarURL,
		ProfileUsername:    route.ProfileUsername,
		CardUUID:           route.CardUUID,
		CardName:           route.CardName,
		IsLocked:           route.Status == model.RouteStatusLocked || route.CardIsLocked,
		IsActive:           route.Status == model.RouteStatusActive && (route.CardID == nil || route.CardIsActive),
		RequiresPasscode:   route.CardHasPasscode || (linkPasscode && !unlocked),
		LeadCaptureEnabled: route.CardLeadCapture || model.LeadCaptureOn(route.PresentationConfig),
		PresentationConfig: publicPresentation(route.PresentationConfig),
	}

	// Legacy provenance (source rows, owner emails and extra_json) is private account data.
	// The public resolver deliberately omits legacy_metadata.

	// Compute Target URL based on TargetType. Profiles open their public page
	// ({frontend}/{username}); the app's /profiles/{uuid} needs a login and is only a fallback.
	profileURL := ""
	switch {
	case route.ProfileUsername != "":
		profileURL = fmt.Sprintf("%s/%s", s.frontendURL, route.ProfileUsername)
	case route.ProfileUUID != nil:
		profileURL = fmt.Sprintf("%s/profiles/%s", s.frontendURL, route.ProfileUUID.String())
	}
	switch route.TargetType {
	case model.TargetTypeDirectURL:
		resolution.TargetURL = route.DirectURL
	case model.TargetTypeVCardDownload:
		if route.ProfileUsername != "" {
			resolution.TargetURL = fmt.Sprintf("%s/api/v1/public/p/%s/vcard", s.frontendURL, route.ProfileUsername)
		} else {
			resolution.TargetURL = profileURL
		}
	default: // profile, social_hub, custom_modules
		if profileURL != "" {
			resolution.TargetURL = profileURL
		} else if route.DirectURL != "" {
			resolution.TargetURL = route.DirectURL
		}
	}
	// Retired routes (cancelled/refunded cards, removed links) never send the visitor anywhere;
	// the client shows "card deactivated" (architecture decision #6).
	if route.Status == model.RouteStatusRetired {
		resolution.TargetURL = ""
	}
	if linkPasscode && !unlocked {
		resolution.TargetURL, resolution.DirectURL = "", ""
		resolution.ProfileUUID, resolution.ProfileUsername, resolution.ProfileAvatarURL = nil, "", ""
	}

	return resolution
}

// List returns the routes of an account. Route management is account-scoped: every member can
// read, owners and admins can write (docs/TENANCY_AND_ACCESS.md).
func (s *Service) List(ctx context.Context, account authctx.Account, routeType, status, q *string, limit, offset int32) ([]model.RouteSummary, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return s.repo.ListRoutes(ctx, account.ID, routeType, status, q, limit, offset)
}

// Get returns a route of the account; routes of other accounts are not found.
func (s *Service) Get(ctx context.Context, id uuid.UUID, account authctx.Account) (*model.RouteDetail, error) {
	route, err := s.getOwned(ctx, id, account.ID)
	if err != nil {
		return nil, err
	}
	return ownerView(route), nil
}

// getOwned loads a route and hides it (ErrRouteNotFound) unless it belongs to accountID.
func (s *Service) getOwned(ctx context.Context, id uuid.UUID, accountID int64) (*model.RouteDetail, error) {
	route, err := s.repo.GetRouteByUUID(ctx, id)
	if err != nil {
		return nil, err
	}
	if route == nil || route.AccountID == nil || *route.AccountID != accountID {
		return nil, ErrRouteNotFound
	}
	return route, nil
}

// getWritable is getOwned plus the owner/admin role check.
func (s *Service) getWritable(ctx context.Context, id uuid.UUID, account authctx.Account) (*model.RouteDetail, error) {
	route, err := s.getOwned(ctx, id, account.ID)
	if err != nil {
		return nil, err
	}
	if !account.CanWrite() {
		return nil, ErrForbidden
	}
	// Usernames are managed by the profile's public page (/v1/profiles/{uuid}/public-page): a
	// generic edit or delete here would bypass its rules, and deleting a retired username would
	// free it for someone else while old links still point at it.
	if route.RouteType == model.RouteTypeCanonicalProfile {
		return nil, ErrRouteFieldForbidden
	}
	return route, nil
}

func (s *Service) CreateCustomSmartLink(ctx context.Context, account authctx.Account, creatorUserID int64, input model.CreateSmartLinkInput) (*model.RouteDetail, error) {
	if !account.CanWrite() {
		return nil, ErrForbidden
	}
	if err := entitlement.Guard(ctx, s.quotas, account.ID, entitlement.FeatureSmartLinksMax); err != nil {
		return nil, err
	}
	rawSlug := strings.TrimSpace(input.Slug)
	if !strings.HasPrefix(rawSlug, "s/") {
		rawSlug = "s/" + rawSlug
	}
	normPath := s.NormalizePath(rawSlug)

	if err := s.ValidatePathFormat(normPath); err != nil {
		return nil, err
	}

	// Check if path is reserved keyword (e.g. "s/platform")
	slugPart := strings.TrimPrefix(normPath, "s/")
	if isReservedKeyword(slugPart) {
		return nil, ErrRoutePathReserved
	}

	exists, err := s.repo.CheckPathExists(ctx, normPath, nil)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrRoutePathConflict
	}

	var profileID *int64
	if input.ProfileUUID != nil {
		pid, err := s.repo.GetProfileIDByUUID(ctx, *input.ProfileUUID, account.ID)
		if err != nil || pid == nil {
			return nil, ErrInvalidProfileTarget
		}
		profileID = pid
	}

	targetType := input.TargetType
	if targetType == "" {
		targetType = model.TargetTypeProfile
	}
	if err := validateTarget(targetType, profileID != nil, input.DirectURL); err != nil {
		return nil, err
	}
	presentation, err := normalizePresentation(input.PresentationConfig, nil)
	if err != nil {
		return nil, err
	}
	if err := s.requirePremiumTheme(ctx, account.ID, presentation, nil); err != nil {
		return nil, err
	}

	created, err := s.repo.CreateRoute(ctx, model.CreateReservedRouteInput{
		Path:               normPath,
		RouteType:          model.RouteTypeCustomSmartLink,
		Status:             model.RouteStatusActive,
		IsImmutable:        false,
		TargetType:         targetType,
		ProfileID:          profileID,
		DirectURL:          input.DirectURL,
		AccountID:          &account.ID,
		OwnerUserID:        &creatorUserID,
		PresentationConfig: presentation,
	})
	if err != nil {
		return nil, err
	}
	return ownerView(created), nil
}

func (s *Service) CreateReservedCardRoute(ctx context.Context, cardID int64, smartCode string, ownerUserID *int64, accountID *int64) (*model.RouteDetail, error) {
	path := s.NormalizePath("c/" + smartCode)
	exists, err := s.repo.CheckPathExists(ctx, path, nil)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrRoutePathConflict
	}

	return s.repo.CreateRoute(ctx, model.CreateReservedRouteInput{
		Path:        path,
		RouteType:   model.RouteTypeCardRoute,
		Status:      model.RouteStatusReserved,
		IsImmutable: true,
		TargetType:  model.TargetTypeProfile,
		CardID:      &cardID,
		AccountID:   accountID,
		OwnerUserID: ownerUserID,
	})
}

func (s *Service) RegisterLegacyRoute(ctx context.Context, input model.CreateReservedRouteInput) (*model.RouteDetail, error) {
	normPath := s.NormalizePath(input.Path)
	if normPath == "" {
		return nil, ErrInvalidRoutePath
	}

	exists, err := s.repo.CheckPathExists(ctx, normPath, nil)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrRoutePathConflict
	}

	input.Path = normPath
	input.RouteType = model.RouteTypeLegacyRoute
	input.IsImmutable = true
	if input.Status == "" {
		input.Status = model.RouteStatusActive
	}

	return s.repo.CreateRoute(ctx, input)
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, account authctx.Account, input model.UpdateRouteInput) (*model.RouteDetail, error) {
	existing, err := s.getWritable(ctx, id, account)
	if err != nil {
		return nil, err
	}

	// Lifecycle status is driven by activation, lock toggle and retirement flows, not by direct edits.
	if input.Status != nil && *input.Status != existing.Status {
		return nil, ErrRouteFieldForbidden
	}
	// Only customer-managed smart links may change their public path; card, canonical and
	// legacy paths are printed on hardware or are identity anchors.
	if input.Path != nil && existing.RouteType != model.RouteTypeCustomSmartLink &&
		s.NormalizePath(*input.Path) != existing.Path {
		return nil, ErrRouteFieldForbidden
	}

	if existing.IsImmutable && input.Path != nil && s.NormalizePath(*input.Path) != existing.Path {
		return nil, ErrRouteImmutable
	}

	// A renamed smart link goes through the same rules as a new one: it stays under s/, keeps the
	// path format, avoids reserved words and must be free.
	if input.Path != nil && s.NormalizePath(*input.Path) != existing.Path {
		raw := strings.TrimSpace(*input.Path)
		if !strings.HasPrefix(raw, "s/") {
			raw = "s/" + raw
		}
		normPath := s.NormalizePath(raw)
		if err := s.ValidatePathFormat(normPath); err != nil {
			return nil, err
		}
		if isReservedKeyword(strings.TrimPrefix(normPath, "s/")) {
			return nil, ErrRoutePathReserved
		}
		exists, err := s.repo.CheckPathExists(ctx, normPath, &id)
		if err != nil {
			return nil, err
		}
		if exists {
			return nil, ErrRoutePathConflict
		}
		input.Path = &normPath
	}

	var profileID *int64
	setProfileID := false
	if input.ProfileUUID != nil {
		pid, err := s.repo.GetProfileIDByUUID(ctx, *input.ProfileUUID, account.ID)
		if err != nil || pid == nil {
			return nil, ErrInvalidProfileTarget
		}
		profileID = pid
		setProfileID = true
	} else if input.TargetType != nil && *input.TargetType != model.TargetTypeProfile {
		profileID = nil
		setProfileID = true
	}

	if existing.RouteType == model.RouteTypeCustomSmartLink {
		if err := s.validateSmartLinkUpdate(ctx, account.ID, existing, &input, profileID, setProfileID); err != nil {
			return nil, err
		}
	}

	renamed := input.Path != nil && s.NormalizePath(*input.Path) != existing.Path
	var actorID *int64
	if p, ok := authctx.PrincipalFrom(ctx); ok {
		actorID = &p.UserInternal
	}
	if renamed {
		// Written before the update: if the update then fails, the history row only holds a path
		// the link still owns.
		if err := s.repo.RecordPathChange(ctx, model.PathChange{
			RouteID: existing.ID, RouteType: existing.RouteType, AccountID: existing.AccountID,
			OldPath: existing.Path, NewPath: *input.Path, ChangedByUserID: actorID,
			Reason: model.PathChangeSmartLinkRename,
		}); err != nil {
			return nil, err
		}
	}

	updated, err := s.repo.UpdateRoute(ctx, id, account.ID, input, profileID, nil, setProfileID, false)
	if err != nil {
		return nil, err
	}
	s.cache.InvalidatePaths(ctx, existing.Path, updated.Path)
	if renamed {
		s.activity.Record(ctx, actorID, "route.path_changed", "routes", &updated.UUID, map[string]any{
			"before": map[string]any{"path": existing.Path},
			"after":  map[string]any{"path": updated.Path},
			"reason": model.PathChangeSmartLinkRename,
		}, nil)
	}
	return ownerView(updated), nil
}

// validateSmartLinkUpdate checks the target the link ends up with and normalizes a new
// presentation_config (keeping the stored passcode unless it is replaced or turned off).
func (s *Service) validateSmartLinkUpdate(ctx context.Context, accountID int64, existing *model.RouteDetail, input *model.UpdateRouteInput, profileID *int64, setProfileID bool) error {
	if input.TargetType != nil || input.DirectURL != nil || setProfileID {
		targetType, directURL, hasProfile := existing.TargetType, existing.DirectURL, existing.ProfileID != nil
		if input.TargetType != nil {
			targetType = *input.TargetType
		}
		if input.DirectURL != nil {
			directURL = *input.DirectURL
		}
		if setProfileID {
			hasProfile = profileID != nil
		}
		if err := validateTarget(targetType, hasProfile, directURL); err != nil {
			return err
		}
	}
	if input.PresentationConfig != nil {
		cfg, err := normalizePresentation(input.PresentationConfig, existing.PresentationConfig)
		if err != nil {
			return err
		}
		if err := s.requirePremiumTheme(ctx, accountID, cfg, existing.PresentationConfig); err != nil {
			return err
		}
		input.PresentationConfig = cfg
	}
	return nil
}

func (s *Service) ToggleLock(ctx context.Context, id uuid.UUID, account authctx.Account) (*model.RouteDetail, error) {
	existing, err := s.getWritable(ctx, id, account)
	if err != nil {
		return nil, err
	}

	// Reserved routes must go through card activation and retired routes must stay retired.
	var newStatus string
	switch existing.Status {
	case model.RouteStatusActive:
		newStatus = model.RouteStatusLocked
	case model.RouteStatusLocked:
		newStatus = model.RouteStatusActive
	default:
		return nil, ErrRouteNotLockable
	}

	updated, err := s.repo.UpdateRoute(ctx, id, account.ID, model.UpdateRouteInput{
		Status: &newStatus,
	}, nil, nil, false, false)
	if err != nil {
		return nil, err
	}
	s.cache.InvalidatePaths(ctx, existing.Path)
	return ownerView(updated), nil
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID, account authctx.Account) error {
	existing, err := s.getWritable(ctx, id, account)
	if err != nil {
		return err
	}
	if existing.IsImmutable {
		return ErrRouteImmutable
	}

	if err := s.repo.DeleteRoute(ctx, id, account.ID); err != nil {
		return err
	}
	s.cache.InvalidatePaths(ctx, existing.Path)
	return nil
}
