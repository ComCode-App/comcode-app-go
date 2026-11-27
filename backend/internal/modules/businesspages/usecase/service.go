// Package usecase manages ComCode Business pages (TEC-1327): a company account's own link page
// at /{username}. App methods act on the active account only (company accounts, owner/admin to
// write, the business_page.enabled plan feature to save or publish). The visitor read is a
// separate method that serves only a published page of a live company.
package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/businesspages/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/businesspages/repository"
	pmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/publicpath"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/google/uuid"
)

// Store is the persistence the service needs (repository.Postgres).
type Store interface {
	Page(ctx context.Context, accountID int64) (*db.GetBusinessPageRow, error)
	Blocks(ctx context.Context, accountID, pageID int64) ([]db.ListBusinessPageBlocksRow, error)
	LiveRoute(ctx context.Context, accountID int64) (*repository.Route, error)
	PathOwner(ctx context.Context, path string) (*repository.PathOwner, error)
	Save(ctx context.Context, in repository.Save) error
	SetStatus(ctx context.Context, accountID, userID int64, status string, route *repository.Route) error
	PublicRoute(ctx context.Context, path string) (*repository.PublicRoute, error)
	Team(ctx context.Context, accountID int64, locale string) ([]db.ListBusinessTeamProfilesRow, error)
	Offices(ctx context.Context, accountID int64) ([]db.ListBusinessOfficesRow, error)
	BankAccounts(ctx context.Context, accountID int64, ids []uuid.UUID) ([]db.ListBusinessBankAccountsRow, error)
	MediaKeys(ctx context.Context, accountID int64, ids []uuid.UUID) (map[uuid.UUID]string, error)
	SocialIcons(ctx context.Context, slugs []string) (map[string]string, error)
}

// MediaURLs turns object keys into API-owned public media URLs (storage driver).
type MediaURLs interface {
	PublicURL(key string) string
}

// BrandingSource is the company look (companybranding service).
type BrandingSource interface {
	AccountBranding(ctx context.Context, accountID int64) (*pmodel.AccountBranding, error)
}

// Service is the business page use case.
type Service struct {
	store    Store
	media    MediaURLs
	features entitlement.Features
	branding BrandingSource
}

func New(store Store, media MediaURLs) *Service { return &Service{store: store, media: media} }

// SetFeatures wires the business_page.enabled plan check. Without it saving and publishing are
// refused and public pages answer 404 (fail-closed).
func (s *Service) SetFeatures(f entitlement.Features) { s.features = f }

// SetBranding lets the page inherit the company theme, color, logo and cover.
func (s *Service) SetBranding(b BrandingSource) { s.branding = b }

func access(p authctx.Principal, write bool) error {
	permission := rbac.PermAppBusinessPageRead
	if write {
		permission = rbac.PermAppBusinessPageWrite
	}
	if p.Account.ID <= 0 || p.Account.Type != authctx.AccountTypeCompany || !p.HasPermission(permission) {
		return model.ErrForbidden
	}
	if write && !p.Account.CanWrite() {
		return model.ErrForbidden
	}
	return nil
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", model.ErrInvalid, fmt.Sprintf(format, args...))
}

func (s *Service) mediaURL(key string) string {
	if key == "" {
		return ""
	}
	if s.media == nil {
		return "/api/v1/storage/stream/" + key
	}
	return s.media.PublicURL(key)
}

// Get returns the active company's page.
func (s *Service) Get(ctx context.Context, p authctx.Principal) (*model.Page, error) {
	if err := access(p, false); err != nil {
		return nil, err
	}
	return s.page(ctx, p.Account.ID)
}

