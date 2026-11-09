package usecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	notifmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// InviteTTL is how long an invite link stays valid after it was (re)sent.
const InviteTTL = 7 * 24 * time.Hour

// Invite statuses as the API shows them. "expired" is a pending row past expires_at.
const (
	InviteStatusPending  = "pending"
	InviteStatusExpired  = "expired"
	InviteStatusAccepted = "accepted"
	InviteStatusRevoked  = "revoked"
)

var (
	ErrInviteExpired       = errors.New("invite has expired")
	ErrInviteClosed        = errors.New("invite is no longer open")
	ErrInviteEmailMismatch = errors.New("invite was sent to another email address")
)

// InviteNotifier queues the invite email (notifications usecase.Service).
type InviteNotifier interface {
	Enqueue(ctx context.Context, in notifmodel.EnqueueInput) ([]notifmodel.Notification, error)
}

// Invites manages email invitations to company accounts (TEC-945). App methods act on the
// caller's active account and need an owner/admin role there; the route adds the
// app.account_invites.* permission. The link holds a random token; only its hash is stored.
//
// Mail goes out only when an invite is created or resent, never for existing users added
// directly (legacy-migrated users are not emailed).
type Invites struct {
	pool        *pgxpool.Pool
	q           *db.Queries
	notifier    InviteNotifier
	frontendURL string
	log         *slog.Logger
	now         func() time.Time
}

func NewInvites(pool *pgxpool.Pool, q *db.Queries, notifier InviteNotifier, frontendURL string, log *slog.Logger) *Invites {
	if log == nil {
		log = slog.Default()
	}
	return &Invites{
		pool: pool, q: q, notifier: notifier, frontendURL: strings.TrimRight(frontendURL, "/"),
		log: log, now: time.Now,
	}
}

// Invite is an invite as the account's owners/admins see it.
type Invite struct {
	UUID          uuid.UUID  `json:"uuid"`
	Email         string     `json:"email"`
	Role          string     `json:"role"`
	Status        string     `json:"status"`
	Source        string     `json:"source"`
	ExpiresAt     string     `json:"expires_at"`
	LastSentAt    string     `json:"last_sent_at,omitempty"`
	SendCount     int        `json:"send_count"`
	AcceptedAt    string     `json:"accepted_at,omitempty"`
	RevokedAt     string     `json:"revoked_at,omitempty"`
	ProfileUUID   *uuid.UUID `json:"profile_uuid"`
	OfficeUUID    *uuid.UUID `json:"office_uuid"`
	GroupUUID     *uuid.UUID `json:"group_uuid"`
	InvitedByUUID *uuid.UUID `json:"invited_by_uuid"`
	InvitedByName string     `json:"invited_by_name"`
	CreatedAt     string     `json:"created_at"`
}

// InviteFilter narrows the invite list. Status "" lists open invites (pending and expired).
type InviteFilter struct {
	Status string
	Q      string
	Limit  int32
	Offset int32
}

// InvitePreview is what the link holder sees before accepting.
type InvitePreview struct {
	AccountName   string `json:"account_name"`
	Email         string `json:"email"`
	Role          string `json:"role"`
	Status        string `json:"status"`
	ExpiresAt     string `json:"expires_at"`
	InvitedByName string `json:"invited_by_name"`
	// UserExists tells the page whether to offer login or sign-up.
	UserExists bool `json:"user_exists"`
}

// AcceptResult is the account the caller joined.
type AcceptResult struct {
	AccountUUID uuid.UUID  `json:"account_uuid"`
	AccountName string     `json:"account_name"`
	Role        string     `json:"role"`
	ProfileUUID *uuid.UUID `json:"profile_uuid"`
}

func (s *Invites) manager(p authctx.Principal) error {
	if p.Account.ID <= 0 || p.Account.IsPersonal() {
		return ErrPersonalAccount
	}
	if !p.Account.CanManageMembers() {
		return ErrForbidden
	}
	return nil
}

