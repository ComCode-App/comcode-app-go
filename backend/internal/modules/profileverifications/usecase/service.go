// Package usecase implements the verified profile badge (TEC-549): owners request verification
// of a profile, platform admins approve, reject or revoke it, and a verified profile that changes
// its display name or organization loses the badge and goes back to review.
package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	notifmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profileverifications/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
)

var (
	ErrNotFound       = errors.New("not found")
	ErrForbidden      = errors.New("forbidden")
	ErrInvalidRequest = errors.New("invalid request")
	// ErrConflict means the request is not in a state that allows the action.
	ErrConflict = errors.New("conflict")
)

// Notifier enqueues notifications (notifications.Service).
type Notifier interface {
	Enqueue(ctx context.Context, in notifmodel.EnqueueInput) ([]notifmodel.Notification, error)
}

// ProfileCache drops cached visitor pages of a profile (routecache.Cache).
type ProfileCache interface {
	InvalidateProfile(ctx context.Context, profileUUID uuid.UUID)
}

// Service is the verification use case. Owner methods are scoped to the principal's active
// account; platform methods (List, Get, Approve, Reject, Revoke) are unscoped and rely on the
// platform.profile_verifications.manage route gate.
type Service struct {
	pool        *pgxpool.Pool
	q           *db.Queries
	notifier    Notifier
	cache       ProfileCache
	frontendURL string
	log         *slog.Logger
}

// New creates the service. notifier may be nil (no notifications).
func New(pool *pgxpool.Pool, q *db.Queries, notifier Notifier, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{pool: pool, q: q, notifier: notifier, log: log}
}

// SetProfileCache invalidates visitor caches when the badge changes.
func (s *Service) SetProfileCache(c ProfileCache) { s.cache = c }

// SetFrontendURL is the absolute app origin used for links in notification emails.
func (s *Service) SetFrontendURL(u string) { s.frontendURL = strings.TrimRight(u, "/") }

// profileID resolves a live profile of the account; other accounts' profiles are not found.
func (s *Service) profileID(ctx context.Context, accountID int64, profileUUID uuid.UUID) (int64, error) {
	row, err := s.q.GetProfileIDForAccount(ctx, db.GetProfileIDForAccountParams{Uuid: profileUUID, AccountID: accountID})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("profile ref: %w", err)
	}
	return row.ID, nil
}

func (s *Service) latest(ctx context.Context, profileID int64) (*db.ProfileVerificationRequest, error) {
	row, err := s.q.GetLatestProfileVerification(ctx, profileID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("latest verification: %w", err)
	}
	return &row, nil
}

// Status is the profile's verification status (none | pending | verified | rejected).
func (s *Service) Status(ctx context.Context, accountID int64, profileUUID uuid.UUID) (string, error) {
	id, err := s.profileID(ctx, accountID, profileUUID)
	if err != nil {
		return "", err
	}
	row, err := s.latest(ctx, id)
	if err != nil || row == nil {
		return model.StatusNone, err
	}
	return model.StatusOf(row.Status), nil
}

// Verified reports whether a profile currently holds the badge (visitor API).
func (s *Service) Verified(ctx context.Context, profileID int64) (bool, error) {
	return s.q.IsProfileVerified(ctx, profileID)
}

// OwnerGet returns the verification view of a profile in the active account.
func (s *Service) OwnerGet(ctx context.Context, p authctx.Principal, profileUUID uuid.UUID) (*model.OwnerView, error) {
	id, err := s.profileID(ctx, p.Account.ID, profileUUID)
	if err != nil {
		return nil, err
	}
	return s.ownerView(ctx, id)
}

