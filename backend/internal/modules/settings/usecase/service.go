package usecase

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/appversion"
	"github.com/jackc/pgx/v5/pgtype"
)

var ErrNotFound = errors.New("not found")

// Service manages app_settings singleton.
type Service struct {
	q *db.Queries
}

// New creates a settings service.
func New(q *db.Queries) *Service {
	return &Service{q: q}
}

// LetterheadPublic is the public branding subset.
type LetterheadPublic struct {
	CompanyName  string  `json:"company_name"`
	Tagline      string  `json:"tagline"`
	PrimaryColor string  `json:"primary_color"`
	LogoURL      *string `json:"logo_url,omitempty"`
}

// Settings is the full settings payload.
type Settings struct {
	CompanyName  string  `json:"company_name"`
	Tagline      string  `json:"tagline"`
	PrimaryColor string  `json:"primary_color"`
	Address      string  `json:"address"`
	Phone        string  `json:"phone"`
	Email        string  `json:"email"`
	Website      string  `json:"website"`
	FooterText   string  `json:"footer_text"`
	PaperSize    string  `json:"paper_size"`
	LogoURL      *string `json:"logo_url,omitempty"`
	// Mobile app version gate and store links (TEC-1178); "" = not set.
	MobileMinVersion      string `json:"mobile_min_version"`
	MobileLatestVersion   string `json:"mobile_latest_version"`
	MobileIOSStoreURL     string `json:"mobile_ios_store_url"`
	MobileAndroidStoreURL string `json:"mobile_android_store_url"`
}

// MobileStoreURLs are the store pages the app opens on an update prompt.
type MobileStoreURLs struct {
	IOS     string `json:"ios"`
	Android string `json:"android"`
}

// MobileConfig is the `mobile` object of GET /v1/app/config (TEC-1178).
type MobileConfig struct {
	MinVersion    string          `json:"min_version"`
	LatestVersion string          `json:"latest_version"`
	StoreURLs     MobileStoreURLs `json:"store_urls"`
}

func mapSettings(row db.AppSetting) Settings {
	var logo *string
	if row.LogoObjectKey.Valid && row.LogoObjectKey.String != "" {
		u := "/v1/platform/settings/logo"
		logo = &u
	}
	return Settings{
		CompanyName: row.CompanyName, Tagline: row.Tagline, PrimaryColor: row.PrimaryColor,
		Address: row.Address, Phone: row.Phone, Email: row.Email, Website: row.Website,
		FooterText: row.FooterText, PaperSize: row.PaperSize, LogoURL: logo,
		MobileMinVersion: row.MobileMinVersion, MobileLatestVersion: row.MobileLatestVersion,
		MobileIOSStoreURL: row.MobileIosStoreUrl, MobileAndroidStoreURL: row.MobileAndroidStoreUrl,
	}
}

// Mobile returns the mobile app version settings published in GET /v1/app/config.
func (s *Service) Mobile(ctx context.Context) (MobileConfig, error) {
	row, err := s.q.GetAppSettings(ctx)
	if err != nil {
		return MobileConfig{}, err
	}
	return MobileConfig{
		MinVersion:    row.MobileMinVersion,
		LatestVersion: row.MobileLatestVersion,
		StoreURLs:     MobileStoreURLs{IOS: row.MobileIosStoreUrl, Android: row.MobileAndroidStoreUrl},
	}, nil
}

// MinMobileVersion is the appversion.Gate source: the configured minimum ("" = no gate).
func (s *Service) MinMobileVersion(ctx context.Context) (string, error) {
	m, err := s.Mobile(ctx)
	return m.MinVersion, err
}

// Get returns current settings.
func (s *Service) Get(ctx context.Context) (Settings, error) {
	row, err := s.q.GetAppSettings(ctx)
	if err != nil {
		return Settings{}, err
	}
	return mapSettings(row), nil
}

// PublicLetterhead returns branding for app config.
func (s *Service) PublicLetterhead(ctx context.Context) (LetterheadPublic, error) {
	row, err := s.q.GetAppSettings(ctx)
	if err != nil {
		return LetterheadPublic{}, err
	}
	var logo *string
	if row.LogoObjectKey.Valid && row.LogoObjectKey.String != "" {
		u := "/v1/platform/settings/logo"
		logo = &u
	}
	return LetterheadPublic{
		CompanyName: row.CompanyName, Tagline: row.Tagline,
		PrimaryColor: row.PrimaryColor, LogoURL: logo,
	}, nil
}