func (s *Service) page(ctx context.Context, accountID int64) (*model.Page, error) {
	row, err := s.store.Page(ctx, accountID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	blocks, err := s.store.Blocks(ctx, accountID, row.ID)
	if err != nil {
		return nil, err
	}
	route, err := s.store.LiveRoute(ctx, accountID)
	if err != nil {
		return nil, err
	}
	out := &model.Page{
		UUID: row.Uuid, Status: row.Status, Theme: row.Theme, PrimaryLocale: row.PrimaryLocale,
		Translations: map[string]model.PageText{}, Blocks: make([]model.Block, 0, len(blocks)),
		UpdatedAt: row.UpdatedAt.Time,
	}
	if route != nil {
		out.Username = route.Path
	}
	if row.PublishedAt.Valid {
		t := row.PublishedAt.Time
		out.PublishedAt = &t
	}
	_ = json.Unmarshal(row.Translations, &out.Translations)
	for _, b := range blocks {
		blk := model.Block{UUID: b.Uuid, Type: b.Type, Enabled: b.Enabled, Config: b.Config, Translations: map[string]model.BlockText{}}
		_ = json.Unmarshal(b.Translations, &blk.Translations)
		out.Blocks = append(out.Blocks, blk)
	}
	return out, nil
}

// Save replaces the active company's page (owner/admin, business_page.enabled). The first save
// needs a username; a different username later renames the page and the old one redirects.
func (s *Service) Save(ctx context.Context, p authctx.Principal, in model.SaveInput) (*model.Page, error) {
	if err := access(p, true); err != nil {
		return nil, err
	}
	if err := entitlement.RequireFeature(ctx, s.features, p.Account.ID, entitlement.FeatureBusinessPage); err != nil {
		return nil, err
	}
	save := repository.Save{AccountID: p.Account.ID, UserID: p.UserInternal}

	save.Theme = strings.TrimSpace(in.Theme)
	if save.Theme != "" && !slices.Contains(pmodel.PublicThemes, save.Theme) {
		return nil, invalid("unknown theme %q", save.Theme)
	}
	save.PrimaryLocale = strings.TrimSpace(in.PrimaryLocale)
	if save.PrimaryLocale == "" {
		save.PrimaryLocale = "tr"
	}
	if !slices.Contains(model.Locales, save.PrimaryLocale) {
		return nil, invalid("primary_locale must be tr or en")
	}
	texts, err := cleanPageTexts(in.Translations)
	if err != nil {
		return nil, err
	}
	if save.Translations, err = json.Marshal(texts); err != nil {
		return nil, err
	}
	if len(in.Blocks) > model.MaxBlocks {
		return nil, invalid("at most %d blocks", model.MaxBlocks)
	}
	for i, b := range in.Blocks {
		nb, err := s.cleanBlock(ctx, p.Account.ID, i, b)
		if err != nil {
			return nil, err
		}
		save.Blocks = append(save.Blocks, nb)
	}

	current, err := s.store.LiveRoute(ctx, p.Account.ID)
	if err != nil {
		return nil, err
	}
	save.Current = current
	save.Status = "locked"
	if current != nil {
		save.Status = current.Status
	}
	username := publicpath.NormalizeUsername(in.Username)
	switch {
	case username == "" && current == nil:
		return nil, invalid("username is required")
	case username != "" && (current == nil || username != current.Path):
		if err := publicpath.ValidateUsername(username); err != nil {
			return nil, invalid("%s", err.Error())
		}
		owner, err := s.store.PathOwner(ctx, username)
		if err != nil {
			return nil, err
		}
		if owner != nil {
			// A retired username of this company's own page can be taken back.
			if owner.RouteType != "business_page" || owner.AccountID != p.Account.ID || owner.Status != "retired" {
				return nil, model.ErrUsernameTaken
			}
			save.ReviveID = owner.RouteID
		}
		save.NewPath = username
	}
	if err := s.store.Save(ctx, save); err != nil {
		return nil, err
	}
	return s.page(ctx, p.Account.ID)
}

// SetPublished publishes or unpublishes the page. A draft page answers 404 to visitors.
func (s *Service) SetPublished(ctx context.Context, p authctx.Principal, published bool) (*model.Page, error) {
	if err := access(p, true); err != nil {
		return nil, err
	}
	status := model.StatusDraft
	if published {
		if err := entitlement.RequireFeature(ctx, s.features, p.Account.ID, entitlement.FeatureBusinessPage); err != nil {
			return nil, err
		}
		status = model.StatusPublished
	}
	route, err := s.store.LiveRoute(ctx, p.Account.ID)
	if err != nil {
		return nil, err
	}
	if err := s.store.SetStatus(ctx, p.Account.ID, p.UserInternal, status, route); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, model.ErrNotFound
		}
		return nil, err
	}
	return s.page(ctx, p.Account.ID)
}

func cleanPageTexts(in map[string]model.PageText) (map[string]model.PageText, error) {
	out := map[string]model.PageText{}
	for locale, t := range in {
		if !slices.Contains(model.Locales, locale) {
			return nil, invalid("unknown locale %q", locale)
		}
		t.Name, t.Tagline = strings.TrimSpace(t.Name), strings.TrimSpace(t.Tagline)
		if len([]rune(t.Name)) > 120 || len([]rune(t.Tagline)) > 200 {
			return nil, invalid("translations.%s: name is at most 120 and tagline 200 characters", locale)
		}
		out[locale] = t
	}
	return out, nil
}

func cleanBlockTexts(i int, in map[string]model.BlockText) ([]byte, error) {
	out := map[string]model.BlockText{}
	for locale, t := range in {
		if !slices.Contains(model.Locales, locale) {
			return nil, invalid("blocks[%d]: unknown locale %q", i, locale)
		}
		t.Title, t.Subtitle = strings.TrimSpace(t.Title), strings.TrimSpace(t.Subtitle)
		if len([]rune(t.Title)) > 120 || len([]rune(t.Subtitle)) > 200 {
			return nil, invalid("blocks[%d].translations.%s: title is at most 120 and subtitle 200 characters", i, locale)
		}
		out[locale] = t
	}
	return json.Marshal(out)
}

