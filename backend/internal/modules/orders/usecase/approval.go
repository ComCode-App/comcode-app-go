package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	notifModel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/orders/repository"
	profilesusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
	"github.com/google/uuid"
)

// StatusAwaitingApproval is a company bulk order waiting for a super admin before payment
// (TEC-1317). It cannot be paid; approval moves it to pending, rejection to cancelled.
const StatusAwaitingApproval = "awaiting_approval"

var (
	// ErrNotAwaitingApproval: approve/reject on an order in another status.
	ErrNotAwaitingApproval = errors.New("order is not awaiting approval")
	// ErrAwaitingApproval: a status or payment change on an order that waits for approval; use
	// approve/reject instead.
	ErrAwaitingApproval = errors.New("order is awaiting approval; approve or reject it first")
	// ErrRejectionReasonRequired: reject without a reason.
	ErrRejectionReasonRequired = errors.New("rejection reason is required")
)

// QuotaExceededError blocks an approval while the company holds more profiles than its plan
// allows; Missing is how many profiles are over the limit.
type QuotaExceededError struct {
	Used    int64
	Limit   int32
	Missing int64
}

func (e *QuotaExceededError) Error() string {
	return fmt.Sprintf("profile quota exceeded: %d profiles over the plan limit of %d", e.Missing, e.Limit)
}

// DraftProfiles creates and removes the recipients' profiles of a company bulk order.
type DraftProfiles interface {
	CreateOrderDraft(ctx context.Context, accountID, creatorUserID int64, creatorUUID uuid.UUID, in profilesusecase.OrderDraftInput) (uuid.UUID, error)
	DeleteOrderDraft(ctx context.Context, accountID int64, profileUUID uuid.UUID) error
	InvalidateProfiles(ctx context.Context, ids []uuid.UUID)
}

// ProfileQuota reads the profile quota of an account at approval time.
type ProfileQuota interface {
	QuotaUsageFor(ctx context.Context, accountID int64, key string) (entitlement.QuotaUsage, error)
}

// SetBulkApproval wires the company bulk order approval flow (TEC-1317). Without it, company
// bulk orders keep the plain pending flow.
func (s *Service) SetBulkApproval(drafts DraftProfiles, quota ProfileQuota) {
	s.drafts = drafts
	s.quota = quota
}

// needsApproval reports whether an app order waits for platform approval: bulk orders of a
// company account. Single orders and personal accounts keep the plain flow.
func (s *Service) needsApproval(account authctx.Account, input model.CreateOrderInput) bool {
	return s.drafts != nil && account.Type == authctx.AccountTypeCompany && input.OrderType == "bulk"
}

// CreateForPrincipal places an app order for the caller's active account. A company bulk order
// opens as awaiting_approval with a profile per recipient: an existing profile of this company
// with the recipient's e-mail is linked, otherwise a draft profile with a closed public page is
// created. Plan quota is checked at approval.
func (s *Service) CreateForPrincipal(ctx context.Context, p authctx.Principal, input model.CreateOrderInput) (*model.OrderDetail, error) {
	if !s.needsApproval(p.Account, input) {
		return s.CreateForAccount(ctx, p.Account, p.UserInternal, input)
	}
	if !p.Account.CanWrite() {
		return nil, ErrForbidden
	}
	if len(input.Items) == 0 {
		return nil, ErrEmptyItems
	}
	created := map[string]bool{}
	cleanup := func() {
		for id := range created {
			_ = s.drafts.DeleteOrderDraft(context.WithoutCancel(ctx), p.Account.ID, uuid.MustParse(id))
		}
	}
	for i := range input.Items {
		it := &input.Items[i]
		if it.ProfileUUID != nil && strings.TrimSpace(*it.ProfileUUID) != "" {
			continue
		}
		if email := strings.TrimSpace(it.RecipientEmail); email != "" {
			_, existing, found, err := s.repo.FindAccountProfileByEmail(ctx, p.Account.ID, email)
			if err != nil {
				cleanup()
				return nil, err
			}
			if found {
				v := existing.String()
				it.ProfileUUID = &v
				continue
			}
		}
		if strings.TrimSpace(it.RecipientName) == "" {
			continue
		}
		id, err := s.drafts.CreateOrderDraft(ctx, p.Account.ID, p.UserInternal, p.UserID, profilesusecase.OrderDraftInput{
			Name: it.RecipientName, Title: it.RecipientTitle, Department: it.RecipientDepartment,
			Email: it.RecipientEmail, Phone: it.RecipientPhone,
		})
		if err != nil {
			cleanup()
			return nil, fmt.Errorf("create recipient profile: %w", err)
		}
		v := id.String()
		it.ProfileUUID = &v
		created[v] = true
	}
	input.InitialStatus = StatusAwaitingApproval
	ord, err := s.create(ctx, input, p.Account.ID, p.UserInternal)
	if err != nil {
		cleanup()
		return nil, err
	}
	for _, item := range ord.Items {
		if item.ProfileUUID != nil && created[*item.ProfileUUID] {
			if err := s.repo.MarkItemProfileCreated(ctx, ord.ID, uuid.MustParse(item.UUID)); err != nil {
				return nil, err
			}
		}
	}
	return s.indexed(s.repo.GetByUUID(ctx, uuid.MustParse(ord.UUID)))
}