// PatchInput partial update.
type PatchInput struct {
	CompanyName  *string `json:"company_name"`
	Tagline      *string `json:"tagline"`
	PrimaryColor *string `json:"primary_color"`
	Address      *string `json:"address"`
	Phone        *string `json:"phone"`
	Email        *string `json:"email"`
	Website      *string `json:"website"`
	FooterText   *string `json:"footer_text"`
	PaperSize    *string `json:"paper_size"`

	MobileMinVersion      *string `json:"mobile_min_version"`
	MobileLatestVersion   *string `json:"mobile_latest_version"`
	MobileIOSStoreURL     *string `json:"mobile_ios_store_url"`
	MobileAndroidStoreURL *string `json:"mobile_android_store_url"`
}

// FieldError is one invalid PatchInput field.
type FieldError struct {
	Field   string
	Message string
}

// Validate checks the mobile fields: versions are "" or major[.minor[.patch]], store URLs are ""
// or an absolute https URL, and latest_version is not below min_version. current fills in the
// fields the patch leaves out, so the min/latest order is checked against the stored values.
func (in PatchInput) Validate(current Settings) []FieldError {
	var out []FieldError
	minV, latestV := current.MobileMinVersion, current.MobileLatestVersion
	for _, f := range []struct {
		name string
		val  *string
		dst  *string
	}{
		{"mobile_min_version", in.MobileMinVersion, &minV},
		{"mobile_latest_version", in.MobileLatestVersion, &latestV},
	} {
		if f.val == nil {
			continue
		}
		v := strings.TrimSpace(*f.val)
		if v != "" && !appversion.Valid(v) {
			out = append(out, FieldError{f.name, "must look like 1.2.3"})
			continue
		}
		*f.dst = v
	}
	if len(out) == 0 && minV != "" && latestV != "" {
		a, _ := appversion.Parse(latestV)
		b, _ := appversion.Parse(minV)
		if a.Less(b) {
			out = append(out, FieldError{"mobile_latest_version", "must not be lower than mobile_min_version"})
		}
	}
	for _, f := range []struct {
		name string
		val  *string
	}{
		{"mobile_ios_store_url", in.MobileIOSStoreURL},
		{"mobile_android_store_url", in.MobileAndroidStoreURL},
	} {
		if f.val == nil {
			continue
		}
		if v := strings.TrimSpace(*f.val); v != "" && !isStoreURL(v) {
			out = append(out, FieldError{f.name, "must be an https URL"})
		}
	}
	return out
}

func isStoreURL(v string) bool {
	if len(v) > 512 {
		return false
	}
	u, err := url.Parse(v)
	return err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil
}

func trimmed(p *string) *string {
	if p == nil {
		return nil
	}
	v := strings.TrimSpace(*p)
	return &v
}

// Patch updates settings.
func (s *Service) Patch(ctx context.Context, in PatchInput) (Settings, error) {
	row, err := s.q.UpdateAppSettings(ctx, db.UpdateAppSettingsParams{
		CompanyName: textNarg(in.CompanyName), Tagline: textNarg(in.Tagline),
		PrimaryColor: textNarg(in.PrimaryColor), Address: textNarg(in.Address),
		Phone: textNarg(in.Phone), Email: textNarg(in.Email), Website: textNarg(in.Website),
		FooterText: textNarg(in.FooterText), PaperSize: textNarg(in.PaperSize),
		MobileMinVersion:      textNarg(trimmed(in.MobileMinVersion)),
		MobileLatestVersion:   textNarg(trimmed(in.MobileLatestVersion)),
		MobileIosStoreUrl:     textNarg(trimmed(in.MobileIOSStoreURL)),
		MobileAndroidStoreUrl: textNarg(trimmed(in.MobileAndroidStoreURL)),
	})
	if err != nil {
		return Settings{}, err
	}
	return mapSettings(row), nil
}

// SetLogo stores logo object key.
func (s *Service) SetLogo(ctx context.Context, objectKey string) (Settings, error) {
	row, err := s.q.SetAppSettingsLogo(ctx, pgtype.Text{String: objectKey, Valid: objectKey != ""})
	if err != nil {
		return Settings{}, err
	}
	return mapSettings(row), nil
}

// ClearLogo removes logo.
func (s *Service) ClearLogo(ctx context.Context) (Settings, error) {
	row, err := s.q.ClearAppSettingsLogo(ctx)
	if err != nil {
		return Settings{}, err
	}
	return mapSettings(row), nil
}

// LogoObjectKey returns stored logo key if any.
func (s *Service) LogoObjectKey(ctx context.Context) (string, error) {
	row, err := s.q.GetAppSettings(ctx)
	if err != nil {
		return "", err
	}
	if !row.LogoObjectKey.Valid {
		return "", ErrNotFound
	}
	return row.LogoObjectKey.String, nil
}

func textNarg(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}
