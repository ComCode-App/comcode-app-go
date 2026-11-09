package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyprofiles/model"
	pmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/bulkengine"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Media is the object storage surface of rollbacks (storage.Driver): logo jobs run before
// TEC-946 copied files into the profiles, and undoing them removes the copy.
type Media interface {
	Delete(ctx context.Context, path string) error
}

// Branding resolves the company branding logo or cover (companybranding usecase).
type Branding interface {
	BrandingMediaURL(ctx context.Context, accountID int64, slot string) (string, error)
}

// maxBulkTargets bounds a "query" target scope.
const maxBulkTargets = 5000

// BulkAdapter is the bulk-engine adapter of employee profiles (model.ResourceCompanyProfiles).
// Every call runs inside the job's company account (jobscope) and re-checks the actor.
type BulkAdapter struct {
	q        Store
	profiles Profiles
	media    Media
	branding Branding
	// features gates premium themes (design.premium, TEC-947); nil refuses them.
	features entitlement.Features
}

func NewBulkAdapter(q Store, profiles Profiles, media Media, branding Branding) *BulkAdapter {
	return &BulkAdapter{q: q, profiles: profiles, media: media, branding: branding}
}

// SetFeatures wires the design.premium check for the theme action.
func (a *BulkAdapter) SetFeatures(f entitlement.Features) { a.features = f }

var _ bulkengine.AccountScoped = (*BulkAdapter)(nil)

func (a *BulkAdapter) Resource() string { return model.ResourceCompanyProfiles }

func (a *BulkAdapter) BulkActions() []bulkengine.BulkActionDef {
	def := func(id string) bulkengine.BulkActionDef {
		return bulkengine.BulkActionDef{
			ID: id, LabelKey: "bulk.actions.company_profiles." + id,
			Permission: rbac.PermAppCompanyProfilesBulk, Reversible: true,
			ConfirmKey: "bulk.confirm.company_profiles." + id,
		}
	}
	return []bulkengine.BulkActionDef{
		def(model.ActionSetContact), def(model.ActionSetLogo), def(model.ActionSetTheme), def(model.ActionAddLinks),
	}
}

// snapshot is the before-state of one profile; only the parts an action changed are set, and
// RevertItem restores exactly those.
type snapshot struct {
	Translations   *[]pmodel.UpsertTranslationInput   `json:"translations,omitempty"`
	Addresses      *[]pmodel.CreateAddressInput       `json:"addresses,omitempty"`
	ContactDetails *[]pmodel.CreateContactDetailInput `json:"contact_details,omitempty"`
	SocialLinks    *[]pmodel.CreateSocialLinkInput    `json:"social_links,omitempty"`
	AvatarURL      *string                            `json:"avatar_url,omitempty"`
	CoverURL       *string                            `json:"cover_url,omitempty"`
	CopiedKey      string                             `json:"copied_key,omitempty"`
	Theme          *string                            `json:"theme,omitempty"`
	LayoutMode     *string                            `json:"layout_mode,omitempty"`
}

func (s snapshot) toMap() map[string]any {
	b, _ := json.Marshal(s)
	out := map[string]any{}
	_ = json.Unmarshal(b, &out)
	return out
}

func snapshotFrom(m map[string]any) (snapshot, error) {
	var s snapshot
	b, err := json.Marshal(m)
	if err != nil {
		return s, err
	}
	err = json.Unmarshal(b, &s)
	return s, err
}

func decodeParams[T any](raw json.RawMessage) (T, error) {
	var v T
	if len(raw) == 0 {
		return v, fmt.Errorf("%w: params are required", model.ErrInvalid)
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		return v, fmt.Errorf("%w: params: %v", model.ErrInvalid, err)
	}
	return v, nil
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", model.ErrInvalid, fmt.Sprintf(format, args...))
}

func trimPtr(v *string) *string {
	if v == nil {
		return nil
	}
	t := strings.TrimSpace(*v)
	return &t
}

func tooLong(v string, max int) bool { return utf8.RuneCountInString(v) > max }

func validWebURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func (a *BulkAdapter) contactParams(raw json.RawMessage) (model.ContactParams, error) {
	p, err := decodeParams[model.ContactParams](raw)
	if err != nil {
		return p, err
	}
	p.CompanyName, p.Phone, p.Email, p.Website = trimPtr(p.CompanyName), trimPtr(p.Phone), trimPtr(p.Email), trimPtr(p.Website)
	if p.CompanyName == nil && p.Address == nil && !p.UseOfficeAddress && p.Phone == nil && p.Email == nil && p.Website == nil {
		return p, invalid("nothing to change")
	}
	if p.Address != nil && p.UseOfficeAddress {
		return p, invalid("address and use_office_address are exclusive")
	}
	if p.CompanyName != nil && tooLong(*p.CompanyName, 255) {
		return p, invalid("company_name is too long")
	}
	if p.Address != nil {
		ad := p.Address
		for _, f := range []*string{&ad.Label, &ad.Street, &ad.Extended, &ad.City, &ad.Region, &ad.PostalCode, &ad.Country} {
			*f = strings.TrimSpace(*f)
		}
		if tooLong(ad.Label, 100) || tooLong(ad.Street, 1000) || tooLong(ad.Extended, 1000) || tooLong(ad.City, 100) ||
			tooLong(ad.Region, 100) || tooLong(ad.PostalCode, 32) || tooLong(ad.Country, 100) {
			return p, invalid("address field is too long")
		}
	}
	if p.Phone != nil && tooLong(*p.Phone, 50) {
		return p, invalid("phone is too long")
	}
	if p.Email != nil && *p.Email != "" && (tooLong(*p.Email, 255) || !strings.Contains(*p.Email, "@") || strings.ContainsAny(*p.Email, " \t")) {
		return p, invalid("email is invalid")
	}
	if p.Website != nil && *p.Website != "" && (tooLong(*p.Website, 2048) || !validWebURL(*p.Website)) {
		return p, invalid("website must be an http(s) URL")
	}
	return p, nil
}

func validMediaField(f string) bool { return f == model.MediaAvatar || f == model.MediaCover }

func (a *BulkAdapter) themeParams(raw json.RawMessage) (model.ThemeParams, error) {
	p, err := decodeParams[model.ThemeParams](raw)
	if err != nil {
		return p, err
	}
	p.Theme, p.LayoutMode = trimPtr(p.Theme), trimPtr(p.LayoutMode)
	if p.Theme == nil && p.LayoutMode == nil {
		return p, invalid("nothing to change")
	}
	if p.Theme != nil && *p.Theme != "" && !slices.Contains(pmodel.PublicThemes, *p.Theme) {
		return p, invalid("theme %q is not available", *p.Theme)
	}
	if p.LayoutMode != nil && *p.LayoutMode != "" && !slices.Contains(pmodel.PublicLayoutModes, *p.LayoutMode) {
		return p, invalid("layout_mode %q is not available", *p.LayoutMode)
	}
	return p, nil
}

func (a *BulkAdapter) linksParams(raw json.RawMessage) (model.LinksParams, error) {
	p, err := decodeParams[model.LinksParams](raw)
	if err != nil {
		return p, err
	}
	if len(p.Links) == 0 || len(p.Links) > 10 {
		return p, invalid("between 1 and 10 links are required")
	}
	for i := range p.Links {
		l := &p.Links[i]
		l.Platform = strings.ToLower(strings.TrimSpace(l.Platform))
		l.Label = strings.TrimSpace(l.Label)
		l.URL = strings.TrimSpace(l.URL)
		if l.Platform == "" {
			l.Platform = "custom"
		}
		if tooLong(l.Label, 100) || tooLong(l.Platform, 64) {
			return p, invalid("link %d: label or platform is too long", i+1)
		}
		if tooLong(l.URL, 2048) || !validWebURL(l.URL) {
			return p, invalid("link %d: url must be an http(s) URL", i+1)
		}
	}
	return p, nil
}

// ValidateParams checks the action parameters and, for the logo, that the company branding has
// the referenced logo or cover.
func (a *BulkAdapter) ValidateParams(ctx context.Context, action string, raw json.RawMessage) error {
	ac, _, err := scopedActor(ctx, a.q)
	if err != nil {
		return err
	}
	switch action {
	case model.ActionSetContact:
		_, err = a.contactParams(raw)
	case model.ActionSetTheme:
		var p model.ThemeParams
		if p, err = a.themeParams(raw); err == nil && p.Theme != nil {
			if _, premium := pmodel.PremiumThemes[*p.Theme]; premium {
				err = entitlement.RequireFeature(ctx, a.features, ac.accountID, entitlement.FeatureDesignPremium)
			}
		}
	case model.ActionAddLinks:
		var p model.LinksParams
		if p, err = a.linksParams(raw); err == nil {
			for _, l := range p.Links {
				if _, err = a.platform(ctx, l.Platform); err != nil {
					return err
				}
			}
		}
	case model.ActionSetLogo:
		var p model.LogoParams
		if p, err = decodeParams[model.LogoParams](raw); err == nil {
			_, err = a.logoSource(ctx, ac, p)
		}
	default:
		err = invalid("unknown action")
	}
	return err
}

