package usecase_test

import (
	"errors"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/businesspages/model"
	leadsmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/leads/model"
	leadsrepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/leads/repository"
	leadsusecase "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/leads/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/entitlement"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
)

// TEC-1327 C: an enabled lead form button turns the business page route's form on; the lead
// goes to the company's inbox as a business_page lead, owners/admins only.
func TestIntegrationBusinessPageLeadForm(t *testing.T) {
	e := newEnv(t)
	acme, acmeOwner := e.company("Lead Co")
	memberUser := e.d.CreateUser(t)
	e.member(acme, memberUser, db.AccountRoleMember)
	owner := principal(acme.ID, authctx.AccountTypeCompany, acmeOwner, authctx.AccountRoleOwner)
	leads := leadsusecase.New(leadsrepo.NewPostgres(e.d.Queries), nil, nil).SetQuotas(entitlement.New(e.d.Queries))
	username := "leadco-" + pgtest.Unique()

	page := model.SaveInput{
		Username: username,
		Blocks: []model.BlockInput{
			{Type: model.BlockHeader},
			{Type: model.BlockButton, Config: raw(model.ButtonConfig{Target: model.TargetURL, Value: "https://leadco.test"})},
		},
	}
	if _, err := e.svc.Save(e.ctx, owner, page); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.SetPublished(e.ctx, owner, true); err != nil {
		t.Fatal(err)
	}
	// No lead form button: the form is off.
	cfg, err := leads.FormConfig(e.ctx, username)
	if err != nil || cfg.Enabled {
		t.Fatalf("form without button: %+v %v", cfg, err)
	}
	submit := leadsmodel.SubmitInput{Path: username, Name: "Ziyaretçi", Email: "v@example.test", Consent: true, ConsentVersion: leadsmodel.ConsentVersion}
	if _, err := leads.Submit(e.ctx, submit, leadsmodel.Visitor{IP: "203.0.113.9"}); !errors.Is(err, leadsusecase.ErrFormUnavailable) {
		t.Fatalf("submit without form: %v", err)
	}

	// A lead form button turns it on.
	page.Blocks = append(page.Blocks, model.BlockInput{Type: model.BlockButton, Config: raw(model.ButtonConfig{Target: model.TargetLeadForm})})
	if _, err := e.svc.Save(e.ctx, owner, page); err != nil {
		t.Fatal(err)
	}
	cfg, err = leads.FormConfig(e.ctx, username)
	if err != nil || !cfg.Enabled || cfg.SourceType != leadsmodel.SourceBusinessPage || cfg.ProfileUUID != nil {
		t.Fatalf("form with button: %+v %v", cfg, err)
	}
	res, err := leads.Submit(e.ctx, submit, leadsmodel.Visitor{IP: "203.0.113.9"})
	if err != nil {
		t.Fatal(err)
	}
	ownerP := authctx.Principal{UserID: acmeOwner.UUID, UserInternal: acmeOwner.ID,
		Permissions: []string{rbac.PermAppLeadsRead},
		Account:     authctx.Account{ID: acme.ID, Type: authctx.AccountTypeCompany, Role: authctx.AccountRoleOwner}}
	got, err := leads.Get(e.ctx, ownerP, res.UUID)
	if err != nil || got.SourceType != leadsmodel.SourceBusinessPage || got.RoutePath != username {
		t.Fatalf("owner reads lead: %+v %v", got, err)
	}
	memberP := ownerP
	memberP.UserID, memberP.UserInternal, memberP.Account.Role = memberUser.UUID, memberUser.ID, authctx.AccountRoleMember
	if _, err := leads.Get(e.ctx, memberP, res.UUID); err == nil {
		t.Fatal("a member reads a business page lead")
	}

	// A disabled lead form button turns it off again; a draft page has no form either.
	page.Blocks[2].Enabled = new(bool)
	if _, err := e.svc.Save(e.ctx, owner, page); err != nil {
		t.Fatal(err)
	}
	if cfg, _ := leads.FormConfig(e.ctx, username); cfg.Enabled {
		t.Fatal("disabled button keeps the form on")
	}
	page.Blocks[2].Enabled = nil
	if _, err := e.svc.Save(e.ctx, owner, page); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.SetPublished(e.ctx, owner, false); err != nil {
		t.Fatal(err)
	}
	if cfg, _ := leads.FormConfig(e.ctx, username); cfg.Enabled {
		t.Fatal("draft page has a form")
	}
}