// decodeStrict reads a block config, refusing unknown fields; an empty config is the zero value.
func decodeStrict(raw json.RawMessage, v any) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func (s *Service) cleanBlock(ctx context.Context, accountID int64, i int, in model.BlockInput) (repository.NewBlock, error) {
	out := repository.NewBlock{UUID: uuid.New(), Type: in.Type, Enabled: in.Enabled == nil || *in.Enabled}
	if !slices.Contains(model.BlockTypes, in.Type) {
		return out, invalid("blocks[%d]: unknown type %q", i, in.Type)
	}
	var err error
	if out.Translations, err = cleanBlockTexts(i, in.Translations); err != nil {
		return out, err
	}
	var cfg any
	switch in.Type {
	case model.BlockHeader:
		var c model.HeaderConfig
		if err := decodeStrict(in.Config, &c); err != nil {
			return out, invalid("blocks[%d].config: %v", i, err)
		}
		ids := []uuid.UUID{}
		for _, id := range []*uuid.UUID{c.LogoMediaUUID, c.CoverMediaUUID} {
			if id != nil {
				ids = append(ids, *id)
			}
		}
		keys, err := s.store.MediaKeys(ctx, accountID, ids)
		if err != nil {
			return out, err
		}
		for _, id := range ids {
			if _, ok := keys[id]; !ok {
				return out, invalid("blocks[%d]: image %s is not in this company's library", i, id)
			}
		}
		cfg = c
	case model.BlockButton:
		var c model.ButtonConfig
		if err := decodeStrict(in.Config, &c); err != nil {
			return out, invalid("blocks[%d].config: %v", i, err)
		}
		if c, err = cleanButton(c); err != nil {
			return out, invalid("blocks[%d]: %v", i, err)
		}
		cfg = c
	case model.BlockSocial:
		var c model.SocialConfig
		if err := decodeStrict(in.Config, &c); err != nil {
			return out, invalid("blocks[%d].config: %v", i, err)
		}
		if len(c.Links) > 20 {
			return out, invalid("blocks[%d]: at most 20 social links", i)
		}
		links := make([]model.SocialLink, 0, len(c.Links))
		for j, l := range c.Links {
			l.Platform = strings.ToLower(strings.TrimSpace(l.Platform))
			l.URL = strings.TrimSpace(l.URL)
			if l.Platform == "" || len(l.Platform) > 100 {
				return out, invalid("blocks[%d].links[%d]: platform is required", i, j)
			}
			if !isWebURL(l.URL) {
				return out, invalid("blocks[%d].links[%d]: url must be an http(s) address", i, j)
			}
			links = append(links, l)
		}
		cfg = model.SocialConfig{Links: links}
	case model.BlockBankAccounts:
		var c model.BankAccountsConfig
		if err := decodeStrict(in.Config, &c); err != nil {
			return out, invalid("blocks[%d].config: %v", i, err)
		}
		if len(c.BankAccountUUIDs) > 20 {
			return out, invalid("blocks[%d]: at most 20 bank accounts", i)
		}
		rows, err := s.store.BankAccounts(ctx, accountID, c.BankAccountUUIDs)
		if err != nil {
			return out, err
		}
		for _, id := range c.BankAccountUUIDs {
			if !slices.ContainsFunc(rows, func(r db.ListBusinessBankAccountsRow) bool { return r.Uuid == id }) {
				return out, invalid("blocks[%d]: bank account %s is not an active account of this company", i, id)
			}
		}
		if c.BankAccountUUIDs == nil {
			c.BankAccountUUIDs = []uuid.UUID{}
		}
		cfg = c
	default: // offices, team: no settings
		if err := decodeStrict(in.Config, &struct{}{}); err != nil {
			return out, invalid("blocks[%d].config: %v", i, err)
		}
		cfg = struct{}{}
	}
	out.Config, err = json.Marshal(cfg)
	return out, err
}

func isWebURL(v string) bool {
	low := strings.ToLower(v)
	return len(v) <= 2048 && (strings.HasPrefix(low, "https://") || strings.HasPrefix(low, "http://")) && len(v) > len("https://")
}

func cleanButton(c model.ButtonConfig) (model.ButtonConfig, error) {
	c.Target = strings.TrimSpace(c.Target)
	c.Value = strings.TrimSpace(c.Value)
	c.Icon = strings.TrimSpace(c.Icon)
	if len(c.Icon) > 40 {
		return c, errors.New("icon is at most 40 characters")
	}
	switch c.Target {
	case model.TargetURL, model.TargetGoogleReview:
		if !isWebURL(c.Value) {
			return c, errors.New("value must be an http(s) address")
		}
	case model.TargetMaps:
		// An address to search, or a maps link.
		if c.Value == "" || len(c.Value) > 2048 {
			return c, errors.New("value must be an address or a maps link")
		}
	case model.TargetPhone, model.TargetWhatsApp:
		n := 0
		for _, r := range c.Value {
			switch {
			case r >= '0' && r <= '9':
				n++
			case strings.ContainsRune("+ ()-.", r):
			default:
				return c, errors.New("value must be a phone number")
			}
		}
		if n < 5 || n > 20 {
			return c, errors.New("value must be a phone number")
		}
	case model.TargetEmail:
		at := strings.Index(c.Value, "@")
		if at < 1 || at == len(c.Value)-1 || strings.ContainsAny(c.Value, " <>\"") || len(c.Value) > 254 {
			return c, errors.New("value must be an e-mail address")
		}
	case model.TargetLeadForm:
		c.Value = ""
	default:
		return c, fmt.Errorf("unknown target %q", c.Target)
	}
	return c, nil
}
