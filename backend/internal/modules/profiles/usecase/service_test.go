package usecase

import (
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/storage"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/upload"
	"github.com/google/uuid"
)

type mockRepo struct {
	profiles map[uuid.UUID]*model.ProfileDetail
	accounts map[uuid.UUID]int64
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		profiles: make(map[uuid.UUID]*model.ProfileDetail),
		accounts: make(map[uuid.UUID]int64),
	}
}

func (m *mockRepo) find(accountID int64, profileUUID uuid.UUID) (*model.ProfileDetail, error) {
	p, ok := m.profiles[profileUUID]
	if !ok || m.accounts[profileUUID] != accountID {
		return nil, repository.ErrNotFound
	}
	return p, nil
}

func (m *mockRepo) ListByAccountID(ctx context.Context, accountID int64, limit, offset int32, q, status string) ([]model.ProfileSummary, int64, error) {
	var items []model.ProfileSummary
	for id, p := range m.profiles {
		if m.accounts[id] != accountID {
			continue
		}
		item := model.ProfileSummary{UUID: p.UUID, IsPrimary: p.IsPrimary, CompletionRate: p.CompletionRate}
		if len(p.Translations) > 0 {
			item.FirstName = p.Translations[0].FirstName
			item.LastName = p.Translations[0].LastName
		}
		items = append(items, item)
	}
	return items, int64(len(items)), nil
}

func (m *mockRepo) GetByUUID(ctx context.Context, accountID int64, profileUUID uuid.UUID) (*model.ProfileDetail, error) {
	return m.find(accountID, profileUUID)
}

func (m *mockRepo) GetPrimaryByAccountID(ctx context.Context, accountID int64) (*model.ProfileDetail, error) {
	for id, p := range m.profiles {
		if p.IsPrimary && m.accounts[id] == accountID {
			return p, nil
		}
	}
	return nil, repository.ErrNotFound
}

func translationsOf(in model.UpsertProfileInput) []model.ProfileTranslationDTO {
	out := make([]model.ProfileTranslationDTO, len(in.Translations))
	for i, tr := range in.Translations {
		out[i] = model.ProfileTranslationDTO{
			Locale: tr.Locale, FirstName: tr.FirstName, LastName: tr.LastName,
			Title: tr.Title, CompanyName: tr.CompanyName, IsDefault: tr.IsDefault,
		}
	}
	return out
}

func (m *mockRepo) Create(ctx context.Context, accountID, creatorUserID int64, userUUID uuid.UUID, in model.UpsertProfileInput, completionRate int) (*model.ProfileDetail, error) {
	newID := uuid.New()
	p := &model.ProfileDetail{
		UUID: newID, UserUUID: userUUID, PrimaryLocale: in.PrimaryLocale, IsPrimary: in.IsPrimary,
		CompletionRate: completionRate, Translations: translationsOf(in),
	}
	m.profiles[newID] = p
	m.accounts[newID] = accountID
	return p, nil
}

func (m *mockRepo) Update(ctx context.Context, accountID int64, profileUUID uuid.UUID, userUUID uuid.UUID, in model.UpsertProfileInput, completionRate int) (*model.ProfileDetail, error) {
	p, err := m.find(accountID, profileUUID)
	if err != nil {
		return nil, err
	}
	p.PrimaryLocale = in.PrimaryLocale
	p.IsPrimary = in.IsPrimary
	p.CompletionRate = completionRate
	p.Translations = translationsOf(in)
	return p, nil
}

func (m *mockRepo) UpdateAvatar(ctx context.Context, accountID int64, profileUUID uuid.UUID, avatarURL string) (*model.ProfileDetail, error) {
	p, err := m.find(accountID, profileUUID)
	if err != nil {
		return nil, err
	}
	p.AvatarURL = avatarURL
	return p, nil
}

func (m *mockRepo) UpdateCover(ctx context.Context, accountID int64, profileUUID uuid.UUID, coverURL string) (*model.ProfileDetail, error) {
	p, err := m.find(accountID, profileUUID)
	if err != nil {
		return nil, err
	}
	p.CoverURL = coverURL
	return p, nil
}

