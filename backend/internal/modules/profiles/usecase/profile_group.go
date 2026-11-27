package usecase

import (
	"context"
	"errors"
	"strings"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/google/uuid"
)

// ErrOpenLocked means the company keeps employee profiles open, so the switch cannot close one.
var ErrOpenLocked = errors.New("the company keeps employee profiles open")

// SetProfileGroups wires the open switch and the swipe group of public pages (TEC-1326).
func (s *Service) SetProfileGroups(store repository.ProfileGroupStore) { s.groups = store }

// markCanToggleOpen sets CanToggleOpen on a profile list: owners and admins may flip every
// profile of the account, any other member only their own employee profile.
func (s *Service) markCanToggleOpen(ctx context.Context, p authctx.Principal, items []model.ProfileSummary) error {
	if s.groups == nil {
		return nil
	}
	if p.Account.CanWrite() {
		for i := range items {
			items[i].CanToggleOpen = true
		}
		return nil
	}
	own, err := s.groups.OwnMemberProfile(ctx, p.Account.ID, p.UserInternal)
	if err != nil || own == uuid.Nil {
		return err
	}
	for i := range items {
		items[i].CanToggleOpen = items[i].UUID == own
	}
	return nil
}

func openStateOf(p authctx.Principal, st *repository.OpenState) *model.OpenState {
	locked := st.MemberProfile && st.AlwaysOpen
	return &model.OpenState{
		IsOpen:        st.IsOpen || locked,
		OpenLocked:    locked,
		CanToggleOpen: p.Account.CanWrite() || st.OwnMemberProfile,
	}
}

// OpenState returns the open switch of a profile in the active account.
func (s *Service) OpenState(ctx context.Context, p authctx.Principal, profileUUID uuid.UUID) (*model.OpenState, error) {
	if s.groups == nil {
		return nil, ErrPublicPagesUnavailable
	}
	st, err := s.groups.OpenState(ctx, p.Account.ID, p.UserInternal, profileUUID)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	return openStateOf(p, st), nil
}

// SetOpen turns a profile's open switch on or off. Owners and admins may change any profile of
// the account; another member only their own employee profile. While the company keeps employee
// profiles open, an employee profile cannot be closed.
func (s *Service) SetOpen(ctx context.Context, p authctx.Principal, profileUUID uuid.UUID, in model.SetOpenInput) (*model.OpenState, error) {
	if s.groups == nil {
		return nil, ErrPublicPagesUnavailable
	}
	if in.IsOpen == nil {
		return nil, errors.Join(ErrInvalidRequest, errors.New("is_open is required"))
	}
	st, err := s.groups.OpenState(ctx, p.Account.ID, p.UserInternal, profileUUID)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	if !p.Account.CanWrite() && !st.OwnMemberProfile {
		return nil, ErrForbidden
	}
	if !*in.IsOpen && st.MemberProfile && st.AlwaysOpen {
		return nil, ErrOpenLocked
	}
	if st.IsOpen != *in.IsOpen {
		if err := s.groups.SetOpen(ctx, p.Account.ID, profileUUID, *in.IsOpen); err != nil {
			return nil, mapRepoErr(err)
		}
		s.routeCache.InvalidateProfile(ctx, profileUUID)
		s.activity.Record(ctx, &p.UserInternal, "profile.open_changed", "profiles", &profileUUID, map[string]any{
			"before": map[string]any{"is_open": st.IsOpen},
			"after":  map[string]any{"is_open": *in.IsOpen},
		}, nil)
		st.IsOpen = *in.IsOpen
	}
	return openStateOf(p, st), nil
}

// shownProfile is the profile a visitor sees for a resolved public route.
type shownProfile struct {
	route    *repository.PublicRoute
	detail   *model.ProfileDetail
	siblings []model.PublicSibling
}

// swipeTarget applies the open switch to a resolved public route (TEC-1326). An open profile is
// shown with the person's other open profiles as siblings. A closed one is replaced by the
// person's first open profile, so none of its data reaches the visitor. Nil: nothing is open.
func (s *Service) swipeTarget(ctx context.Context, route *repository.PublicRoute, detail *model.ProfileDetail) (*shownProfile, error) {
	if s.groups == nil {
		return &shownProfile{route: route, detail: detail}, nil
	}
	group, err := s.groups.ProfileGroup(ctx, route.ProfileID)
	if err != nil {
		return nil, err
	}
	targetOpen, inGroup := detail.IsOpen, false
	open := make([]repository.GroupProfile, 0, len(group))
	for _, g := range group {
		if g.ProfileID == route.ProfileID {
			targetOpen, inGroup = g.Open, true
		}
		if g.Open {
			open = append(open, g)
		}
	}
	if !inGroup {
		// A profile without a known person (a company profile no member is linked to) is shown
		// on its own.
		if !targetOpen {
			return nil, nil
		}
		return &shownProfile{route: route, detail: detail}, nil
	}
	shown := &shownProfile{route: route, detail: detail}
	if !targetOpen {
		if len(open) == 0 {
			return nil, nil
		}
		first := open[0]
		next, err := s.pages.RouteByPath(ctx, first.Username)
		if err != nil {
			return nil, mapRepoErr(err)
		}
		nextDetail, err := s.repo.GetByUUID(ctx, next.AccountID, next.ProfileUUID)
		if err != nil {
			return nil, mapRepoErr(err)
		}
		shown = &shownProfile{route: next, detail: nextDetail}
	}
	if len(open) > 1 {
		shown.siblings = make([]model.PublicSibling, 0, len(open))
		for _, g := range open {
			shown.siblings = append(shown.siblings, model.PublicSibling{
				Username: g.Username, ProfileUUID: g.ProfileUUID, ShortName: shortName(g),
			})
		}
	}
	return shown, nil
}

// shortName labels a profile in the swipe: its company name, else the person's name, else the
// username.
func shortName(g repository.GroupProfile) string {
	if n := strings.TrimSpace(g.CompanyName); n != "" {
		return n
	}
	if n := strings.TrimSpace(strings.TrimSpace(g.FirstName) + " " + strings.TrimSpace(g.LastName)); n != "" {
		return n
	}
	return g.Username
}
