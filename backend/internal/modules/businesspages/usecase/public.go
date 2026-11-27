package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"slices"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/businesspages/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/businesspages/repository"
	pmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/publicpath"
	"github.com/google/uuid"
)

// featureOn reports whether the account's plan has business_page.enabled. A failed lookup or an
// unwired checker hides the page (fail-closed).
func (s *Service) featureOn(ctx context.Context, accountID int64) bool {
	if s.features == nil {
		return false
	}
	ok, err := s.features.CheckBool(ctx, accountID, false, entitlement.FeatureBusinessPage)
	return err == nil && ok
}

func (s *Service) companyBranding(ctx context.Context, accountID int64) pmodel.AccountBranding {
	if s.branding == nil {
		return pmodel.AccountBranding{}
	}
	b, err := s.branding.AccountBranding(ctx, accountID)
	if err != nil || b == nil {
		return pmodel.AccountBranding{}
	}
	return *b
}

// Public returns the visitor view of a business page (no auth). Only a published page of a live
// company whose plan has the feature is served; a draft, an unknown or a suspended one is
// ErrNotFound. A renamed username returns only RedirectTo.
func (s *Service) Public(ctx context.Context, username string) (*model.PublicPage, error) {
	route, err := s.store.PublicRoute(ctx, publicpath.NormalizeUsername(username))
	if errors.Is(err, repository.ErrNotFound) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	switch route.Status {
	case "active":
	case "retired":
		live, err := s.store.LiveRoute(ctx, route.AccountID)
		if err != nil {
			return nil, err
		}
		if live == nil || live.Status != "active" {
			return nil, model.ErrNotFound
		}
		return &model.PublicPage{Username: route.Path, RedirectTo: live.Path}, nil
	default:
		return nil, model.ErrNotFound
	}
	if !s.featureOn(ctx, route.AccountID) {
		return nil, model.ErrNotFound
	}
	row, err := s.store.Page(ctx, route.AccountID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, model.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if row.Status != model.StatusPublished {
		return nil, model.ErrNotFound
	}
	blocks, err := s.store.Blocks(ctx, route.AccountID, row.ID)
	if err != nil {
		return nil, err
	}

	brand := s.companyBranding(ctx, route.AccountID)
	out := &model.PublicPage{
		Username: route.Path, PageUUID: row.Uuid, CompanyName: route.AccountName,
		PrimaryLocale: row.PrimaryLocale, Translations: map[string]model.PageText{},
		Theme: row.Theme, PrimaryColor: brand.PrimaryColor,
		LogoURL: brand.LogoURL, CoverURL: brand.CoverURL,
		Blocks: []model.PublicBlock{},
	}
	_ = json.Unmarshal(row.Translations, &out.Translations)
	if out.Theme == "" && slices.Contains(pmodel.PublicThemes, brand.Theme) {
		out.Theme = brand.Theme
	}
	// A premium theme renders its basic stand-in without design.premium (TEC-947).
	if fallback, premium := pmodel.PremiumThemes[out.Theme]; premium {
		ok, err := s.features.CheckBool(ctx, route.AccountID, false, entitlement.FeatureDesignPremium)
		if err != nil || !ok {
			out.Theme = fallback
		}
	}

	for _, b := range blocks {
		if !b.Enabled {
			continue
		}
		pb := model.PublicBlock{Type: b.Type, Translations: map[string]model.BlockText{}}
		_ = json.Unmarshal(b.Translations, &pb.Translations)
		switch b.Type {
		case model.BlockHeader:
			var c model.HeaderConfig
			_ = json.Unmarshal(b.Config, &c)
			ids := []uuid.UUID{}
			for _, id := range []*uuid.UUID{c.LogoMediaUUID, c.CoverMediaUUID} {
				if id != nil {
					ids = append(ids, *id)
				}
			}
			keys, err := s.store.MediaKeys(ctx, route.AccountID, ids)
			if err != nil {
				return nil, err
			}
			// A deleted library file falls back to the company branding.
			if c.LogoMediaUUID != nil && keys[*c.LogoMediaUUID] != "" {
				out.LogoURL = s.mediaURL(keys[*c.LogoMediaUUID])
			}
			if c.CoverMediaUUID != nil && keys[*c.CoverMediaUUID] != "" {
				out.CoverURL = s.mediaURL(keys[*c.CoverMediaUUID])
			}
		case model.BlockButton:
			var c model.ButtonConfig
			_ = json.Unmarshal(b.Config, &c)
			pb.Button = &model.PublicButton{Target: c.Target, Value: c.Value, Icon: c.Icon}
		case model.BlockSocial:
			var c model.SocialConfig
			_ = json.Unmarshal(b.Config, &c)
			slugs := make([]string, 0, len(c.Links))
			for _, l := range c.Links {
				slugs = append(slugs, l.Platform)
			}
			icons, err := s.store.SocialIcons(ctx, slugs)
			if err != nil {
				return nil, err
			}
			pb.Social = make([]model.PublicSocialLink, 0, len(c.Links))
			for _, l := range c.Links {
				pb.Social = append(pb.Social, model.PublicSocialLink{Platform: l.Platform, URL: l.URL, IconURL: icons[l.Platform]})
			}
		case model.BlockOffices:
			rows, err := s.store.Offices(ctx, route.AccountID)
			if err != nil {
				return nil, err
			}
			pb.Offices = make([]model.PublicOffice, 0, len(rows))
			for _, o := range rows {
				pb.Offices = append(pb.Offices, model.PublicOffice{Name: o.Name, City: o.City, District: o.District, Address: o.Address, Phone: o.Phone})
			}
		case model.BlockTeam:
			rows, err := s.store.Team(ctx, route.AccountID, row.PrimaryLocale)
			if err != nil {
				return nil, err
			}
			pb.Team = make([]model.PublicTeamMember, 0, len(rows))
			for _, m := range rows {
				pb.Team = append(pb.Team, model.PublicTeamMember{Username: m.Username, FirstName: m.FirstName, LastName: m.LastName, Title: m.Title, AvatarURL: m.AvatarUrl})
			}
		case model.BlockBankAccounts:
			var c model.BankAccountsConfig
			_ = json.Unmarshal(b.Config, &c)
			rows, err := s.store.BankAccounts(ctx, route.AccountID, c.BankAccountUUIDs)
			if err != nil {
				return nil, err
			}
			pb.BankAccounts = make([]model.PublicBankAccount, 0, len(rows))
			for _, a := range rows {
				pb.BankAccounts = append(pb.BankAccounts, model.PublicBankAccount{
					BankName: a.BankName, BankIconURL: a.BankIconUrl, BankBrandColor: a.BankBrandColor,
					AccountHolder: a.AccountHolder, IBAN: a.Iban, Currency: a.Currency, Title: a.Title,
				})
			}
		}
		out.Blocks = append(out.Blocks, pb)
	}
	return out, nil
}