func (m *mockRepo) Delete(ctx context.Context, accountID int64, profileUUID uuid.UUID) error {
	if _, err := m.find(accountID, profileUUID); err != nil {
		return err
	}
	delete(m.profiles, profileUUID)
	return nil
}

func (m *mockRepo) SetPrimary(ctx context.Context, accountID int64, profileUUID uuid.UUID) error {
	for id, p := range m.profiles {
		if m.accounts[id] == accountID {
			p.IsPrimary = id == profileUUID
		}
	}
	return nil
}

func principalIn(accountID int64, role string) authctx.Principal {
	return authctx.Principal{
		UserInternal: 10,
		UserID:       uuid.New(),
		Roles:        []string{"subscriber"},
		Account:      authctx.Account{ID: accountID, UUID: uuid.New(), Type: authctx.AccountTypeCompany, Role: role},
	}
}

func sampleProfileInput() model.UpsertProfileInput {
	return model.UpsertProfileInput{
		PrimaryLocale: "tr",
		Translations:  []model.UpsertTranslationInput{{Locale: "tr", FirstName: "Ada", LastName: "Lovelace", IsDefault: true}},
	}
}

func TestProfilesAreIsolatedByAccount(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo, nil)
	svc.SetQuotas(&stubQuotas{})
	ctx := context.Background()

	acme := principalIn(1, authctx.AccountRoleOwner)
	other := principalIn(2, authctx.AccountRoleOwner)
	// A platform super admin acting in another account gets no bypass on app routes.
	other.IsSuperAdmin = true

	created, err := svc.Create(ctx, acme, sampleProfileInput())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err := svc.Get(ctx, other, created.UUID); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get from other account: expected ErrNotFound, got %v", err)
	}
	if _, err := svc.Update(ctx, other, created.UUID, sampleProfileInput()); !errors.Is(err, ErrNotFound) {
		t.Errorf("Update from other account: expected ErrNotFound, got %v", err)
	}
	if err := svc.Delete(ctx, other, created.UUID); !errors.Is(err, ErrNotFound) {
		t.Errorf("Delete from other account: expected ErrNotFound, got %v", err)
	}
	if _, _, err := svc.GenerateVCard(ctx, other, created.UUID, "tr", "4.0"); !errors.Is(err, ErrNotFound) {
		t.Errorf("vCard from other account: expected ErrNotFound, got %v", err)
	}
	if items, _, _ := svc.List(ctx, other, 20, 0, "", ""); len(items) != 0 {
		t.Errorf("List from other account returned %d items", len(items))
	}
}

func TestMemberCanReadButNotWrite(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo, nil)
	svc.SetQuotas(&stubQuotas{})
	ctx := context.Background()

	admin := principalIn(1, authctx.AccountRoleAdmin)
	member := principalIn(1, authctx.AccountRoleMember)

	created, err := svc.Create(ctx, admin, sampleProfileInput())
	if err != nil {
		t.Fatalf("admin create: %v", err)
	}
	if _, err := svc.Get(ctx, member, created.UUID); err != nil {
		t.Errorf("member read: %v", err)
	}
	if _, err := svc.Create(ctx, member, sampleProfileInput()); !errors.Is(err, ErrForbidden) {
		t.Errorf("member create: expected ErrForbidden, got %v", err)
	}
	if _, err := svc.Update(ctx, member, created.UUID, sampleProfileInput()); !errors.Is(err, ErrForbidden) {
		t.Errorf("member update: expected ErrForbidden, got %v", err)
	}
	if err := svc.SetPrimary(ctx, member, created.UUID); !errors.Is(err, ErrForbidden) {
		t.Errorf("member set primary: expected ErrForbidden, got %v", err)
	}
	if err := svc.Delete(ctx, member, created.UUID); !errors.Is(err, ErrForbidden) {
		t.Errorf("member delete: expected ErrForbidden, got %v", err)
	}
}

