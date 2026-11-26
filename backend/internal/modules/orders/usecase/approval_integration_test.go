package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/model"
	ordersRepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/repository"
	productsRepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/products/repository"
	pmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/model"
	profilesrepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/repository"
	profilesusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
)

type allowAll struct{}

func (allowAll) CheckQuota(context.Context, int64, bool, string) error { return nil }

// fakeQuota reports a fixed profile limit against the real usage count.
type fakeQuota struct {
	d     *pgtest.DB
	limit int32
}

func (f *fakeQuota) QuotaUsageFor(ctx context.Context, accountID int64, _ string) (entitlement.QuotaUsage, error) {
	used, err := f.d.Queries.CountAccountProfilesUsage(ctx, accountID)
	return entitlement.QuotaUsage{Used: used, Limit: f.limit}, err
}

type approvalFixture struct {
	d        *pgtest.DB
	svc      *Service
	quota    *fakeQuota
	p        authctx.Principal
	product  uuid.UUID
	existing uuid.UUID
}

func newApprovalFixture(t *testing.T) approvalFixture {
	t.Helper()
	d := pgtest.Open(t)
	ctx := context.Background()
	owner := d.CreateUser(t)
	acc, err := d.Queries.CreateCompanyAccount(ctx, db.CreateCompanyAccountParams{Name: "Co", Slug: "co-" + pgtest.Unique()})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Queries.AddAccountMember(ctx, db.AddAccountMemberParams{AccountID: acc.ID, UserID: owner.ID, Role: db.AccountRoleOwner}); err != nil {
		t.Fatal(err)
	}
	p := authctx.Principal{
		UserID: owner.UUID, UserInternal: owner.ID,
		Account: authctx.Account{ID: acc.ID, UUID: acc.Uuid, Type: authctx.AccountTypeCompany, Role: authctx.AccountRoleOwner},
	}
	prepo := profilesrepo.NewPostgres(d.Pool, d.Queries)
	profiles := profilesusecase.NewService(prepo, nil)
	profiles.SetQuotas(allowAll{})
	profiles.SetPublicPages(prepo)
	existing, err := profiles.Create(ctx, p, pmodel.UpsertProfileInput{
		PrimaryLocale:  "tr",
		Translations:   []pmodel.UpsertTranslationInput{{Locale: "tr", FirstName: "Existing", LastName: "One", IsDefault: true}},
		ContactDetails: []pmodel.CreateContactDetailInput{{Category: "email", Type: "work", Value: "Match@Example.test", IsPrimary: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	repo := ordersRepo.NewPostgres(d.Pool, d.Queries)
	svc := NewService(repo, productsRepo.NewPostgres(d.Queries), nil, nil, "https://comcode.test")
	quota := &fakeQuota{d: d, limit: 100}
	svc.SetBulkApproval(profiles, quota)
	return approvalFixture{d: d, svc: svc, quota: quota, p: p, product: d.CreateProduct(t, 100), existing: existing.UUID}
}

func (f approvalFixture) bulkOrder(t *testing.T) *model.OrderDetail {
	t.Helper()
	ord, err := f.svc.CreateForPrincipal(context.Background(), f.p, model.CreateOrderInput{
		OrderType: "bulk",
		Items: []model.CreateOrderItemInput{
			{ProductUUID: f.product.String(), Quantity: 1, RecipientName: "Matched Person", RecipientEmail: "match@example.test"},
			{ProductUUID: f.product.String(), Quantity: 1, RecipientName: "Ada Lovelace", RecipientTitle: "CTO", RecipientEmail: "ada@example.test"},
			{ProductUUID: f.product.String(), Quantity: 1, RecipientName: "Grace Hopper", RecipientPhone: "+905000000000"},
		},
	})
	if err != nil {
		t.Fatalf("create bulk order: %v", err)
	}
	return ord
}

func (f approvalFixture) routeStatus(t *testing.T, profile string) string {
	t.Helper()
	var s string
	err := f.d.Pool.QueryRow(context.Background(), `
		SELECT r.status FROM routes r JOIN profiles p ON p.id = r.profile_id
		WHERE p.uuid = $1 AND r.route_type = 'canonical_profile' AND r.deleted_at IS NULL`, profile).Scan(&s)
	if err != nil {
		t.Fatalf("route of %s: %v", profile, err)
	}
	return s
}

func (f approvalFixture) profileAlive(t *testing.T, profile string) bool {
	t.Helper()
	var n int
	if err := f.d.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM profiles WHERE uuid = $1 AND deleted_at IS NULL`, profile).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n == 1
}

func TestIntegrationBulkOrderApproval(t *testing.T) {
	f := newApprovalFixture(t)
	ctx := context.Background()
	ord := f.bulkOrder(t)
	id := uuid.MustParse(ord.UUID)

	if ord.Status != StatusAwaitingApproval {
		t.Fatalf("status = %q, want awaiting_approval", ord.Status)
	}
	var drafts []string
	for _, it := range ord.Items {
		if it.ProfileUUID == nil {
			t.Fatalf("item %s has no profile", it.RecipientName)
		}
		if it.RecipientEmail == "match@example.test" {
			if *it.ProfileUUID != f.existing.String() || it.ProfileCreatedByOrder {
				t.Fatalf("matched item = %s created=%v, want existing profile", *it.ProfileUUID, it.ProfileCreatedByOrder)
			}
			continue
		}
		if !it.ProfileCreatedByOrder {
			t.Fatalf("item %s profile not flagged as draft", it.RecipientName)
		}
		if got := f.routeStatus(t, *it.ProfileUUID); got != "locked" {
			t.Fatalf("draft page status = %q, want locked", got)
		}
		drafts = append(drafts, *it.ProfileUUID)
	}
	if len(drafts) != 2 {
		t.Fatalf("drafts = %d, want 2", len(drafts))
	}

	// Not payable, not movable by the generic status routes.
	if IsPayable(ord.Status, ord.PaymentStatus) {
		t.Fatal("awaiting order is payable")
	}
	if _, err := f.svc.UpdateStatus(ctx, id, model.UpdateOrderStatusInput{Status: "paid"}); !errors.Is(err, ErrAwaitingApproval) {
		t.Fatalf("status change err = %v", err)
	}
	if _, err := f.svc.UpdatePaymentStatus(ctx, id, model.UpdateOrderPaymentStatusInput{PaymentStatus: "paid"}); !errors.Is(err, ErrAwaitingApproval) {
		t.Fatalf("payment change err = %v", err)
	}

	// Over quota: 3 profiles against a limit of 1 -> 2 missing.
	f.quota.limit = 1
	var qe *QuotaExceededError
	if _, err := f.svc.ApproveOrder(ctx, id); !errors.As(err, &qe) || qe.Missing != 2 {
		t.Fatalf("approve over quota err = %v", err)
	}
	f.quota.limit = 3
	approved, err := f.svc.ApproveOrder(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if approved.Status != "pending" || !IsPayable(approved.Status, approved.PaymentStatus) {
		t.Fatalf("approved status = %q/%q", approved.Status, approved.PaymentStatus)
	}
	if _, err := f.svc.ApproveOrder(ctx, id); !errors.Is(err, ErrNotAwaitingApproval) {
		t.Fatalf("second approve err = %v", err)
	}
	if _, err := f.svc.RejectOrder(ctx, id, "late"); !errors.Is(err, ErrNotAwaitingApproval) {
		t.Fatalf("reject after approve err = %v", err)
	}

	// Paid: cards are provisioned onto the recipients' profiles and the drafts go live.
	if _, err := f.svc.repo.UpdateStatus(ctx, id, "paid", nil); err != nil {
		t.Fatal(err)
	}
	paid, err := f.svc.ProvisionOrder(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range paid.Items {
		if it.CardUUID == nil {
			t.Fatalf("item %s not provisioned", it.RecipientName)
		}
	}
	for _, p := range drafts {
		if got := f.routeStatus(t, p); got != "active" {
			t.Fatalf("draft page after payment = %q, want active", got)
		}
	}
}

func TestIntegrationBulkOrderRejection(t *testing.T) {
	f := newApprovalFixture(t)
	ctx := context.Background()
	ord := f.bulkOrder(t)
	id := uuid.MustParse(ord.UUID)

	if _, err := f.svc.RejectOrder(ctx, id, "  "); !errors.Is(err, ErrRejectionReasonRequired) {
		t.Fatalf("reject without reason err = %v", err)
	}
	rejected, err := f.svc.RejectOrder(ctx, id, "Eksik bilgi")
	if err != nil {
		t.Fatal(err)
	}
	if rejected.Status != "cancelled" || rejected.RejectionReason != "Eksik bilgi" {
		t.Fatalf("rejected = %q, reason %q", rejected.Status, rejected.RejectionReason)
	}
	for _, it := range ord.Items {
		alive := f.profileAlive(t, *it.ProfileUUID)
		if it.ProfileCreatedByOrder && alive {
			t.Fatalf("draft profile of %s survived the rejection", it.RecipientName)
		}
		if !it.ProfileCreatedByOrder && !alive {
			t.Fatalf("existing profile of %s was deleted", it.RecipientName)
		}
	}
	if _, err := f.svc.ApproveOrder(ctx, id); !errors.Is(err, ErrNotAwaitingApproval) {
		t.Fatalf("approve after reject err = %v", err)
	}
}

func TestIntegrationSingleAndPersonalOrdersSkipApproval(t *testing.T) {
	f := newApprovalFixture(t)
	ctx := context.Background()
	// A company single order keeps the plain flow.
	single, err := f.svc.CreateForPrincipal(ctx, f.p, model.CreateOrderInput{
		Items: []model.CreateOrderItemInput{{ProductUUID: f.product.String(), Quantity: 1, RecipientName: "Solo"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if single.Status != "pending" || single.Items[0].ProfileUUID != nil {
		t.Fatalf("single order = %q, profile %v", single.Status, single.Items[0].ProfileUUID)
	}
	// A personal account bulk order too.
	u := f.d.CreateUser(t)
	personal := authctx.Principal{UserID: u.UUID, UserInternal: u.ID,
		Account: authctx.Account{ID: u.AccountID, Type: authctx.AccountTypePersonal, Role: authctx.AccountRoleOwner}}
	bulk, err := f.svc.CreateForPrincipal(ctx, personal, model.CreateOrderInput{
		OrderType: "bulk",
		Items:     []model.CreateOrderItemInput{{ProductUUID: f.product.String(), Quantity: 1, RecipientName: "A"}, {ProductUUID: f.product.String(), Quantity: 1, RecipientName: "B"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if bulk.Status != "pending" {
		t.Fatalf("personal bulk status = %q", bulk.Status)
	}
}