// List lists the active company account's invites (owner/admin).
func (s *Invites) List(ctx context.Context, p authctx.Principal, f InviteFilter) ([]Invite, int64, error) {
	if err := s.manager(p); err != nil {
		return nil, 0, err
	}
	status := pgtype.Text{}
	switch f.Status {
	case "":
	case InviteStatusPending, InviteStatusExpired, InviteStatusAccepted, InviteStatusRevoked:
		status = pgtype.Text{String: f.Status, Valid: true}
	default:
		return nil, 0, fmt.Errorf("%w: status must be pending, expired, accepted or revoked", ErrInvalidRequest)
	}
	if f.Limit <= 0 || f.Limit > 100 {
		f.Limit = 20
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
	now := pgtype.Timestamptz{Time: s.now(), Valid: true}
	q := strings.ToLower(strings.TrimSpace(f.Q))
	rows, err := s.q.ListAccountInvites(ctx, db.ListAccountInvitesParams{
		AccountID: p.Account.ID, Status: status, Now: now, Q: q, LimitCount: f.Limit, OffsetCount: f.Offset,
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.q.CountAccountInvites(ctx, db.CountAccountInvitesParams{AccountID: p.Account.ID, Status: status, Now: now, Q: q})
	if err != nil {
		return nil, 0, err
	}
	out := make([]Invite, 0, len(rows))
	for _, r := range rows {
		out = append(out, s.mapInvite(db.GetAccountInviteRow(r)))
	}
	return out, total, nil
}

// Create invites email with role to the active company account and sends the link. Owners may
// invite any role; admins admins, office managers and members. An existing member or an open
// invite for the same address is a conflict (resend the open one instead).
func (s *Invites) Create(ctx context.Context, p authctx.Principal, email, role string) (Invite, error) {
	if err := s.manager(p); err != nil {
		return Invite{}, err
	}
	role, err := parseRole(role, authctx.AccountRoleMember)
	if err != nil {
		return Invite{}, err
	}
	if role == authctx.AccountRoleOwner && !p.Account.IsOwner() {
		return Invite{}, ErrForbidden
	}
	email, err = normalizeEmail(email)
	if err != nil {
		return Invite{}, err
	}
	if userID, err := s.q.GetActiveUserIDByEmail(ctx, email); err == nil {
		if _, err := s.q.GetAccountMemberIDByUser(ctx, db.GetAccountMemberIDByUserParams{AccountID: p.Account.ID, UserID: userID}); err == nil {
			return Invite{}, fmt.Errorf("%w: already a member", ErrConflict)
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return Invite{}, err
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return Invite{}, err
	}
	token, hash, err := newInviteToken()
	if err != nil {
		return Invite{}, err
	}
	now := s.now()
	id, err := s.q.CreateAccountInvite(ctx, db.CreateAccountInviteParams{
		AccountID: p.Account.ID, Email: email, Role: db.AccountRole(role), TokenHash: hash, Source: "manual",
		InvitedByUserID: pgtype.Int8{Int64: p.UserInternal, Valid: p.UserInternal > 0},
		ExpiresAt:       pgtype.Timestamptz{Time: now.Add(InviteTTL), Valid: true},
		LastSentAt:      pgtype.Timestamptz{Time: now, Valid: true}, SendCount: 1,
	})
	if err != nil {
		if mapped := mapWriteErr(err); errors.Is(mapped, ErrConflict) {
			return Invite{}, fmt.Errorf("%w: an open invite exists for this email; resend it", ErrConflict)
		}
		return Invite{}, err
	}
	inv, err := s.inviteByID(ctx, p.Account.ID, id)
	if err != nil {
		return Invite{}, err
	}
	if err := s.send(ctx, p.Account.ID, p.UserInternal, inv.Email, role, token); err != nil {
		return Invite{}, err
	}
	return inv, nil
}

// Resend issues a new link (the old one stops working) valid for another 7 days and mails it.
// Expired invites can be resent; accepted or revoked ones cannot.
func (s *Invites) Resend(ctx context.Context, p authctx.Principal, id uuid.UUID) (Invite, error) {
	if err := s.manager(p); err != nil {
		return Invite{}, err
	}
	row, err := s.q.GetAccountInvite(ctx, db.GetAccountInviteParams{AccountID: p.Account.ID, Uuid: id})
	if err != nil {
		return Invite{}, mapNotFound(err)
	}
	if string(row.Role) == authctx.AccountRoleOwner && !p.Account.IsOwner() {
		return Invite{}, ErrForbidden
	}
	token, err := s.renew(ctx, p.Account.ID, row.ID)
	if err != nil {
		return Invite{}, err
	}
	if err := s.send(ctx, p.Account.ID, p.UserInternal, row.Email, string(row.Role), token); err != nil {
		return Invite{}, err
	}
	return s.inviteByID(ctx, p.Account.ID, row.Uuid)
}

// Revoke closes an open invite; its link stops working. A profile the import created for it
// stays in the account.
func (s *Invites) Revoke(ctx context.Context, p authctx.Principal, id uuid.UUID) error {
	if err := s.manager(p); err != nil {
		return err
	}
	row, err := s.q.GetAccountInvite(ctx, db.GetAccountInviteParams{AccountID: p.Account.ID, Uuid: id})
	if err != nil {
		return mapNotFound(err)
	}
	if string(row.Role) == authctx.AccountRoleOwner && !p.Account.IsOwner() {
		return ErrForbidden
	}
	n, err := s.q.RevokeAccountInvite(ctx, db.RevokeAccountInviteParams{AccountID: p.Account.ID, ID: row.ID})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrInviteClosed
	}
	return nil
}

// Preview describes the invite behind token for the invite page (no session needed).
func (s *Invites) Preview(ctx context.Context, token string) (InvitePreview, error) {
	row, err := s.byToken(ctx, token)
	if err != nil {
		return InvitePreview{}, err
	}
	_, userErr := s.q.GetActiveUserIDByEmail(ctx, row.Email)
	if userErr != nil && !errors.Is(userErr, pgx.ErrNoRows) {
		return InvitePreview{}, userErr
	}
	return InvitePreview{
		AccountName: row.AccountName, Email: row.Email, Role: string(row.Role),
		Status: s.status(row.Status, row.ExpiresAt), ExpiresAt: formatTime(row.ExpiresAt),
		InvitedByName: row.InvitedByName, UserExists: userErr == nil,
	}, nil
}

// PendingFor reports whether token is an open, unexpired invite for email. Registration uses
// it to let an invited address sign up while self-registration is closed.
func (s *Invites) PendingFor(ctx context.Context, token, email string) bool {
	row, err := s.byToken(ctx, token)
	if err != nil {
		return false
	}
	return s.status(row.Status, row.ExpiresAt) == InviteStatusPending &&
		strings.EqualFold(row.Email, strings.TrimSpace(email)) && row.AccountStatus == db.AccountStatusActive
}

// Accept makes the signed-in user a member of the invite's account. The user's email must be
// the invited address. An existing member keeps their role. A profile the import created for
// the invite becomes the member's employee profile, and the member is placed in the invite's
// office/group, unless the member already has a profile link or a placement.
func (s *Invites) Accept(ctx context.Context, userID int64, token string) (AcceptResult, error) {
	row, err := s.byToken(ctx, token)
	if err != nil {
		return AcceptResult{}, err
	}
	user, err := s.q.GetUserForInvite(ctx, userID)
	if err != nil {
		return AcceptResult{}, mapNotFound(err)
	}
	if !strings.EqualFold(user.Email, row.Email) {
		return AcceptResult{}, ErrInviteEmailMismatch
	}
	if row.AccountStatus != db.AccountStatusActive {
		return AcceptResult{}, fmt.Errorf("%w: account is suspended", ErrForbidden)
	}
	res := AcceptResult{AccountUUID: row.AccountUuid, AccountName: row.AccountName, Role: string(row.Role)}
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		locked, err := q.LockAccountInvite(ctx, db.LockAccountInviteParams{ID: row.ID, TokenHash: hashInviteToken(strings.TrimSpace(token))})
		if err != nil {
			return mapNotFound(err)
		}
		switch s.status(locked.Status, locked.ExpiresAt) {
		case InviteStatusPending:
		case InviteStatusExpired:
			return ErrInviteExpired
		default:
			return ErrInviteClosed
		}
		member, err := q.GetAccountMemberIDByUser(ctx, db.GetAccountMemberIDByUserParams{AccountID: row.AccountID, UserID: user.ID})
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			if err := q.AddAccountMember(ctx, db.AddAccountMemberParams{AccountID: row.AccountID, UserID: user.ID, Role: row.Role}); err != nil {
				return mapWriteErr(err)
			}
			if member, err = q.GetAccountMemberIDByUser(ctx, db.GetAccountMemberIDByUserParams{AccountID: row.AccountID, UserID: user.ID}); err != nil {
				return err
			}
		case err != nil:
			return err
		}
		res.Role = string(member.Role)
		if row.ProfileID.Valid {
			if _, err := q.LinkInvitedMemberProfile(ctx, db.LinkInvitedMemberProfileParams{
				AccountID: row.AccountID, MemberID: member.ID, ProfileID: row.ProfileID.Int64,
			}); err != nil {
				return mapWriteErr(err)
			}
		}
		if row.OfficeID.Valid || row.GroupID.Valid {
			// Placement is best effort (the group may no longer fit the office): a savepoint keeps
			// a failed insert from aborting the join.
			sp, err := tx.Begin(ctx)
			if err != nil {
				return err
			}
			if _, err := s.q.WithTx(sp).AssignInvitedMember(ctx, db.AssignInvitedMemberParams{
				AccountID: row.AccountID, MemberID: member.ID, OfficeID: row.OfficeID, GroupID: row.GroupID,
			}); err != nil {
				s.log.Warn("account_invite_assignment_skipped", "invite", row.Uuid, "error", err)
				if err := sp.Rollback(ctx); err != nil {
					return err
				}
			} else if err := sp.Commit(ctx); err != nil {
				return err
			}
		}
		n, err := q.MarkAccountInviteAccepted(ctx, db.MarkAccountInviteAcceptedParams{ID: row.ID, UserID: pgtype.Int8{Int64: user.ID, Valid: true}})
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrInviteClosed
		}
		return nil
	})
	if err != nil {
		return AcceptResult{}, err
	}
	s.fillProfile(ctx, row.AccountID, user.ID, &res)
	return res, nil
}