func TestService_CreateAndGet(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo, nil)
	svc.SetQuotas(&stubQuotas{})
	principal := principalIn(1, authctx.AccountRoleOwner)

	created, err := svc.Create(context.Background(), principal, model.UpsertProfileInput{
		PrimaryLocale: "tr",
		Translations: []model.UpsertTranslationInput{
			{
				Locale:      "tr",
				FirstName:   "Furkan",
				LastName:    "Meclis",
				Title:       "Software Architect",
				CompanyName: "Comcode",
				IsDefault:   true,
			},
		},
		ContactDetails: []model.CreateContactDetailInput{
			{Category: "phone", Value: "+905551234567"},
			{Category: "email", Value: "furkan@example.com"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error creating profile: %v", err)
	}

	if created.CompletionRate <= 0 {
		t.Errorf("expected completion rate > 0, got %d", created.CompletionRate)
	}

	// Fetch detail
	fetched, err := svc.Get(context.Background(), principal, created.UUID)
	if err != nil {
		t.Fatalf("unexpected error fetching profile: %v", err)
	}
	if len(fetched.Translations) == 0 || fetched.Translations[0].FirstName != "Furkan" {
		t.Errorf("expected translation FirstName 'Furkan'")
	}

	// Generate vCard
	vcf30, filename, err := svc.GenerateVCard(context.Background(), principal, created.UUID, "tr", "3.0")
	if err != nil {
		t.Fatalf("unexpected error generating vcard 3.0: %v", err)
	}
	if filename != "Furkan_Meclis.vcf" {
		t.Errorf("expected filename Furkan_Meclis.vcf, got %s", filename)
	}
	if !strings.Contains(vcf30, "FN:Furkan Meclis") {
		t.Errorf("expected vcf to contain FN:Furkan Meclis, got:\n%s", vcf30)
	}
}

// memoryDriver adapts the in-memory store to storage.Driver; methods the profile service does not
// use fall through to the nil embedded interface and would panic if called.
type memoryDriver struct {
	storage.Driver
	mem *storage.Memory
}

func (d memoryDriver) Upload(ctx context.Context, f storage.File, path string) error {
	return d.mem.Upload(ctx, f, path)
}

func (d memoryDriver) Download(ctx context.Context, path string) (io.ReadCloser, int64, error) {
	return d.mem.Download(ctx, path)
}

func (d memoryDriver) Delete(ctx context.Context, path string) error { return d.mem.Delete(ctx, path) }

func (d memoryDriver) PublicURL(path string) string { return d.mem.PublicURL(path) }

func TestProfileMediaURLsMustBeAPIOwned(t *testing.T) {
	store := memoryDriver{mem: storage.NewMemory()}
	repo := newMockRepo()
	svc := NewService(repo, store)
	svc.SetQuotas(&stubQuotas{})
	ctx := context.Background()
	owner := principalIn(1, authctx.AccountRoleOwner)

	ssrf := []string{
		"http://169.254.169.254/latest/meta-data/iam/security-credentials/",
		"http://meilisearch:7700/keys",
		"https://example.com/profiles/x/avatar.jpg",
		"profiles/../exports/job/export.csv",
	}
	for _, raw := range ssrf {
		in := sampleProfileInput()
		in.AvatarURL = raw
		if _, err := svc.Create(ctx, owner, in); !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("create with avatar %q: expected ErrInvalidRequest, got %v", raw, err)
		}
	}

	created, err := svc.Create(ctx, owner, sampleProfileInput())
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	other, err := svc.Create(ctx, owner, sampleProfileInput())
	if err != nil {
		t.Fatalf("create other: %v", err)
	}

	// Own stored media is accepted; another profile's media is not.
	own := sampleProfileInput()
	own.AvatarURL = store.PublicURL(storage.ProfileAvatarObjectKey(created.UUID.String(), "jpg"))
	if _, err := svc.Update(ctx, owner, created.UUID, own); err != nil {
		t.Errorf("update with own avatar: %v", err)
	}
	foreign := sampleProfileInput()
	foreign.AvatarURL = store.PublicURL(storage.ProfileAvatarObjectKey(other.UUID.String(), "jpg"))
	if _, err := svc.Update(ctx, owner, created.UUID, foreign); !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("update with another profile's avatar: expected ErrInvalidRequest, got %v", err)
	}

	// vCard generation never fetches remote URLs, even if one is already stored.
	if data, _ := svc.fetchPhoto(ctx, created.UUID, "http://169.254.169.254/latest/meta-data/"); data != nil {
		t.Error("fetchPhoto followed a remote URL")
	}
}

