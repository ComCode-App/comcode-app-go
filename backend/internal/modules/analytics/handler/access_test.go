package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/domain"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/google/uuid"
)

type ownedSet map[uuid.UUID]int64 // subject -> account

func (o ownedSet) AccountOwnsSubject(_ context.Context, accountID int64, _ string, id uuid.UUID) (bool, error) {
	return o[id] == accountID, nil
}

type fixedHistory struct {
	days      int32
	unlimited bool
}

func (h fixedHistory) HistoryWindow(context.Context, int64, bool) (int32, bool, error) {
	return h.days, h.unlimited, nil
}

func TestAnalyticsAccess(t *testing.T) {
	now := time.Date(2026, 10, 11, 15, 0, 0, 0, time.UTC)
	mine, theirs := uuid.New(), uuid.New()
	access := Access{
		Ownership: ownedSet{mine: 1, theirs: 2},
		History:   fixedHistory{days: 30},
		Now:       func() time.Time { return now },
	}
	app := authctx.Principal{Account: authctx.Account{ID: 1}}
	platform := authctx.Principal{Account: authctx.Account{ID: 1}, Permissions: []string{rbac.PermPlatformAnalyticsRead}}
	longAgo := now.AddDate(-1, 0, 0)

	cases := []struct {
		name      string
		p         authctx.Principal
		subType   string
		subID     string
		wantErr   error
		wantFrom  time.Time
		wantLimit string
	}{
		{"own subject is clamped to the plan window", app, SubjectProfile, mine.String(), nil, time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC), "30"},
		{"other account's subject is not found", app, SubjectProfile, theirs.String(), errSubjectNotFound, time.Time{}, ""},
		{"unknown subject type is not found", app, "user", mine.String(), errSubjectNotFound, time.Time{}, ""},
		{"non-uuid subject is not found", app, SubjectCard, "42", errSubjectNotFound, time.Time{}, ""},
		{"subject is required", app, "", "", errSubjectRequired, time.Time{}, ""},
		{"platform reader sees any subject and full history", platform, SubjectProfile, theirs.String(), nil, longAgo, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r = r.WithContext(authctx.WithPrincipal(r.Context(), tc.p))
			w := httptest.NewRecorder()
			f := domain.AnalyticsFilter{SubjectType: tc.subType, SubjectID: tc.subID, DateFrom: longAgo, DateTo: now}
			err := access.authorize(w, r, &f)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if err != nil {
				return
			}
			if !f.DateFrom.Equal(tc.wantFrom) {
				t.Errorf("from = %s, want %s", f.DateFrom, tc.wantFrom)
			}
			if got := w.Header().Get(HistoryHeader); got != tc.wantLimit {
				t.Errorf("%s = %q, want %q", HistoryHeader, got, tc.wantLimit)
			}
		})
	}
}

func TestAnalyticsAccessUnconfiguredDenies(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = r.WithContext(authctx.WithPrincipal(r.Context(), authctx.Principal{Account: authctx.Account{ID: 1}}))
	f := domain.AnalyticsFilter{SubjectType: SubjectProfile, SubjectID: uuid.NewString()}
	if err := (Access{}).authorize(httptest.NewRecorder(), r, &f); err == nil {
		t.Fatal("unconfigured access allowed the query")
	}
}
