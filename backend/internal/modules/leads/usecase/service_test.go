package usecase

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/leads/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/leads/repository"
	notifmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/notifications/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/google/uuid"
)

func validInput() model.SubmitInput {
	return model.SubmitInput{
		Path: "ahmet", Name: "Ayşe Yılmaz", Email: "ayse@example.com",
		Consent: true, ConsentVersion: model.ConsentVersion,
	}
}

func fieldCodes(err error) map[string]string {
	var verr *ValidationError
	if !errors.As(err, &verr) {
		return nil
	}
	out := map[string]string{}
	for _, f := range verr.Fields {
		out[f.Field] = f.Code
	}
	return out
}

func TestValidate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		mutate func(*model.SubmitInput)
		want   map[string]string // field -> code; nil = valid
	}{
		{"valid email only", func(*model.SubmitInput) {}, nil},
		{"valid phone only", func(in *model.SubmitInput) { in.Email = ""; in.Phone = "+90 (532) 123-45-67" }, nil},
		{"name required", func(in *model.SubmitInput) { in.Name = "   " }, map[string]string{"name": "required"}},
		{"name too long", func(in *model.SubmitInput) { in.Name = strings.Repeat("a", model.MaxName+1) }, map[string]string{"name": "too_long"}},
		{"name control chars", func(in *model.SubmitInput) { in.Name = "a\u0000b" }, map[string]string{"name": "invalid"}},
		{"email or phone", func(in *model.SubmitInput) { in.Email = "" }, map[string]string{"email": "email_or_phone_required"}},
		{"bad email", func(in *model.SubmitInput) { in.Email = "not-an-email" }, map[string]string{"email": "invalid"}},
		{"display name email", func(in *model.SubmitInput) { in.Email = "Ayse <ayse@example.com>" }, map[string]string{"email": "invalid"}},
		{"email without dot domain", func(in *model.SubmitInput) { in.Email = "a@localhost" }, map[string]string{"email": "invalid"}},
		{"bad phone letters", func(in *model.SubmitInput) { in.Phone = "call me" }, map[string]string{"phone": "invalid"}},
		{"phone too short", func(in *model.SubmitInput) { in.Phone = "12345" }, map[string]string{"phone": "invalid"}},
		{"plus in the middle", func(in *model.SubmitInput) { in.Phone = "90+5321234567" }, map[string]string{"phone": "invalid"}},
		{"message too long", func(in *model.SubmitInput) { in.Message = strings.Repeat("x", model.MaxMessage+1) }, map[string]string{"message": "too_long"}},
		{"consent required", func(in *model.SubmitInput) { in.Consent = false }, map[string]string{"consent": "required"}},
		{"unknown consent version", func(in *model.SubmitInput) { in.ConsentVersion = "v0" }, map[string]string{"consent_version": "invalid"}},
		{"path required", func(in *model.SubmitInput) { in.Path = " / " }, map[string]string{"path": "required"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			in := validInput()
			tc.mutate(&in)
			_, err := Validate(in)
			if tc.want == nil {
				if err != nil {
					t.Fatalf("want valid, got %v", err)
				}
				return
			}
			got := fieldCodes(err)
			for field, code := range tc.want {
				if got[field] != code {
					t.Fatalf("field %s: want %q, got %v (err %v)", field, code, got, err)
				}
			}
		})
	}
}