func (a *BulkAdapter) platform(ctx context.Context, slug string) (*db.SocialPlatform, error) {
	if slug == "custom" {
		return nil, nil
	}
	pl, err := a.q.GetSocialPlatformBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, invalid("unknown platform %q", slug)
		}
		return nil, err
	}
	if !pl.IsActive {
		return nil, invalid("platform %q is not active", slug)
	}
	return &pl, nil
}

// logoSource validates LogoParams and returns the media URL of the referenced branding logo
// or cover of the job's account.
func (a *BulkAdapter) logoSource(ctx context.Context, ac actor, p model.LogoParams) (string, error) {
	if p.Source != model.BrandingLogo && p.Source != model.BrandingCover {
		return "", invalid("source must be logo or cover")
	}
	if !validMediaField(p.TargetField) {
		return "", invalid("target_field must be avatar or cover")
	}
	if a.branding == nil {
		return "", invalid("company branding is not available")
	}
	mediaURL, err := a.branding.BrandingMediaURL(ctx, ac.accountID, p.Source)
	if err != nil {
		return "", err
	}
	if mediaURL == "" {
		return "", invalid("the company branding has no %s; pick one from the media library first", p.Source)
	}
	return mediaURL, nil
}

// ResolveTargets returns profile uuids. "ids" are profile uuids (checked per item); "query"
// selects the linked profiles of members matching office_uuid, group_uuid and q, limited to the
// actor's offices for office managers.
func (a *BulkAdapter) ResolveTargets(ctx context.Context, _ string, target bulkengine.BulkTarget) ([]string, error) {
	ac, scope, err := scopedActor(ctx, a.q)
	if err != nil {
		return nil, err
	}
	switch target.Scope {
	case "ids":
		seen := map[uuid.UUID]bool{}
		out := make([]string, 0, len(target.IDs))
		for _, raw := range target.IDs {
			id, err := uuid.Parse(strings.TrimSpace(raw))
			if err != nil {
				return nil, invalid("invalid profile uuid %q", raw)
			}
			if !seen[id] {
				seen[id] = true
				out = append(out, id.String())
			}
		}
		if len(out) > maxBulkTargets {
			return nil, invalid("too many targets")
		}
		return out, nil
	case "query":
		params := db.ListCompanyBulkProfileUUIDsParams{
			AccountID: scope.AccountID, Q: strings.TrimSpace(target.Query["q"]), LimitCount: maxBulkTargets,
		}
		for key, dst := range map[string]*pgtype.UUID{"office_uuid": &params.OfficeUuid, "group_uuid": &params.GroupUuid} {
			if v := strings.TrimSpace(target.Query[key]); v != "" {
				id, err := uuid.Parse(v)
				if err != nil {
					return nil, invalid("invalid %s", key)
				}
				*dst = pgtype.UUID{Bytes: id, Valid: true}
			}
		}
		if !ac.isAdmin() {
			params.ScopeMemberID = pgtype.Int8{Int64: ac.memberID, Valid: true}
		}
		rows, err := a.q.ListCompanyBulkProfileUUIDs(ctx, params)
		if err != nil {
			return nil, err
		}
		out := make([]string, 0, len(rows))
		for _, id := range rows {
			out = append(out, id.String())
		}
		return out, nil
	default:
		return nil, invalid("invalid target scope")
	}
}

func failed(id, msg string) bulkengine.BulkItemResult {
	return bulkengine.BulkItemResult{EntityUUID: id, EntityType: "profile", OK: false, Error: msg}
}