func (s *Invites) fillProfile(ctx context.Context, accountID, userID int64, res *AcceptResult) {
	member, err := s.q.GetAccountMemberIDByUser(ctx, db.GetAccountMemberIDByUserParams{AccountID: accountID, UserID: userID})
	if err != nil {
		return
	}
	if link, err := s.q.GetCompanyMemberLink(ctx, db.GetCompanyMemberLinkParams{AccountID: accountID, MemberID: member.ID}); err == nil && !link.Deleted {
		id := link.ProfileUuid
		res.ProfileUUID = &id
	}
}

// ---- Employee import (companyprofiles) ----

// InviteEmployee opens an invite (role member, source import) for an employee row whose email
// has no user, attaching the profile the import just created and the row's office/group. An
// open invite for the address is reused: the attachments are replaced, and an expired one is
// renewed and resent. created reports a new invite. Exactly one email goes out for a new or
// renewed invite. The caller (the import) authorizes the actor.
func (s *Invites) InviteEmployee(ctx context.Context, accountID, invitedBy int64, email string, profile uuid.UUID, office, group *uuid.UUID) (uuid.UUID, bool, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return uuid.Nil, false, err
	}
	now := s.now()
	existing, err := s.q.GetPendingAccountInviteByEmail(ctx, db.GetPendingAccountInviteByEmailParams{AccountID: accountID, Email: email})
	switch {
	case err == nil:
		if _, err := s.q.SetAccountInviteProfile(ctx, db.SetAccountInviteProfileParams{
			AccountID: accountID, ID: existing.ID, ProfileUuid: pgUUID(&profile), OfficeUuid: pgUUID(office), GroupUuid: pgUUID(group),
		}); err != nil {
			return uuid.Nil, false, mapWriteErr(err)
		}
		if existing.ExpiresAt.Valid && !existing.ExpiresAt.Time.After(now) {
			token, err := s.renew(ctx, accountID, existing.ID)
			if err != nil {
				return uuid.Nil, false, err
			}
			if err := s.send(ctx, accountID, invitedBy, email, authctx.AccountRoleMember, token); err != nil {
				return uuid.Nil, false, err
			}
		}
		return existing.Uuid, false, nil
	case !errors.Is(err, pgx.ErrNoRows):
		return uuid.Nil, false, err
	}
	token, hash, err := newInviteToken()
	if err != nil {
		return uuid.Nil, false, err
	}
	id, err := s.q.CreateAccountInvite(ctx, db.CreateAccountInviteParams{
		AccountID: accountID, Email: email, Role: db.AccountRoleMember, TokenHash: hash, Source: "import",
		InvitedByUserID: pgtype.Int8{Int64: invitedBy, Valid: invitedBy > 0},
		ProfileUuid:     pgUUID(&profile), OfficeUuid: pgUUID(office), GroupUuid: pgUUID(group),
		ExpiresAt:  pgtype.Timestamptz{Time: now.Add(InviteTTL), Valid: true},
		LastSentAt: pgtype.Timestamptz{Time: now, Valid: true}, SendCount: 1,
	})
	if err != nil {
		return uuid.Nil, false, mapWriteErr(err)
	}
	if err := s.send(ctx, accountID, invitedBy, email, authctx.AccountRoleMember, token); err != nil {
		_ = s.UndoEmployeeInvite(ctx, accountID, id, true, nil, nil, nil)
		return uuid.Nil, false, err
	}
	return id, true, nil
}