func TestCreateIsBlockedByProfileQuota(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo, nil)
	quotas := &stubQuotas{deny: map[string]bool{entitlement.FeatureProfilesMax: true}}
	svc.SetQuotas(quotas)
	ctx := context.Background()
	owner := principalIn(1, authctx.AccountRoleOwner)

	if _, err := svc.Create(ctx, owner, sampleProfileInput()); !errors.Is(err, entitlement.ErrPlanLimitExceeded) {
		t.Fatalf("expected ErrPlanLimitExceeded, got %v", err)
	}
	if len(quotas.checked) != 1 || quotas.checked[0] != entitlement.FeatureProfilesMax {
		t.Fatalf("checked %v", quotas.checked)
	}
	if _, total, _ := svc.List(ctx, owner, 20, 0, "", ""); total != 0 {
		t.Fatalf("denied create persisted %d profiles", total)
	}
}

func TestCreateWithoutQuotaCheckerIsDenied(t *testing.T) {
	svc := NewService(newMockRepo(), nil)
	if _, err := svc.Create(context.Background(), principalIn(1, authctx.AccountRoleOwner), sampleProfileInput()); err == nil {
		t.Fatal("create succeeded without a quota checker")
	}
}

// TEC-1097: the avatar type and extension come from the bytes, never from the
// client filename or Content-Type, so avatar.html cannot be stored as HTML.
func TestUploadAvatarSniffsContent(t *testing.T) {
	mem := storage.NewMemory()
	store := memoryDriver{mem: mem}
	repo := newMockRepo()
	svc := NewService(repo, store)
	svc.SetQuotas(&stubQuotas{})
	ctx := context.Background()
	owner := principalIn(1, authctx.AccountRoleOwner)
	created, err := svc.Create(ctx, owner, sampleProfileInput())
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	html := "<!doctype html><script>alert(document.cookie)</script>"
	if _, err := svc.UploadAvatar(ctx, owner, created.UUID, strings.NewReader(html), "avatar.html", "text/html", int64(len(html))); !errors.Is(err, upload.ErrUnsupportedType) {
		t.Fatalf("html avatar: %v", err)
	}
	if _, err := svc.UploadCover(ctx, owner, created.UUID, strings.NewReader(html), "cover.png", "image/png", int64(len(html))); !errors.Is(err, upload.ErrUnsupportedType) {
		t.Fatalf("html disguised as png cover: %v", err)
	}
	svg := `<svg xmlns="http://www.w3.org/2000/svg"/>`
	if _, err := svc.UploadAvatar(ctx, owner, created.UUID, strings.NewReader(svg), "a.svg", "image/svg+xml", int64(len(svg))); !errors.Is(err, upload.ErrUnsupportedType) {
		t.Fatalf("svg avatar must stay raster-only: %v", err)
	}

	png := "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"
	if _, err := svc.UploadAvatar(ctx, owner, created.UUID, strings.NewReader(png), "avatar.html", "text/html", int64(len(png))); err != nil {
		t.Fatalf("png avatar: %v", err)
	}
	key := storage.ProfileAvatarObjectKey(created.UUID.String(), "png")
	if ct, _, ok := mem.GetMeta(key); !ok || ct != "image/png" {
		t.Fatalf("avatar stored as %q (found %v)", ct, ok)
	}

	// A company-library SVG logo cannot become an avatar.
	in := sampleProfileInput()
	in.AvatarURL = store.PublicURL(storage.CompanyMediaObjectKey(owner.Account.UUID.String(), uuid.NewString(), "svg"))
	if _, err := svc.Update(ctx, owner, created.UUID, in); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("svg avatar url: %v", err)
	}
}