// ApplyItem changes one profile and returns its before-state.
func (a *BulkAdapter) ApplyItem(ctx context.Context, action, entityUUID string) (bulkengine.BulkItemResult, error) {
	id, err := uuid.Parse(entityUUID)
	if err != nil {
		return failed(entityUUID, "not_found"), nil
	}
	ac, scope, err := scopedActor(ctx, a.q)
	if err != nil {
		return failed(entityUUID, itemError(err)), nil
	}
	if err := canTouchProfile(ctx, a.q, ac, id); err != nil {
		return failed(entityUUID, itemError(err)), nil
	}
	var prev snapshot
	switch action {
	case model.ActionSetContact:
		prev, err = a.applyContact(ctx, ac, id, scope.Params)
	case model.ActionSetLogo:
		prev, err = a.applyLogo(ctx, ac, id, scope.Params)
	case model.ActionSetTheme:
		prev, err = a.applyTheme(ctx, ac, id, scope.Params)
	case model.ActionAddLinks:
		prev, err = a.applyLinks(ctx, ac, id, scope.Params)
	default:
		err = invalid("unknown action")
	}
	if err != nil {
		return failed(entityUUID, itemError(err)), nil
	}
	return bulkengine.BulkItemResult{
		EntityUUID: entityUUID, EntityType: "profile", OK: true, Op: "update", Previous: prev.toMap(),
	}, nil
}

// RevertItem restores the parts of the profile recorded in previous. The user rolling back must
// still be allowed to change the profile.
func (a *BulkAdapter) RevertItem(ctx context.Context, _ string, entityUUID string, previous map[string]any) error {
	id, err := uuid.Parse(entityUUID)
	if err != nil {
		return err
	}
	ac, _, err := scopedActor(ctx, a.q)
	if err != nil {
		return err
	}
	if err := canTouchProfile(ctx, a.q, ac, id); err != nil {
		return err
	}
	prev, err := snapshotFrom(previous)
	if err != nil {
		return err
	}
	w := writer(ac)
	if prev.Theme != nil || prev.LayoutMode != nil {
		if _, err := a.profiles.UpdatePublicPage(ctx, w, id, pmodel.UpdatePublicPageInput{Theme: prev.Theme, LayoutMode: prev.LayoutMode}); err != nil {
			return err
		}
	}
	if prev.Translations == nil && prev.Addresses == nil && prev.ContactDetails == nil && prev.SocialLinks == nil &&
		prev.AvatarURL == nil && prev.CoverURL == nil {
		return nil
	}
	detail, err := a.profiles.Get(ctx, w, id)
	if err != nil {
		return err
	}
	in := toUpsert(detail)
	if prev.Translations != nil {
		in.Translations = *prev.Translations
	}
	if prev.Addresses != nil {
		in.Addresses = *prev.Addresses
	}
	if prev.ContactDetails != nil {
		in.ContactDetails = *prev.ContactDetails
	}
	if prev.SocialLinks != nil {
		in.SocialLinks = *prev.SocialLinks
	}
	if prev.AvatarURL != nil {
		in.AvatarURL = *prev.AvatarURL
	}
	if prev.CoverURL != nil {
		in.CoverURL = *prev.CoverURL
	}
	// The upsert keeps media on an empty value, so a profile that had none is emptied first;
	// the update below then refreshes caches and the vCard.
	clearAvatar, clearCover := prev.AvatarURL != nil && *prev.AvatarURL == "", prev.CoverURL != nil && *prev.CoverURL == ""
	if clearAvatar || clearCover {
		if _, err := a.q.ClearCompanyProfileMedia(ctx, db.ClearCompanyProfileMediaParams{
			ClearAvatar: clearAvatar, ClearCover: clearCover, AccountID: ac.accountID, ProfileUuid: id,
		}); err != nil {
			return err
		}
	}
	if _, err := a.profiles.Update(ctx, w, id, in); err != nil {
		return err
	}
	if prev.CopiedKey != "" && a.media != nil {
		_ = a.media.Delete(ctx, prev.CopiedKey)
	}
	return nil
}

func (a *BulkAdapter) load(ctx context.Context, ac actor, id uuid.UUID) (pmodel.UpsertProfileInput, error) {
	detail, err := a.profiles.Get(ctx, writer(ac), id)
	if err != nil {
		return pmodel.UpsertProfileInput{}, err
	}
	return toUpsert(detail), nil
}