func TestValidateNormalises(t *testing.T) {
	t.Parallel()
	out, err := Validate(model.SubmitInput{
		Path: " /c/ABC123/ ", Name: "  Ayşe   Yılmaz ", Email: " AYSE@Example.COM ",
		Message: "  merhaba  ", Locale: "EN", Consent: true, ConsentVersion: model.ConsentVersion,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Path != "c/ABC123" || out.Name != "Ayşe Yılmaz" || out.Email != "ayse@example.com" ||
		out.Message != "merhaba" || out.Locale != "en" {
		t.Fatalf("not normalised: %+v", out)
	}
	if out, _ := Validate(model.SubmitInput{Locale: "de"}); out.Locale != "" {
		t.Fatalf("unknown locale kept: %q", out.Locale)
	}
}

type memRepo struct {
	routes  map[string]*repository.CaptureRoute
	created []repository.NewLead
	leads   map[uuid.UUID]int64 // lead -> account
	owners  []int64
	senders map[uuid.UUID]memSender
	onlyFor map[int64]map[uuid.UUID]bool // viewer -> leads they may read (unset: all)
	names   map[int64]string
	deleted []uuid.UUID
	stored  map[uuid.UUID]repository.NewLead // what Create stored; ConsentAt stands for created_at
}

type memSender struct {
	accountID int64
	profile   repository.SenderProfile
}

func newMemRepo() *memRepo {
	return &memRepo{
		routes: map[string]*repository.CaptureRoute{}, leads: map[uuid.UUID]int64{},
		senders: map[uuid.UUID]memSender{}, stored: map[uuid.UUID]repository.NewLead{},
	}
}

func (r *memRepo) CaptureRoute(_ context.Context, path string) (*repository.CaptureRoute, error) {
	return r.routes[strings.ToLower(path)], nil
}

func (r *memRepo) Create(_ context.Context, in repository.NewLead) (model.SubmitResult, error) {
	r.created = append(r.created, in)
	id := uuid.New()
	r.leads[id] = in.AccountID
	r.stored[id] = in
	return model.SubmitResult{UUID: id, CreatedAt: in.ConsentAt}, nil
}

func (r *memRepo) List(_ context.Context, scope repository.Scope, _ model.ListFilter) ([]model.Lead, int64, error) {
	var out []model.Lead
	for id, acc := range r.leads {
		if acc == scope.AccountID && r.visible(scope, id) {
			out = append(out, r.lead(id))
		}
	}
	return out, int64(len(out)), nil
}

func (r *memRepo) Get(_ context.Context, scope repository.Scope, id uuid.UUID) (*model.Lead, error) {
	if acc, ok := r.leads[id]; ok && acc == scope.AccountID && r.visible(scope, id) {
		l := r.lead(id)
		return &l, nil
	}
	return nil, nil
}

func (r *memRepo) lead(id uuid.UUID) model.Lead {
	in := r.stored[id]
	return model.Lead{
		UUID: id, Kind: in.Kind, Name: in.Name, Email: in.Email, Phone: in.Phone, Message: in.Message,
		Locked: in.Locked, CreatedAt: in.ConsentAt,
	}
}

func (r *memRepo) HasLocked(_ context.Context, accountID int64) (bool, error) {
	for id, in := range r.stored {
		if in.Locked && r.leads[id] == accountID {
			return true, nil
		}
	}
	return false, nil
}

func (r *memRepo) UnlockBefore(_ context.Context, accountID int64, start time.Time) (int64, error) {
	var n int64
	for id, in := range r.stored {
		if in.Locked && in.AccountID == accountID && in.ConsentAt.Before(start) {
			in.Locked = false
			r.stored[id] = in
			n++
		}
	}
	return n, nil
}

func (r *memRepo) UnlockMonth(_ context.Context, accountID int64, kind string, start time.Time, maxRows int32) (int64, error) {
	var ids []uuid.UUID
	for id, in := range r.stored {
		if in.Locked && in.AccountID == accountID && in.Kind == kind && !in.ConsentAt.Before(start) {
			ids = append(ids, id)
		}
	}
	slices.SortFunc(ids, func(a, b uuid.UUID) int { return r.stored[a].ConsentAt.Compare(r.stored[b].ConsentAt) })
	var n int64
	for _, id := range ids {
		if n >= int64(maxRows) {
			break
		}
		in := r.stored[id]
		in.Locked = false
		r.stored[id] = in
		n++
	}
	return n, nil
}

// visible mirrors the SQL viewer scope: a viewer listed in onlyFor sees only those leads.
func (r *memRepo) visible(scope repository.Scope, id uuid.UUID) bool {
	own, limited := r.onlyFor[scope.ViewerUserID]
	return !limited || own[id]
}

func (r *memRepo) MarkRead(_ context.Context, accountID int64, id uuid.UUID) (bool, error) {
	acc, ok := r.leads[id]
	return ok && acc == accountID, nil
}

func (r *memRepo) Delete(_ context.Context, accountID int64, id uuid.UUID) (bool, error) {
	if acc, ok := r.leads[id]; ok && acc == accountID {
		delete(r.leads, id)
		return true, nil
	}
	return false, nil
}

func (r *memRepo) DeleteVisitorExchange(_ context.Context, id uuid.UUID) (bool, error) {
	if _, ok := r.leads[id]; !ok {
		return false, nil
	}
	delete(r.leads, id)
	r.deleted = append(r.deleted, id)
	return true, nil
}

func (r *memRepo) ProfileName(_ context.Context, id int64, _ string) (string, error) {
	return r.names[id], nil
}

func (r *memRepo) NotifyUserIDs(context.Context, int64) ([]int64, error) { return r.owners, nil }

func (r *memRepo) SenderProfile(_ context.Context, accountID int64, id uuid.UUID) (*repository.SenderProfile, error) {
	if s, ok := r.senders[id]; ok && s.accountID == accountID {
		p := s.profile
		return &p, nil
	}
	return nil, nil
}

type memNotifier struct{ sent []notifmodel.EnqueueInput }

func (n *memNotifier) Enqueue(_ context.Context, in notifmodel.EnqueueInput) ([]notifmodel.Notification, error) {
	n.sent = append(n.sent, in)
	return nil, nil
}

func int64p(v int64) *int64 { return &v }

func TestSubmitRules(t *testing.T) {
	t.Parallel()
	on := []byte(`{"lead_capture":{"enabled":true}}`)
	off := []byte(`{"visible_modules":["bio"]}`)
	routes := map[string]*repository.CaptureRoute{
		"profile-on":  {ID: 1, RouteType: "canonical_profile", Status: "active", AccountID: 7, PresentationConfig: on, ProfileID: int64p(3)},
		"profile-off": {ID: 2, RouteType: "canonical_profile", Status: "active", AccountID: 7, PresentationConfig: off},
		"locked":      {ID: 3, RouteType: "canonical_profile", Status: "locked", AccountID: 7, PresentationConfig: on},
		"s/link":      {ID: 4, RouteType: "custom_smart_link", Status: "active", AccountID: 7, PresentationConfig: on},
		"c/column":    {ID: 5, RouteType: "card_route", Status: "active", AccountID: 7, CardID: int64p(9), CardLeadCapture: true, CardIsActive: true},
		"c/inactive":  {ID: 6, RouteType: "card_route", Status: "active", AccountID: 7, CardID: int64p(10), CardLeadCapture: true},
		"c/off":       {ID: 7, RouteType: "card_route", Status: "active", AccountID: 7, CardID: int64p(11), CardIsActive: true},
		"legacy":      {ID: 8, RouteType: "legacy_route", Status: "active", AccountID: 7, PresentationConfig: on},
	}
	cases := map[string]string{ // path -> wanted source ("" = unavailable)
		"profile-on": model.SourceProfile, "profile-off": "", "locked": "", "s/link": model.SourceSmartLink,
		"c/column": model.SourceCard, "c/inactive": "", "c/off": "", "legacy": "", "unknown": "",
	}
	for path, want := range cases {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			repo := newMemRepo()
			repo.routes = routes
			svc := newTestService(repo, nil)
			in := validInput()
			in.Path = path
			_, err := svc.Submit(context.Background(), in, model.Visitor{})
			if want == "" {
				if !errors.Is(err, ErrFormUnavailable) {
					t.Fatalf("want ErrFormUnavailable, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := repo.created[0].SourceType; got != want {
				t.Fatalf("source: want %s, got %s", want, got)
			}
			cfg, _ := svc.FormConfig(context.Background(), path)
			if !cfg.Enabled || cfg.SourceType != want {
				t.Fatalf("form config disagrees: %+v", cfg)
			}
		})
	}
}

func TestSubmitStoresConsentAndNotifiesOwners(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	repo.routes["ahmet"] = &repository.CaptureRoute{
		ID: 1, RouteType: "canonical_profile", Status: "active", AccountID: 42,
		PresentationConfig: []byte(`{"lead_capture":{"enabled":true}}`), ProfileID: int64p(5),
	}
	repo.owners = []int64{100, 101}
	notifier := &memNotifier{}
	svc := newTestService(repo, notifier)
	fixed := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return fixed }

	res, err := svc.Submit(context.Background(), validInput(), model.Visitor{IP: "203.0.113.9", UserAgent: "UA"})
	if err != nil {
		t.Fatal(err)
	}
	got := repo.created[0]
	if got.AccountID != 42 || got.RouteID != 1 || *got.ProfileID != 5 || got.RoutePath != "ahmet" {
		t.Fatalf("wrong ownership/source: %+v", got)
	}
	if !got.ConsentAt.Equal(fixed) || got.ConsentVersion != model.ConsentVersion || got.IP != "203.0.113.9" {
		t.Fatalf("consent evidence not stored: %+v", got)
	}
	if len(notifier.sent) != 2 || *notifier.sent[0].UserID != 100 || notifier.sent[0].TemplateCode != "leads.received" {
		t.Fatalf("owners not notified: %+v", notifier.sent)
	}
	if !strings.Contains(*notifier.sent[0].ActionURL, res.UUID.String()) {
		t.Fatalf("action url misses the lead: %s", *notifier.sent[0].ActionURL)
	}
}

func TestSubmitInvalidStoresNothing(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	svc := newTestService(repo, nil)
	in := validInput()
	in.Consent = false
	if _, err := svc.Submit(context.Background(), in, model.Visitor{}); fieldCodes(err)["consent"] != "required" {
		t.Fatalf("want consent error, got %v", err)
	}
	if len(repo.created) != 0 {
		t.Fatal("invalid lead was stored")
	}
}

func TestAppAccessIsAccountScoped(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	mine, theirs := uuid.New(), uuid.New()
	repo.leads[mine] = 1
	repo.leads[theirs] = 2
	svc := newTestService(repo, nil)
	ctx := context.Background()
	owner := authctx.Principal{UserInternal: 10, Account: authctx.Account{ID: 1, Role: authctx.AccountRoleOwner}}
	member := authctx.Principal{UserInternal: 11, Account: authctx.Account{ID: 1, Role: authctx.AccountRoleMember}}

	if items, total, _ := svc.List(ctx, owner, model.ListFilter{}); total != 1 || items[0].UUID != mine {
		t.Fatalf("list leaked or missed rows: %v", items)
	}
	if _, err := svc.Get(ctx, owner, theirs); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other account's lead visible: %v", err)
	}
	if _, err := svc.Get(ctx, member, mine); err != nil {
		t.Fatalf("members can read: %v", err)
	}
	if _, err := svc.MarkRead(ctx, member, mine); !errors.Is(err, ErrForbidden) {
		t.Fatalf("member marked read: %v", err)
	}
	if err := svc.Delete(ctx, member, mine); !errors.Is(err, ErrForbidden) {
		t.Fatalf("member deleted: %v", err)
	}
	if err := svc.Delete(ctx, owner, theirs); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted another account's lead: %v", err)
	}
	if _, err := svc.MarkRead(ctx, owner, mine); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, owner, mine); err != nil {
		t.Fatal(err)
	}
}

func TestReadsPassTheViewer(t *testing.T) {
	t.Parallel()
	repo := newMemRepo()
	ownLead, colleagueLead := uuid.New(), uuid.New()
	repo.leads[ownLead], repo.leads[colleagueLead] = 1, 1
	repo.onlyFor = map[int64]map[uuid.UUID]bool{11: {ownLead: true}}
	svc := newTestService(repo, nil)
	ctx := context.Background()
	member := authctx.Principal{UserInternal: 11, Account: authctx.Account{ID: 1, Role: authctx.AccountRoleMember}}

	if items, total, _ := svc.List(ctx, member, model.ListFilter{}); total != 1 || items[0].UUID != ownLead {
		t.Fatalf("member list: %v", items)
	}
	if _, err := svc.Get(ctx, member, colleagueLead); !errors.Is(err, ErrNotFound) {
		t.Fatalf("member read a colleague's lead: %v", err)
	}
}

// newTestService is New with unlimited monthly quotas (TEC-941).
func newTestService(repo repository.Repository, n Notifier) *Service {
	return New(repo, n, nil).SetQuotas(newMemQuotas(time.Now, map[string]int32{}))
}