func (s *Service) ownerView(ctx context.Context, profileID int64) (*model.OwnerView, error) {
	row, err := s.latest(ctx, profileID)
	if err != nil {
		return nil, err
	}
	view := &model.OwnerView{Status: model.StatusNone, CanRequest: true}
	if row == nil {
		return view, nil
	}
	view.Status = model.StatusOf(row.Status)
	view.CanRequest = view.Status == model.StatusNone || view.Status == model.StatusRejected
	view.Request = &model.OwnerRequest{
		UUID: row.Uuid, Status: row.Status, Origin: row.Origin, Note: row.Note,
		RequestedAt: row.CreatedAt.Time, ReviewedAt: timePtr(row.ReviewedAt), ReviewReason: row.ReviewReason,
		RevokedAt: timePtr(row.RevokedAt), RevokeReason: row.RevokeReason,
	}
	return view, nil
}

// Request asks the platform to verify a profile. Needs an owner/admin account role; a profile
// that is pending or verified already answers ErrConflict.
func (s *Service) Request(ctx context.Context, p authctx.Principal, profileUUID uuid.UUID, in model.RequestInput) (*model.OwnerView, error) {
	id, err := s.profileID(ctx, p.Account.ID, profileUUID)
	if err != nil {
		return nil, err
	}
	if !p.Account.CanWrite() {
		return nil, ErrForbidden
	}
	note := strings.TrimSpace(in.Note)
	if utf8.RuneCountInString(note) > model.MaxNoteLength {
		return nil, fmt.Errorf("%w: note is longer than %d characters", ErrInvalidRequest, model.MaxNoteLength)
	}
	row, err := s.latest(ctx, id)
	if err != nil {
		return nil, err
	}
	if row != nil {
		switch row.Status {
		case model.RequestPending:
			return nil, fmt.Errorf("%w: a verification request is already pending", ErrConflict)
		case model.RequestApproved:
			return nil, fmt.Errorf("%w: the profile is already verified", ErrConflict)
		}
	}
	if _, err := s.q.CreateProfileVerificationRequest(ctx, db.CreateProfileVerificationRequestParams{
		AccountID: p.Account.ID, ProfileID: id, Note: note,
		RequestedByUserID: optUserID(p.UserInternal), Origin: model.OriginOwner,
	}); err != nil {
		if isUniqueViolation(err) { // a concurrent request won the race
			return nil, fmt.Errorf("%w: a verification request is already open", ErrConflict)
		}
		return nil, fmt.Errorf("create verification request: %w", err)
	}
	return s.ownerView(ctx, id)
}

// Cancel withdraws the profile's pending request.
func (s *Service) Cancel(ctx context.Context, p authctx.Principal, profileUUID uuid.UUID) (*model.OwnerView, error) {
	id, err := s.profileID(ctx, p.Account.ID, profileUUID)
	if err != nil {
		return nil, err
	}
	if !p.Account.CanWrite() {
		return nil, ErrForbidden
	}
	if _, err := s.q.CancelPendingProfileVerification(ctx, db.CancelPendingProfileVerificationParams{ProfileID: id, AccountID: p.Account.ID}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("%w: no pending verification request", ErrConflict)
		}
		return nil, fmt.Errorf("cancel verification request: %w", err)
	}
	return s.ownerView(ctx, id)
}

// RevokeForIdentityChange is called by the profiles service before it stores a change of a
// verified profile's display name or organization: the badge is revoked (reasons recorded) and a
// pending re-review request is opened in the same transaction. It is a no-op when the profile is
// not verified.
func (s *Service) RevokeForIdentityChange(ctx context.Context, accountID int64, profileUUID uuid.UUID, actorUserID int64, reasons []string) error {
	id, err := s.profileID(ctx, accountID, profileUUID)
	if err != nil {
		return err
	}
	reason := strings.Join(reasons, ",")
	revoked := false
	err = s.withTx(ctx, func(q *db.Queries) error {
		if _, err := q.RevokeProfileVerificationForProfile(ctx, db.RevokeProfileVerificationForProfileParams{
			RevokedByUserID: optUserID(actorUserID), RevokeReason: reason, ProfileID: id,
		}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil // not verified: nothing to revoke
			}
			return fmt.Errorf("revoke verification: %w", err)
		}
		revoked = true
		if _, err := q.CreateProfileVerificationRequest(ctx, db.CreateProfileVerificationRequestParams{
			AccountID: accountID, ProfileID: id, RequestedByUserID: optUserID(actorUserID), Origin: model.OriginReReview,
		}); err != nil {
			return fmt.Errorf("open re-review: %w", err)
		}
		return nil
	})
	if err != nil || !revoked {
		return err
	}
	s.invalidate(ctx, profileUUID)
	if row, err := s.latest(ctx, id); err == nil && row != nil {
		if review, err := s.Get(ctx, row.Uuid); err == nil {
			s.notify(ctx, review, "profiles.verification.re_review", "")
		}
	}
	return nil
}