func (a *BulkAdapter) applyContact(ctx context.Context, ac actor, id uuid.UUID, raw json.RawMessage) (snapshot, error) {
	p, err := a.contactParams(raw)
	if err != nil {
		return snapshot{}, err
	}
	in, err := a.load(ctx, ac, id)
	if err != nil {
		return snapshot{}, err
	}
	before := toUpsertCopy(in)
	var prev snapshot
	address, phone := p.Address, p.Phone
	if p.UseOfficeAddress {
		office, err := a.memberOffice(ctx, ac.accountID, id)
		if err != nil {
			return snapshot{}, err
		}
		address = &model.AddressParams{Label: office.Name, Street: office.Address, City: office.City}
		if phone == nil && strings.TrimSpace(office.Phone) != "" {
			v := strings.TrimSpace(office.Phone)
			phone = &v
		}
	}
	if p.CompanyName != nil {
		for i := range in.Translations {
			in.Translations[i].CompanyName = *p.CompanyName
		}
		prev.Translations = &before.Translations
	}
	if address != nil {
		in.Addresses = setWorkAddress(in.Addresses, *address)
		prev.Addresses = &before.Addresses
	}
	changedContacts := false
	for _, c := range []struct {
		category string
		value    *string
	}{{"phone", phone}, {"email", p.Email}, {"url", p.Website}} {
		if c.value != nil {
			in.ContactDetails = setWorkContact(in.ContactDetails, c.category, *c.value)
			changedContacts = true
		}
	}
	if changedContacts {
		prev.ContactDetails = &before.ContactDetails
	}
	if _, err := a.profiles.Update(ctx, writer(ac), id, in); err != nil {
		return snapshot{}, err
	}
	return prev, nil
}

// memberOffice is the office of the member the profile is linked to.
func (a *BulkAdapter) memberOffice(ctx context.Context, accountID int64, profile uuid.UUID) (db.CompanyOffice, error) {
	target, err := a.q.GetCompanyProfileTarget(ctx, db.GetCompanyProfileTargetParams{AccountID: accountID, ProfileUuid: profile})
	if err != nil {
		return db.CompanyOffice{}, mapDBError(err)
	}
	if !target.OfficeUuid.Valid {
		return db.CompanyOffice{}, invalid("no_office")
	}
	return a.q.GetStructureOffice(ctx, db.GetStructureOfficeParams{AccountID: accountID, Uuid: uuid.UUID(target.OfficeUuid.Bytes)})
}

// toUpsertCopy deep-copies the lists so the snapshot is not changed by later edits.
func toUpsertCopy(in pmodel.UpsertProfileInput) pmodel.UpsertProfileInput {
	out := in
	out.Translations = slices.Clone(in.Translations)
	out.Addresses = slices.Clone(in.Addresses)
	out.ContactDetails = slices.Clone(in.ContactDetails)
	out.SocialLinks = slices.Clone(in.SocialLinks)
	if out.Translations == nil {
		out.Translations = []pmodel.UpsertTranslationInput{}
	}
	if out.Addresses == nil {
		out.Addresses = []pmodel.CreateAddressInput{}
	}
	if out.ContactDetails == nil {
		out.ContactDetails = []pmodel.CreateContactDetailInput{}
	}
	if out.SocialLinks == nil {
		out.SocialLinks = []pmodel.CreateSocialLinkInput{}
	}
	return out
}

// setWorkAddress replaces the work addresses with the company address (first and primary). An
// address with every field empty only removes them.
func setWorkAddress(list []pmodel.CreateAddressInput, ad model.AddressParams) []pmodel.CreateAddressInput {
	out := make([]pmodel.CreateAddressInput, 0, len(list)+1)
	empty := ad.Label == "" && ad.Street == "" && ad.Extended == "" && ad.City == "" && ad.Region == "" &&
		ad.PostalCode == "" && ad.Country == ""
	if !empty {
		out = append(out, pmodel.CreateAddressInput{
			Type: "work", Label: ad.Label, Street: ad.Street, Extended: ad.Extended, City: ad.City,
			Region: ad.Region, PostalCode: ad.PostalCode, Country: ad.Country, IsPrimary: true,
		})
	}
	for _, a := range list {
		if a.Type == "work" {
			continue
		}
		if !empty {
			a.IsPrimary = false
		}
		out = append(out, a)
	}
	for i := range out {
		out[i].DisplayOrder = i
	}
	return out
}

