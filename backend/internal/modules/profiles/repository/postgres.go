package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Postgres struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

func NewPostgres(pool *pgxpool.Pool, q *db.Queries) *Postgres {
	return &Postgres{pool: pool, q: q}
}

func (r *Postgres) withTx(ctx context.Context, fn func(q *db.Queries) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(r.q.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Postgres) ListByAccountID(ctx context.Context, accountID int64, limit, offset int32, q, status string) ([]model.ProfileSummary, int64, error) {
	var qParam *string
	if s := strings.TrimSpace(q); s != "" {
		qParam = &s
	}

	rows, err := r.q.ListProfilesByAccountID(ctx, db.ListProfilesByAccountIDParams{
		AccountID:   accountID,
		Q:           textToPg(qParam),
		LimitCount:  limit,
		OffsetCount: offset,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list profiles: %w", err)
	}

	total, err := r.q.CountProfilesByAccountID(ctx, db.CountProfilesByAccountIDParams{
		AccountID: accountID,
		Q:         textToPg(qParam),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("count profiles: %w", err)
	}

	items := make([]model.ProfileSummary, 0, len(rows))
	for _, row := range rows {
		items = append(items, model.ProfileSummary{
			UUID:           row.Uuid,
			FirstName:      row.FirstName,
			LastName:       row.LastName,
			Title:          row.Title,
			CompanyName:    row.CompanyName,
			AvatarURL:      row.AvatarUrl,
			PrimaryLocale:  row.PrimaryLocale,
			IsPrimary:      row.IsPrimary,
			CompletionRate: int(row.CompletionRate),
			Username:       row.Username,
			CreatedAt:      row.CreatedAt.Time,
			UpdatedAt:      row.UpdatedAt.Time,
		})
	}

	return items, total, nil
}

func (r *Postgres) GetByUUID(ctx context.Context, accountID int64, profileUUID uuid.UUID) (*model.ProfileDetail, error) {
	p, err := r.q.GetProfileByUUIDForAccount(ctx, db.GetProfileByUUIDForAccountParams{Uuid: profileUUID, AccountID: accountID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get profile: %w", err)
	}

	translations, err := r.q.ListProfileTranslations(ctx, p.ID)
	if err != nil {
		return nil, fmt.Errorf("list profile translations: %w", err)
	}

	addresses, err := r.q.ListProfileAddresses(ctx, p.ID)
	if err != nil {
		return nil, fmt.Errorf("list profile addresses: %w", err)
	}

	contacts, err := r.q.ListProfileContactDetails(ctx, p.ID)
	if err != nil {
		return nil, fmt.Errorf("list profile contact details: %w", err)
	}

	socials, err := r.q.ListProfileSocialLinks(ctx, p.ID)
	if err != nil {
		return nil, fmt.Errorf("list profile social links: %w", err)
	}

	detail := mapProfileDetail(p, translations, addresses, contacts, socials, uuid.Nil)
	return &detail, nil
}

func (r *Postgres) GetPrimaryByAccountID(ctx context.Context, accountID int64) (*model.ProfileDetail, error) {
	p, err := r.q.GetPrimaryProfileByAccountID(ctx, accountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get primary profile: %w", err)
	}

	translations, err := r.q.ListProfileTranslations(ctx, p.ID)
	if err != nil {
		return nil, fmt.Errorf("list profile translations: %w", err)
	}

	addresses, err := r.q.ListProfileAddresses(ctx, p.ID)
	if err != nil {
		return nil, fmt.Errorf("list profile addresses: %w", err)
	}

	contacts, err := r.q.ListProfileContactDetails(ctx, p.ID)
	if err != nil {
		return nil, fmt.Errorf("list profile contact details: %w", err)
	}

	socials, err := r.q.ListProfileSocialLinks(ctx, p.ID)
	if err != nil {
		return nil, fmt.Errorf("list profile social links: %w", err)
	}

	detail := mapProfileDetail(p, translations, addresses, contacts, socials, uuid.Nil)
	return &detail, nil
}

func (r *Postgres) Create(ctx context.Context, accountID, creatorUserID int64, userUUID uuid.UUID, in model.UpsertProfileInput, completionRate int) (*model.ProfileDetail, error) {
	var detail *model.ProfileDetail

	err := r.withTx(ctx, func(q *db.Queries) error {
		if in.IsPrimary {
			if err := q.ClearPrimaryProfilesForAccount(ctx, accountID); err != nil {
				return fmt.Errorf("clear primary profiles: %w", err)
			}
		}

		var bday pgtype.Date
		if in.Birthday != nil && *in.Birthday != "" {
			t, err := time.Parse("2006-01-02", *in.Birthday)
			if err == nil {
				bday = pgtype.Date{Time: t, Valid: true}
			}
		}

		primLocale := in.PrimaryLocale
		if primLocale == "" {
			primLocale = "tr"
		}

		p, err := q.CreateProfile(ctx, db.CreateProfileParams{
			AccountID:      accountID,
			UserID:         pgtype.Int8{Int64: creatorUserID, Valid: true},
			PrimaryLocale:  primLocale,
			Birthday:       bday,
			Gender:         in.Gender,
			AvatarUrl:      in.AvatarURL,
			CoverUrl:       in.CoverURL,
			IsPrimary:      in.IsPrimary,
			CompletionRate: int32(completionRate),
		})
		if err != nil {
			return fmt.Errorf("create profile: %w", err)
		}

		// Insert Translations
		createdTranslations := make([]db.ProfileTranslation, 0, len(in.Translations))
		for _, tr := range in.Translations {
			if tr.Locale == "" {
				continue
			}
			isDef := tr.IsDefault || tr.Locale == primLocale
			row, err := q.CreateProfileTranslation(ctx, db.CreateProfileTranslationParams{
				ProfileID:   p.ID,
				Locale:      tr.Locale,
				Prefix:      tr.Prefix,
				FirstName:   tr.FirstName,
				LastName:    tr.LastName,
				MiddleName:  tr.MiddleName,
				Suffix:      tr.Suffix,
				Nickname:    tr.Nickname,
				CompanyName: tr.CompanyName,
				Department:  tr.Department,
				Title:       tr.Title,
				Role:        tr.Role,
				Bio:         tr.Bio,
				Note:        tr.Note,
				IsDefault:   isDef,
			})
			if err != nil {
				return fmt.Errorf("create translation %s: %w", tr.Locale, err)
			}
			createdTranslations = append(createdTranslations, row)
		}

		// Insert Addresses
		createdAddresses := make([]db.ProfileAddress, 0, len(in.Addresses))
		for i, addr := range in.Addresses {
			a, err := q.CreateProfileAddress(ctx, db.CreateProfileAddressParams{
				ProfileID:    p.ID,
				Type:         addr.Type,
				Label:        addr.Label,
				Street:       addr.Street,
				Extended:     addr.Extended,
				City:         addr.City,
				Region:       addr.Region,
				PostalCode:   addr.PostalCode,
				Country:      addr.Country,
				IsPrimary:    addr.IsPrimary,
				DisplayOrder: int32(i),
			})
			if err != nil {
				return fmt.Errorf("create address: %w", err)
			}
			createdAddresses = append(createdAddresses, a)
		}

		// Insert Contacts
		createdContacts := make([]db.ProfileContactDetail, 0, len(in.ContactDetails))
		for i, cd := range in.ContactDetails {
			c, err := q.CreateProfileContactDetail(ctx, db.CreateProfileContactDetailParams{
				ProfileID:    p.ID,
				Category:     cd.Category,
				Type:         cd.Type,
				Label:        cd.Label,
				Value:        cd.Value,
				IsPrimary:    cd.IsPrimary,
				DisplayOrder: int32(i),
			})
			if err != nil {
				return fmt.Errorf("create contact: %w", err)
			}
			createdContacts = append(createdContacts, c)
		}

		// Insert Social Links
		createdSocials := make([]db.ProfileSocialLink, 0, len(in.SocialLinks))
		for i, sl := range in.SocialLinks {
			var platID pgtype.Int8
			if sl.PlatformID != nil && *sl.PlatformID > 0 {
				platID = pgtype.Int8{Int64: *sl.PlatformID, Valid: true}
			}
			s, err := q.CreateProfileSocialLink(ctx, db.CreateProfileSocialLinkParams{
				ProfileID:    p.ID,
				PlatformID:   platID,
				Platform:     sl.Platform,
				Label:        sl.Label,
				Url:          sl.URL,
				IconUrl:      sl.IconURL,
				DisplayOrder: int32(i),
				IsActive:     sl.IsActive,
			})
			if err != nil {
				return fmt.Errorf("create social link: %w", err)
			}
			createdSocials = append(createdSocials, s)
		}

		res := mapProfileDetail(p, createdTranslations, createdAddresses, createdContacts, createdSocials, userUUID)
		detail = &res
		return nil
	})

	if err != nil {
		return nil, err
	}
	return detail, nil
}

func (r *Postgres) Update(ctx context.Context, accountID int64, profileUUID uuid.UUID, userUUID uuid.UUID, in model.UpsertProfileInput, completionRate int) (*model.ProfileDetail, error) {
	var detail *model.ProfileDetail

	err := r.withTx(ctx, func(q *db.Queries) error {
		existing, err := q.GetProfileByUUIDForAccount(ctx, db.GetProfileByUUIDForAccountParams{Uuid: profileUUID, AccountID: accountID})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return fmt.Errorf("get profile: %w", err)
		}

		if in.IsPrimary && !existing.IsPrimary {
			if err := q.ClearPrimaryProfilesForAccount(ctx, accountID); err != nil {
				return fmt.Errorf("clear primary profiles: %w", err)
			}
		}

		var bday pgtype.Date
		if in.Birthday != nil && *in.Birthday != "" {
			t, err := time.Parse("2006-01-02", *in.Birthday)
			if err == nil {
				bday = pgtype.Date{Time: t, Valid: true}
			}
		}

		primLocale := in.PrimaryLocale
		if primLocale == "" {
			primLocale = existing.PrimaryLocale
		}
		compRate := int32(completionRate)

		p, err := q.UpdateProfile(ctx, db.UpdateProfileParams{
			Uuid:           profileUUID,
			AccountID:      accountID,
			PrimaryLocale:  textToPg(&primLocale),
			Birthday:       bday,
			Gender:         textToPg(&in.Gender),
			AvatarUrl:      textToPg(&in.AvatarURL),
			CoverUrl:       textToPg(&in.CoverURL),
			IsPrimary:      boolToPg(&in.IsPrimary),
			CompletionRate: int4ToPg(&compRate),
		})
		if err != nil {
			return fmt.Errorf("update profile: %w", err)
		}

		// Replace Translations
		if err := q.DeleteProfileTranslations(ctx, p.ID); err != nil {
			return fmt.Errorf("delete translations: %w", err)
		}
		createdTranslations := make([]db.ProfileTranslation, 0, len(in.Translations))
		for _, tr := range in.Translations {
			if tr.Locale == "" {
				continue
			}
			isDef := tr.IsDefault || tr.Locale == primLocale
			row, err := q.CreateProfileTranslation(ctx, db.CreateProfileTranslationParams{
				ProfileID:   p.ID,
				Locale:      tr.Locale,
				Prefix:      tr.Prefix,
				FirstName:   tr.FirstName,
				LastName:    tr.LastName,
				MiddleName:  tr.MiddleName,
				Suffix:      tr.Suffix,
				Nickname:    tr.Nickname,
				CompanyName: tr.CompanyName,
				Department:  tr.Department,
				Title:       tr.Title,
				Role:        tr.Role,
				Bio:         tr.Bio,
				Note:        tr.Note,
				IsDefault:   isDef,
			})
			if err != nil {
				return fmt.Errorf("create translation %s: %w", tr.Locale, err)
			}
			createdTranslations = append(createdTranslations, row)
		}

		// Replace Addresses
		if err := q.DeleteProfileAddresses(ctx, p.ID); err != nil {
			return fmt.Errorf("delete addresses: %w", err)
		}
		createdAddresses := make([]db.ProfileAddress, 0, len(in.Addresses))
		for i, addr := range in.Addresses {
			a, err := q.CreateProfileAddress(ctx, db.CreateProfileAddressParams{
				ProfileID:    p.ID,
				Type:         addr.Type,
				Label:        addr.Label,
				Street:       addr.Street,
				Extended:     addr.Extended,
				City:         addr.City,
				Region:       addr.Region,
				PostalCode:   addr.PostalCode,
				Country:      addr.Country,
				IsPrimary:    addr.IsPrimary,
				DisplayOrder: int32(i),
			})
			if err != nil {
				return fmt.Errorf("create address: %w", err)
			}
			createdAddresses = append(createdAddresses, a)
		}

		// Replace Contacts
		if err := q.DeleteProfileContactDetails(ctx, p.ID); err != nil {
			return fmt.Errorf("delete contacts: %w", err)
		}
		createdContacts := make([]db.ProfileContactDetail, 0, len(in.ContactDetails))
		for i, cd := range in.ContactDetails {
			c, err := q.CreateProfileContactDetail(ctx, db.CreateProfileContactDetailParams{
				ProfileID:    p.ID,
				Category:     cd.Category,
				Type:         cd.Type,
				Label:        cd.Label,
				Value:        cd.Value,
				IsPrimary:    cd.IsPrimary,
				DisplayOrder: int32(i),
			})
			if err != nil {
				return fmt.Errorf("create contact: %w", err)
			}
			createdContacts = append(createdContacts, c)
		}

		// Replace Social Links
		if err := q.DeleteProfileSocialLinks(ctx, p.ID); err != nil {
			return fmt.Errorf("delete social links: %w", err)
		}
		createdSocials := make([]db.ProfileSocialLink, 0, len(in.SocialLinks))
		for i, sl := range in.SocialLinks {
			var platID pgtype.Int8
			if sl.PlatformID != nil && *sl.PlatformID > 0 {
				platID = pgtype.Int8{Int64: *sl.PlatformID, Valid: true}
			}
			s, err := q.CreateProfileSocialLink(ctx, db.CreateProfileSocialLinkParams{
				ProfileID:    p.ID,
				PlatformID:   platID,
				Platform:     sl.Platform,
				Label:        sl.Label,
				Url:          sl.URL,
				IconUrl:      sl.IconURL,
				DisplayOrder: int32(i),
				IsActive:     sl.IsActive,
			})
			if err != nil {
				return fmt.Errorf("create social link: %w", err)
			}
			createdSocials = append(createdSocials, s)
		}

		res := mapProfileDetail(p, createdTranslations, createdAddresses, createdContacts, createdSocials, userUUID)
		detail = &res
		return nil
	})

	if err != nil {
		return nil, err
	}
	return detail, nil
}

func (r *Postgres) UpdateAvatar(ctx context.Context, accountID int64, profileUUID uuid.UUID, avatarURL string) (*model.ProfileDetail, error) {
	var detail *model.ProfileDetail
	err := r.withTx(ctx, func(q *db.Queries) error {
		p, err := q.UpdateProfile(ctx, db.UpdateProfileParams{
			Uuid:      profileUUID,
			AccountID: accountID,
			AvatarUrl: textToPg(&avatarURL),
		})
		if err != nil {
			return err
		}
		translations, err := q.ListProfileTranslations(ctx, p.ID)
		if err != nil {
			return err
		}
		addresses, err := q.ListProfileAddresses(ctx, p.ID)
		if err != nil {
			return err
		}
		contacts, err := q.ListProfileContactDetails(ctx, p.ID)
		if err != nil {
			return err
		}
		socials, err := q.ListProfileSocialLinks(ctx, p.ID)
		if err != nil {
			return err
		}
		res := mapProfileDetail(p, translations, addresses, contacts, socials, uuid.Nil)
		detail = &res
		return nil
	})
	return detail, err
}

func (r *Postgres) UpdateCover(ctx context.Context, accountID int64, profileUUID uuid.UUID, coverURL string) (*model.ProfileDetail, error) {
	var detail *model.ProfileDetail
	err := r.withTx(ctx, func(q *db.Queries) error {
		p, err := q.UpdateProfile(ctx, db.UpdateProfileParams{
			Uuid:      profileUUID,
			AccountID: accountID,
			CoverUrl:  textToPg(&coverURL),
		})
		if err != nil {
			return err
		}
		translations, err := q.ListProfileTranslations(ctx, p.ID)
		if err != nil {
			return err
		}
		addresses, err := q.ListProfileAddresses(ctx, p.ID)
		if err != nil {
			return err
		}
		contacts, err := q.ListProfileContactDetails(ctx, p.ID)
		if err != nil {
			return err
		}
		socials, err := q.ListProfileSocialLinks(ctx, p.ID)
		if err != nil {
			return err
		}
		res := mapProfileDetail(p, translations, addresses, contacts, socials, uuid.Nil)
		detail = &res
		return nil
	})
	return detail, err
}

func (r *Postgres) Delete(ctx context.Context, accountID int64, profileUUID uuid.UUID) error {
	return r.q.DeleteProfile(ctx, db.DeleteProfileParams{Uuid: profileUUID, AccountID: accountID})
}

func (r *Postgres) SetPrimary(ctx context.Context, accountID int64, profileUUID uuid.UUID) error {
	return r.withTx(ctx, func(q *db.Queries) error {
		if err := q.ClearPrimaryProfilesForAccount(ctx, accountID); err != nil {
			return fmt.Errorf("clear primary profiles: %w", err)
		}
		if err := q.SetPrimaryProfileByUUID(ctx, db.SetPrimaryProfileByUUIDParams{Uuid: profileUUID, AccountID: accountID}); err != nil {
			return fmt.Errorf("set primary profile: %w", err)
		}
		return nil
	})
}

func textToPg(s *string) pgtype.Text {
	if s == nil || *s == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: *s, Valid: true}
}

func boolToPg(b *bool) pgtype.Bool {
	if b == nil {
		return pgtype.Bool{Valid: false}
	}
	return pgtype.Bool{Bool: *b, Valid: true}
}

func int4ToPg(i *int32) pgtype.Int4 {
	if i == nil {
		return pgtype.Int4{Valid: false}
	}
	return pgtype.Int4{Int32: *i, Valid: true}
}

func mapProfileDetail(
	p db.Profile,
	translations []db.ProfileTranslation,
	addresses []db.ProfileAddress,
	contacts []db.ProfileContactDetail,
	socials []db.ProfileSocialLink,
	userUUID uuid.UUID,
) model.ProfileDetail {
	var bdayStr *string
	if p.Birthday.Valid {
		s := p.Birthday.Time.Format("2006-01-02")
		bdayStr = &s
	}

	trList := make([]model.ProfileTranslationDTO, len(translations))
	for i, tr := range translations {
		trList[i] = model.ProfileTranslationDTO{
			Locale:      tr.Locale,
			Prefix:      tr.Prefix,
			FirstName:   tr.FirstName,
			LastName:    tr.LastName,
			MiddleName:  tr.MiddleName,
			Suffix:      tr.Suffix,
			Nickname:    tr.Nickname,
			CompanyName: tr.CompanyName,
			Department:  tr.Department,
			Title:       tr.Title,
			Role:        tr.Role,
			Bio:         tr.Bio,
			Note:        tr.Note,
			IsDefault:   tr.IsDefault,
		}
	}

	addrList := make([]model.AddressDTO, len(addresses))
	for i, a := range addresses {
		addrList[i] = model.AddressDTO{
			UUID:         a.Uuid,
			Type:         a.Type,
			Label:        a.Label,
			Street:       a.Street,
			Extended:     a.Extended,
			City:         a.City,
			Region:       a.Region,
			PostalCode:   a.PostalCode,
			Country:      a.Country,
			IsPrimary:    a.IsPrimary,
			DisplayOrder: int(a.DisplayOrder),
		}
	}

	contactList := make([]model.ContactDetailDTO, len(contacts))
	for i, c := range contacts {
		contactList[i] = model.ContactDetailDTO{
			UUID:         c.Uuid,
			Category:     c.Category,
			Type:         c.Type,
			Label:        c.Label,
			Value:        c.Value,
			IsPrimary:    c.IsPrimary,
			DisplayOrder: int(c.DisplayOrder),
		}
	}

	socialList := make([]model.SocialLinkDTO, len(socials))
	for i, s := range socials {
		var platID *int64
		if s.PlatformID.Valid {
			v := s.PlatformID.Int64
			platID = &v
		}
		socialList[i] = model.SocialLinkDTO{
			UUID:         s.Uuid,
			PlatformID:   platID,
			Platform:     s.Platform,
			Label:        s.Label,
			URL:          s.Url,
			IconURL:      s.IconUrl,
			DisplayOrder: int(s.DisplayOrder),
			IsActive:     s.IsActive,
		}
	}

	return model.ProfileDetail{
		UUID:           p.Uuid,
		UserUUID:       userUUID,
		PrimaryLocale:  p.PrimaryLocale,
		Birthday:       bdayStr,
		Gender:         p.Gender,
		AvatarURL:      p.AvatarUrl,
		CoverURL:       p.CoverUrl,
		IsPrimary:      p.IsPrimary,
		CompletionRate: int(p.CompletionRate),
		Translations:   trList,
		Addresses:      addrList,
		ContactDetails: contactList,
		SocialLinks:    socialList,
		CreatedAt:      p.CreatedAt.Time,
		UpdatedAt:      p.UpdatedAt.Time,
	}
}

// SetCompletionRate stores a recomputed completion score (media uploads change it outside the
// upsert payload).
func (r *Postgres) SetCompletionRate(ctx context.Context, accountID int64, profileUUID uuid.UUID, rate int) error {
	v := int32(rate)
	_, err := r.q.UpdateProfile(ctx, db.UpdateProfileParams{
		Uuid:           profileUUID,
		AccountID:      accountID,
		CompletionRate: int4ToPg(&v),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