// ApproveOrder moves a company bulk order from awaiting_approval to pending so the customer
// can pay. It is blocked (QuotaExceededError) while the company holds more profiles, the
// order's drafts included, than its plan allows.
func (s *Service) ApproveOrder(ctx context.Context, id uuid.UUID) (*model.OrderDetail, error) {
	ord, err := s.repo.GetByUUID(ctx, id)
	if err != nil {
		return nil, err
	}
	if ord == nil {
		return nil, ErrNotFound
	}
	if ord.Status != StatusAwaitingApproval {
		return nil, ErrNotAwaitingApproval
	}
	if s.quota != nil {
		u, err := s.quota.QuotaUsageFor(ctx, ord.AccountID, entitlement.FeatureProfilesMax)
		if err != nil {
			return nil, err
		}
		if missing := u.Missing(); missing > 0 {
			return nil, &QuotaExceededError{Used: u.Used, Limit: u.Limit, Missing: missing}
		}
	}
	out, err := s.repo.ReviewOrder(ctx, id, StatusAwaitingApproval, "pending", "", actorUserID(ctx))
	if err != nil {
		if errors.Is(err, repository.ErrStatusMismatch) {
			return nil, ErrNotAwaitingApproval
		}
		return nil, err
	}
	if out == nil {
		return nil, ErrNotFound
	}
	s.notifyOrderTransitionsWith(ctx, out, map[string]notificationOverride{
		"pending": func(lang string, in notifModel.EnqueueInput) notifModel.EnqueueInput {
			in.TemplateCode = ""
			if lang == "en" {
				in.Title = out.OrderNumber + ": order approved"
				in.Body = "Your order " + out.OrderNumber + " was approved. You can pay for it now: " + derefURL(in.ActionURL)
			} else {
				in.Title = out.OrderNumber + ": sipariş onaylandı"
				in.Body = out.OrderNumber + " numaralı siparişiniz onaylandı. Şimdi ödeme yapabilirsiniz: " + derefURL(in.ActionURL)
			}
			return in
		},
	})
	return s.indexed(out, nil)
}

// RejectOrder cancels a company bulk order awaiting approval with a required reason and deletes
// the draft profiles the order created. Existing profiles linked by e-mail are kept.
func (s *Service) RejectOrder(ctx context.Context, id uuid.UUID, reason string) (*model.OrderDetail, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, ErrRejectionReasonRequired
	}
	if len(reason) > 2000 {
		reason = reason[:2000]
	}
	out, err := s.repo.ReviewOrder(ctx, id, StatusAwaitingApproval, "cancelled", reason, actorUserID(ctx))
	if err != nil {
		if errors.Is(err, repository.ErrStatusMismatch) {
			return nil, ErrNotAwaitingApproval
		}
		return nil, err
	}
	if out == nil {
		return nil, ErrNotFound
	}
	if s.drafts != nil {
		ids, err := s.repo.ListOrderCreatedProfiles(ctx, out.AccountID, out.ID)
		if err != nil {
			return nil, err
		}
		for _, pid := range ids {
			if err := s.drafts.DeleteOrderDraft(ctx, out.AccountID, pid); err != nil && !errors.Is(err, profilesusecase.ErrNotFound) {
				return nil, fmt.Errorf("delete draft profile: %w", err)
			}
		}
		if out, err = s.repo.GetByUUID(ctx, id); err != nil {
			return nil, err
		}
	}
	s.notifyOrderTransitionsWith(ctx, out, map[string]notificationOverride{
		"cancelled": func(lang string, in notifModel.EnqueueInput) notifModel.EnqueueInput {
			in.TemplateCode = ""
			if lang == "en" {
				in.Title = out.OrderNumber + ": order rejected"
				in.Body = "Your order " + out.OrderNumber + " was rejected. Reason: " + reason
			} else {
				in.Title = out.OrderNumber + ": sipariş reddedildi"
				in.Body = out.OrderNumber + " numaralı siparişiniz reddedildi. Neden: " + reason
			}
			if in.Payload != nil {
				in.Payload["rejection_reason"] = reason
			}
			return in
		},
	})
	return s.indexed(out, nil)
}

// openOrderDrafts opens the public pages of the draft profiles a paid order created.
func (s *Service) openOrderDrafts(ctx context.Context, ord *model.OrderDetail) {
	if s.drafts == nil || ord == nil || (ord.Status != "paid" && ord.PaymentStatus != "paid") {
		return
	}
	ids, err := s.repo.OpenOrderDraftProfiles(ctx, ord.AccountID, ord.ID)
	if err == nil && len(ids) > 0 {
		s.drafts.InvalidateProfiles(ctx, ids)
	}
}

func derefURL(u *string) string {
	if u == nil {
		return ""
	}
	return *u
}

// guardAwaitingApproval refuses generic status and payment changes on an order that waits for
// approval, so it cannot be paid or moved on without the approve/reject flow.
// Orders only wait for approval when the flow is wired (SetBulkApproval).
func (s *Service) guardAwaitingApproval(ctx context.Context, id uuid.UUID) error {
	if s.drafts == nil {
		return nil
	}
	ord, err := s.repo.GetByUUID(ctx, id)
	if err != nil {
		return err
	}
	if ord != nil && ord.Status == StatusAwaitingApproval {
		return ErrAwaitingApproval
	}
	return nil
}