// setWorkContact replaces the work entries of one contact category with value ("" removes
// them). The new entry is primary only when no other entry of the category is.
func setWorkContact(list []pmodel.CreateContactDetailInput, category, value string) []pmodel.CreateContactDetailInput {
	out := make([]pmodel.CreateContactDetailInput, 0, len(list)+1)
	hasPrimary := false
	maxOrder := -1
	for _, c := range list {
		if c.Category == category && c.Type == "work" {
			continue
		}
		if c.Category == category && c.IsPrimary {
			hasPrimary = true
		}
		maxOrder = max(maxOrder, c.DisplayOrder)
		out = append(out, c)
	}
	if value != "" {
		out = append(out, pmodel.CreateContactDetailInput{
			Category: category, Type: "work", Value: value, IsPrimary: !hasPrimary, DisplayOrder: maxOrder + 1,
		})
	}
	return out
}

func (a *BulkAdapter) applyLogo(ctx context.Context, ac actor, id uuid.UUID, raw json.RawMessage) (snapshot, error) {
	p, err := decodeParams[model.LogoParams](raw)
	if err != nil {
		return snapshot{}, err
	}
	mediaURL, err := a.logoSource(ctx, ac, p)
	if err != nil {
		return snapshot{}, err
	}
	in, err := a.load(ctx, ac, id)
	if err != nil {
		return snapshot{}, err
	}
	field := &in.AvatarURL
	if p.TargetField == model.MediaCover {
		field = &in.CoverURL
	}
	old := *field
	var prev snapshot
	if p.TargetField == model.MediaCover {
		prev.CoverURL = &old
	} else {
		prev.AvatarURL = &old
	}
	if old == mediaURL {
		return prev, nil // the profile already shows the branding file
	}
	*field = mediaURL
	if _, err := a.profiles.Update(ctx, writer(ac), id, in); err != nil {
		return snapshot{}, err
	}
	return prev, nil
}

func (a *BulkAdapter) applyTheme(ctx context.Context, ac actor, id uuid.UUID, raw json.RawMessage) (snapshot, error) {
	p, err := a.themeParams(raw)
	if err != nil {
		return snapshot{}, err
	}
	w := writer(ac)
	page, err := a.profiles.GetPublicPage(ctx, w, id)
	if err != nil {
		return snapshot{}, err
	}
	var prev snapshot
	if p.Theme != nil {
		prev.Theme = &page.Theme
	}
	if p.LayoutMode != nil {
		prev.LayoutMode = &page.LayoutMode
	}
	if _, err := a.profiles.UpdatePublicPage(ctx, w, id, pmodel.UpdatePublicPageInput{Theme: p.Theme, LayoutMode: p.LayoutMode}); err != nil {
		return snapshot{}, err
	}
	return prev, nil
}

func (a *BulkAdapter) applyLinks(ctx context.Context, ac actor, id uuid.UUID, raw json.RawMessage) (snapshot, error) {
	p, err := a.linksParams(raw)
	if err != nil {
		return snapshot{}, err
	}
	in, err := a.load(ctx, ac, id)
	if err != nil {
		return snapshot{}, err
	}
	before := toUpsertCopy(in)
	links := slices.Clone(in.SocialLinks)
	for _, l := range p.Links {
		pl, err := a.platform(ctx, l.Platform)
		if err != nil {
			return snapshot{}, err
		}
		if slices.ContainsFunc(links, func(x pmodel.CreateSocialLinkInput) bool {
			return strings.EqualFold(strings.TrimSpace(x.URL), l.URL)
		}) {
			continue
		}
		next := pmodel.CreateSocialLinkInput{Platform: l.Platform, Label: l.Label, URL: l.URL, IsActive: true}
		if pl != nil {
			pid := pl.ID
			next.PlatformID = &pid
			next.IconURL = pl.IconUrl
		}
		if p.ReplaceSamePlatform && l.Platform != "custom" {
			if i := slices.IndexFunc(links, func(x pmodel.CreateSocialLinkInput) bool { return x.Platform == l.Platform }); i >= 0 {
				next.DisplayOrder = links[i].DisplayOrder
				next.IsActive = links[i].IsActive
				links[i] = next
				continue
			}
		}
		next.DisplayOrder = len(links)
		links = append(links, next)
	}
	in.SocialLinks = links
	if _, err := a.profiles.Update(ctx, writer(ac), id, in); err != nil {
		return snapshot{}, err
	}
	return snapshot{SocialLinks: &before.SocialLinks}, nil
}
