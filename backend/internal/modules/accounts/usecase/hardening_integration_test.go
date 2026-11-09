package usecase_test

import (
	"errors"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/accounts/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
)

func (e *inviteEnv) members() *usecase.Service {
	s := usecase.New(e.d.Queries)
	s.SetInvites(e.svc)
	return s
}

func withInvitePerm(p authctx.Principal) authctx.Principal {
	p.Permissions = []string{rbac.PermAuthSession, rbac.PermAppAccountInvitesWrite}
	return p
}

// TestIntegrationAddMemberRequiresVerifiedEmail: adding by email joins only a verified user. An
// unverified user (someone may have registered a colleague's address) or an unknown address
// gets an invite to the address instead, so only the mailbox owner can join (TEC-1104).
func TestIntegrationAddMemberRequiresVerifiedEmail(t *testing.T) {
	e := newInviteEnv(t)
	c := e.company("Verified Co")
	svc := e.members()
	owner := withInvitePerm(asRole(c.account, c.owner, authctx.AccountRoleOwner))

	squatter := e.d.CreateUnverifiedUser(t)
	res, err := svc.AddMember(e.ctx, owner, e.email(squatter), "admin")
	if err != nil {
		t.Fatal(err)
	}
	if res.Member != nil || res.Invite == nil || res.Invite.Email != e.email(squatter) || res.Invite.Role != "admin" {
		t.Fatalf("unverified user: %+v", res)
	}
	if _, ok := e.role(c.account, squatter); ok {
		t.Fatal("unverified user was added directly")
	}
	if e.mail.count() != 1 || *e.mail.sent[0].Recipient != e.email(squatter) {
		t.Fatalf("invite mail: %+v", e.mail.sent)
	}

	unknown := "nobody-" + pgtest.Unique() + "@example.test"
	if res, err := svc.AddMember(e.ctx, owner, unknown, ""); err != nil || res.Invite == nil || res.Invite.Role != "member" {
		t.Fatalf("unknown address: %+v %v", res, err)
	}

	verified := e.d.CreateUser(t)
	res, err = svc.AddMember(e.ctx, owner, e.email(verified), "member")
	if err != nil || res.Member == nil || res.Invite != nil || res.Member.UserUUID != verified.UUID {
		t.Fatalf("verified user: %+v %v", res, err)
	}
	if role, ok := e.role(c.account, verified); !ok || role != "member" {
		t.Fatalf("verified user role %q %v", role, ok)
	}

	// The invite fallback needs the invite permission, like POST /v1/account/invites.
	noPerm := asRole(c.account, c.owner, authctx.AccountRoleOwner)
	if _, err := svc.AddMember(e.ctx, noPerm, "other-"+pgtest.Unique()+"@example.test", ""); !errors.Is(err, usecase.ErrForbidden) {
		t.Fatalf("without invite permission: %v", err)
	}
}

// TestIntegrationRemoveMemberRevokesOpenInvites: removing a member (or the member leaving)
// revokes their open invites to the account, so an old link cannot bring them back (TEC-1104).
func TestIntegrationRemoveMemberRevokesOpenInvites(t *testing.T) {
	e := newInviteEnv(t)
	c := e.company("Revoke Co")
	other := e.company("Other Co")
	svc := e.members()
	owner := withInvitePerm(asRole(c.account, c.owner, authctx.AccountRoleOwner))

	for _, tc := range []struct {
		name   string
		remove func(u pgtest.User) error
	}{
		{"removed by owner", func(u pgtest.User) error { return svc.RemoveMember(e.ctx, owner, u.UUID) }},
		{"leaves", func(u pgtest.User) error {
			return svc.RemoveMember(e.ctx, asRole(c.account, u, authctx.AccountRoleMember), u.UUID)
		}},
		{"removed by platform", func(u pgtest.User) error { return svc.PlatformRemoveMember(e.ctx, c.account.Uuid, u.UUID) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u := e.d.CreateUser(t)
			if _, err := e.svc.Create(e.ctx, owner, e.email(u), "admin"); err != nil {
				t.Fatal(err)
			}
			token := e.mail.lastToken(t)
			otherOwner := asRole(other.account, other.owner, authctx.AccountRoleOwner)
			if _, err := e.svc.Create(e.ctx, otherOwner, e.email(u), "member"); err != nil {
				t.Fatal(err)
			}
			otherToken := e.mail.lastToken(t)
			if res, err := svc.AddMember(e.ctx, owner, e.email(u), "member"); err != nil || res.Member == nil {
				t.Fatalf("add: %+v %v", res, err)
			}
			if err := tc.remove(u); err != nil {
				t.Fatal(err)
			}
			if _, err := e.svc.Accept(e.ctx, u.ID, token); !errors.Is(err, usecase.ErrInviteClosed) {
				t.Fatalf("old invite after removal: %v", err)
			}
			if _, ok := e.role(c.account, u); ok {
				t.Fatal("removed member rejoined through the old invite")
			}
			if pv, err := e.svc.Preview(e.ctx, token); err != nil || pv.Status != usecase.InviteStatusRevoked {
				t.Fatalf("preview after removal: %+v %v", pv, err)
			}
			// Invites to other accounts are untouched.
			if pv, err := e.svc.Preview(e.ctx, otherToken); err != nil || pv.Status != usecase.InviteStatusPending {
				t.Fatalf("other account's invite: %+v %v", pv, err)
			}
		})
	}
}