// List returns verification requests across all accounts, pending first (oldest first).
func (s *Service) List(ctx context.Context, f model.ListFilter) ([]model.Review, int64, error) {
	if f.Limit <= 0 || f.Limit > 100 {
		f.Limit = 20
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
	switch f.Status {
	case "", model.RequestPending, model.RequestApproved, model.RequestRejected, model.RequestRevoked, model.RequestCancelled:
	default:
		return nil, 0, fmt.Errorf("%w: unknown status %q", ErrInvalidRequest, f.Status)
	}
	q := strings.TrimSpace(f.Q)
	rows, err := s.q.ListProfileVerifications(ctx, db.ListProfileVerificationsParams{
		Status: f.Status, Q: q, RowLimit: f.Limit, RowOffset: f.Offset,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list verifications: %w", err)
	}
	total, err := s.q.CountProfileVerifications(ctx, db.CountProfileVerificationsParams{Status: f.Status, Q: q})
	if err != nil {
		return nil, 0, fmt.Errorf("count verifications: %w", err)
	}
	out := make([]model.Review, 0, len(rows))
	for _, r := range rows {
		out = append(out, toReview(db.GetProfileVerificationRowRow(r)))
	}
	return out, total, nil
}

// Get returns one request for review.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*model.Review, error) {
	row, err := s.q.GetProfileVerificationRow(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get verification: %w", err)
	}
	r := toReview(row)
	return &r, nil
}

// Approve verifies the profile of a pending request.
func (s *Service) Approve(ctx context.Context, p authctx.Principal, id uuid.UUID) (*model.Review, error) {
	return s.review(ctx, p, id, model.RequestApproved, "")
}

// Reject declines a pending request; the reason is shown to the owner.
func (s *Service) Reject(ctx context.Context, p authctx.Principal, id uuid.UUID, reason string) (*model.Review, error) {
	reason, err := requireReason(reason)
	if err != nil {
		return nil, err
	}
	return s.review(ctx, p, id, model.RequestRejected, reason)
}

func (s *Service) review(ctx context.Context, p authctx.Principal, id uuid.UUID, status, reason string) (*model.Review, error) {
	if _, err := s.q.ReviewProfileVerification(ctx, db.ReviewProfileVerificationParams{
		Status: status, ReviewedByUserID: optUserID(p.UserInternal), ReviewReason: reason, Uuid: id,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, s.notPending(ctx, id, "pending")
		}
		return nil, fmt.Errorf("review verification: %w", err)
	}
	out, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	s.invalidate(ctx, out.Profile.UUID)
	if status == model.RequestApproved {
		s.notify(ctx, out, "profiles.verification.approved", "")
	} else {
		s.notify(ctx, out, "profiles.verification.rejected", reason)
	}
	return out, nil
}

// Revoke removes the badge of an approved request; the owner may request again.
func (s *Service) Revoke(ctx context.Context, p authctx.Principal, id uuid.UUID, reason string) (*model.Review, error) {
	reason, err := requireReason(reason)
	if err != nil {
		return nil, err
	}
	if _, err := s.q.RevokeProfileVerificationByUUID(ctx, db.RevokeProfileVerificationByUUIDParams{
		RevokedByUserID: optUserID(p.UserInternal), RevokeReason: reason, Uuid: id,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, s.notPending(ctx, id, "approved")
		}
		return nil, fmt.Errorf("revoke verification: %w", err)
	}
	out, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	s.invalidate(ctx, out.Profile.UUID)
	s.notify(ctx, out, "profiles.verification.revoked", reason)
	return out, nil
}

// notPending tells a missing request (404) from one in the wrong state (409).
func (s *Service) notPending(ctx context.Context, id uuid.UUID, want string) error {
	if _, err := s.Get(ctx, id); err != nil {
		return err
	}
	return fmt.Errorf("%w: the request is not %s", ErrConflict, want)
}

func (s *Service) invalidate(ctx context.Context, profileUUID uuid.UUID) {
	if s.cache != nil {
		s.cache.InvalidateProfile(ctx, profileUUID)
	}
}

// notify tells the account's owners and admins about a review outcome. Fail-soft: the review is
// already stored.
func (s *Service) notify(ctx context.Context, r *model.Review, code, reason string) {
	if s.notifier == nil || r == nil {
		return
	}
	ids, err := s.q.ListProfileVerificationNotifyUserIDs(ctx, r.AccountID)
	if err != nil {
		s.log.Warn("profile_verification_notify_recipients_failed", "error", err)
		return
	}
	name := strings.TrimSpace(r.Profile.FirstName + " " + r.Profile.LastName)
	if name == "" {
		name = r.Profile.Username
	}
	action := "/profiles/" + r.Profile.UUID.String()
	channels := []string{notifmodel.ChannelInapp}
	if code == "profiles.verification.approved" || code == "profiles.verification.rejected" {
		channels = append(channels, notifmodel.ChannelEmail)
	}
	for _, id := range ids {
		uid := id
		if _, err := s.notifier.Enqueue(ctx, notifmodel.EnqueueInput{
			UserID:       &uid,
			AccountID:    &r.AccountID,
			Channels:     channels,
			TemplateCode: code,
			TemplateVars: map[string]string{"profile_name": name, "reason": reason, "profile_url": s.frontendURL + action},
			ActionURL:    &action,
			Payload:      map[string]any{"profile_uuid": r.Profile.UUID.String(), "verification_uuid": r.UUID.String()},
			SourceEvent:  code,
		}); err != nil {
			s.log.Warn("profile_verification_notify_failed", "error", err)
		}
	}
}

func (s *Service) withTx(ctx context.Context, fn func(q *db.Queries) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(s.q.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func requireReason(reason string) (string, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "", fmt.Errorf("%w: reason is required", ErrInvalidRequest)
	}
	if utf8.RuneCountInString(reason) > model.MaxReasonLength {
		return "", fmt.Errorf("%w: reason is longer than %d characters", ErrInvalidRequest, model.MaxReasonLength)
	}
	return reason, nil
}

func toReview(r db.GetProfileVerificationRowRow) model.Review {
	return model.Review{
		UUID: r.Uuid, Status: r.Status, Origin: r.Origin, Note: r.Note,
		RequestedAt: r.CreatedAt.Time, RequestedByEmail: r.RequestedByEmail,
		ReviewedAt: timePtr(r.ReviewedAt), ReviewReason: r.ReviewReason,
		RevokedAt: timePtr(r.RevokedAt), RevokeReason: r.RevokeReason,
		ReviewedByEmail: r.ReviewedByEmail, AccountID: r.AccountID,
		Profile: model.ReviewProfile{
			UUID: r.ProfileUuid, FirstName: r.FirstName, LastName: r.LastName,
			CompanyName: r.CompanyName, Title: r.Title, AvatarURL: r.AvatarUrl, Username: r.Username,
		},
		Account: model.ReviewAccount{UUID: r.AccountUuid, Name: r.AccountName, Type: r.AccountType},
	}
}

func timePtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}

func optUserID(id int64) pgtype.Int8 { return pgtype.Int8{Int64: id, Valid: id != 0} }

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