// UndoEmployeeInvite reverts InviteEmployee for an import rollback: a created invite is deleted
// (its link stops working), a reused one gets its previous attachments back.
func (s *Invites) UndoEmployeeInvite(ctx context.Context, accountID int64, invite uuid.UUID, created bool, profile, office, group *uuid.UUID) error {
	row, err := s.q.GetAccountInvite(ctx, db.GetAccountInviteParams{AccountID: accountID, Uuid: invite})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if created {
		if row.Status != InviteStatusPending {
			return nil // already accepted or revoked: the invitee's membership is theirs now
		}
		_, err = s.q.DeleteAccountInvite(ctx, db.DeleteAccountInviteParams{AccountID: accountID, ID: row.ID})
		return err
	}
	_, err = s.q.SetAccountInviteProfile(ctx, db.SetAccountInviteProfileParams{
		AccountID: accountID, ID: row.ID, ProfileUuid: pgUUID(profile), OfficeUuid: pgUUID(office), GroupUuid: pgUUID(group),
	})
	return mapWriteErr(err)
}

// ---- helpers ----

func (s *Invites) renew(ctx context.Context, accountID, id int64) (string, error) {
	token, hash, err := newInviteToken()
	if err != nil {
		return "", err
	}
	now := s.now()
	n, err := s.q.RenewAccountInvite(ctx, db.RenewAccountInviteParams{
		AccountID: accountID, ID: id, TokenHash: hash,
		ExpiresAt: pgtype.Timestamptz{Time: now.Add(InviteTTL), Valid: true},
		SentAt:    pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		return "", err
	}
	if n == 0 {
		return "", ErrInviteClosed
	}
	return token, nil
}

// send queues the invite email to the invited address. The recipient may have no user, so the
// row is addressed by email only and in the user's language when one exists.
func (s *Invites) send(ctx context.Context, accountID, invitedBy int64, email, role, token string) error {
	if s.notifier == nil {
		return nil
	}
	account, err := s.q.GetAccountByID(ctx, accountID)
	if err != nil {
		return err
	}
	inviter := account.Name
	if invitedBy > 0 {
		if u, err := s.q.GetUserForInvite(ctx, invitedBy); err == nil {
			inviter = strings.TrimSpace(u.Name + " " + u.Surname)
		}
	}
	lang := "tr"
	if id, err := s.q.GetActiveUserIDByEmail(ctx, email); err == nil {
		if u, err := s.q.GetUserForInvite(ctx, id); err == nil && u.Locale != "" {
			lang = u.Locale
		}
	}
	link := s.InviteURL(token)
	_, err = s.notifier.Enqueue(ctx, notifmodel.EnqueueInput{
		Channels: []string{notifmodel.ChannelEmail}, Recipient: &email, ActionURL: &link,
		TemplateCode: "accounts.invite", SourceEvent: "accounts.invite", Language: lang,
		// The link carries the invitation token: masked on the row, sealed for the mailbox.
		SecretVars: []string{"invite_url"},
		TemplateVars: map[string]string{
			"account_name": account.Name, "inviter_name": inviter, "role": roleLabel(role, lang),
			"invite_url": link, "expires_days": strconv.Itoa(int(InviteTTL.Hours() / 24)),
		},
	})
	return err
}

// InviteURL is the page the email links to.
func (s *Invites) InviteURL(token string) string {
	return s.frontendURL + "/platform/invite/" + token
}

func (s *Invites) byToken(ctx context.Context, token string) (db.GetAccountInviteByTokenHashRow, error) {
	token = strings.TrimSpace(token)
	if token == "" || len(token) > 128 {
		return db.GetAccountInviteByTokenHashRow{}, ErrNotFound
	}
	row, err := s.q.GetAccountInviteByTokenHash(ctx, hashInviteToken(token))
	if err != nil {
		return row, mapNotFound(err)
	}
	if row.AccountType != db.AccountTypeCompany {
		return row, ErrNotFound
	}
	return row, nil
}

func (s *Invites) inviteByID(ctx context.Context, accountID int64, id uuid.UUID) (Invite, error) {
	row, err := s.q.GetAccountInvite(ctx, db.GetAccountInviteParams{AccountID: accountID, Uuid: id})
	if err != nil {
		return Invite{}, mapNotFound(err)
	}
	return s.mapInvite(row), nil
}

func (s *Invites) status(stored string, expires pgtype.Timestamptz) string {
	if stored == InviteStatusPending && expires.Valid && !expires.Time.After(s.now()) {
		return InviteStatusExpired
	}
	return stored
}

func (s *Invites) mapInvite(r db.GetAccountInviteRow) Invite {
	return Invite{
		UUID: r.Uuid, Email: r.Email, Role: string(r.Role), Status: s.status(r.Status, r.ExpiresAt), Source: r.Source,
		ExpiresAt: formatTime(r.ExpiresAt), LastSentAt: formatTime(r.LastSentAt), SendCount: int(r.SendCount),
		AcceptedAt: formatTime(r.AcceptedAt), RevokedAt: formatTime(r.RevokedAt),
		ProfileUUID: fromPgUUID(r.ProfileUuid), OfficeUUID: fromPgUUID(r.OfficeUuid), GroupUUID: fromPgUUID(r.GroupUuid),
		InvitedByUUID: fromPgUUID(r.InvitedByUuid), InvitedByName: r.InvitedByName, CreatedAt: formatTime(r.CreatedAt),
	}
}

func newInviteToken() (string, []byte, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	return token, hashInviteToken(token), nil
}

func hashInviteToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

func normalizeEmail(v string) (string, error) {
	v = strings.ToLower(strings.TrimSpace(v))
	at := strings.Index(v, "@")
	if v == "" || len(v) > 255 || at <= 0 || at == len(v)-1 || strings.ContainsAny(v, " \t\r\n<>,;") {
		return "", fmt.Errorf("%w: a valid email is required", ErrInvalidRequest)
	}
	return v, nil
}

func roleLabel(role, lang string) string {
	tr := map[string]string{"owner": "sahip", "admin": "yönetici", "office_manager": "ofis yöneticisi", "member": "üye"}
	en := map[string]string{"owner": "owner", "admin": "admin", "office_manager": "office manager", "member": "member"}
	if lang == "tr" {
		return tr[role]
	}
	return en[role]
}

func pgUUID(id *uuid.UUID) pgtype.UUID {
	if id == nil || *id == uuid.Nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *id, Valid: true}
}

func fromPgUUID(id pgtype.UUID) *uuid.UUID {
	if !id.Valid {
		return nil
	}
	v := uuid.UUID(id.Bytes)
	return &v
}
