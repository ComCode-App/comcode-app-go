// Package usecase computes company analytics from the existing daily aggregates (TEC-574).
//
// No account-level events or rollups are written: a company's numbers are the sum of the
// event_aggregates_daily rows of its own profiles, routes and cards, attributed to the employee
// profile each subject currently points at. Scope and role are reloaded from the database on
// every request (owner/admin: whole company; office_manager: granted offices; member: none).
package usecase

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyanalytics/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/rbac"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

const day = 24 * time.Hour

// Reader runs fn with the caller's live company role in a read-only snapshot.
type Reader interface {
	Read(ctx context.Context, accountID, userID int64, fn func(*db.Queries, string) error) error
}

// HistoryPolicy is the plan's analytics history window (entitlement.Service implements it).
type HistoryPolicy interface {
	HistoryWindow(ctx context.Context, accountID int64, isSuperAdmin bool) (days int32, unlimited bool, err error)
}

type Service struct {
	repo    Reader
	history HistoryPolicy
	now     func() time.Time
}

func New(repo Reader, history HistoryPolicy) *Service {
	return &Service{repo: repo, history: history, now: time.Now}
}

// SetNow replaces the clock (tests).
func (s *Service) SetNow(now func() time.Time) { s.now = now }

// Today is the current UTC date.
func (s *Service) Today() time.Time { return s.now().UTC().Truncate(day) }

// scope is the resolved query scope shared by every read.
type scope struct {
	accountID, actorID int64
	role               string
	office, group      pgtype.UUID
	all                bool
	from, to           pgtype.Date
}

func canRead(role string) bool {
	return role == authctx.AccountRoleOwner || role == authctx.AccountRoleAdmin || role == authctx.AccountRoleOfficeManager
}

func nullUUID(id *uuid.UUID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *id, Valid: true}
}

func publicUUID(id pgtype.UUID) *uuid.UUID {
	if !id.Valid {
		return nil
	}
	v := uuid.UUID(id.Bytes)
	return &v
}

func date(t time.Time) pgtype.Date { return pgtype.Date{Time: t, Valid: true} }

// window clamps the range to the plan's analytics.history_days, like the per-subject analytics.
func (s *Service) window(ctx context.Context, p authctx.Principal, f model.Filter) (model.Window, error) {
	w := model.Window{From: f.From, To: f.To}
	days, unlimited, err := s.history.HistoryWindow(ctx, p.Account.ID, p.IsSuperAdmin)
	if err != nil {
		return w, err
	}
	if unlimited {
		return w, nil
	}
	w.HistoryDays = &days
	earliest := s.Today().AddDate(0, 0, -int(days))
	if w.From.Before(earliest) {
		w.From = earliest
	}
	if w.To.Before(w.From) {
		w.To = w.From
	}
	return w, nil
}

// read authorizes the caller and runs fn with the resolved scope. Personal accounts, members and
// callers without app.company_analytics.read are forbidden; nothing is ever read unscoped.
func (s *Service) read(ctx context.Context, p authctx.Principal, f model.Filter, fn func(*db.Queries, scope) error) (model.Window, error) {
	if !p.HasPermission(rbac.PermAppCompanyAnalyticsRead) || p.Account.Type != authctx.AccountTypeCompany {
		return model.Window{}, model.ErrForbidden
	}
	w, err := s.window(ctx, p, f)
	if err != nil {
		return w, err
	}
	err = s.repo.Read(ctx, p.Account.ID, p.UserInternal, func(q *db.Queries, role string) error {
		if !canRead(role) {
			return model.ErrForbidden
		}
		sc := scope{
			accountID: p.Account.ID, actorID: p.UserInternal, role: role,
			office: nullUUID(f.OfficeUUID), group: nullUUID(f.GroupUUID),
			from: date(w.From), to: date(w.To),
		}
		// Subjects without an employee profile (e.g. a smart link to a URL) belong to the whole
		// company only: they are counted for owners/admins when no office/group filter is set.
		sc.all = role != authctx.AccountRoleOfficeManager && f.OfficeUUID == nil && f.GroupUUID == nil
		return fn(q, sc)
	})
	return w, err
}

type rows struct {
	byProfile []db.CompanyAnalyticsByProfileRow
	leads     []db.CompanyAnalyticsLeadsRow
}

func (sc scope) byProfile(ctx context.Context, q *db.Queries) (rows, error) {
	var out rows
	var err error
	out.byProfile, err = q.CompanyAnalyticsByProfile(ctx, db.CompanyAnalyticsByProfileParams{
		AccountID: sc.accountID, ActorID: sc.actorID, ActorRole: sc.role, OfficeUuid: sc.office,
		GroupUuid: sc.group, AllScope: sc.all, FromDate: sc.from, ToDate: sc.to,
	})
	if err != nil {
		return out, err
	}
	out.leads, err = q.CompanyAnalyticsLeads(ctx, db.CompanyAnalyticsLeadsParams{
		AccountID: sc.accountID, ActorID: sc.actorID, ActorRole: sc.role, OfficeUuid: sc.office,
		GroupUuid: sc.group, AllScope: sc.all, FromDate: sc.from, ToDate: sc.to,
	})
	return out, err
}

func (sc scope) profiles(ctx context.Context, q *db.Queries, search string) ([]db.CompanyAnalyticsProfilesRow, error) {
	return q.CompanyAnalyticsProfiles(ctx, db.CompanyAnalyticsProfilesParams{
		AccountID: sc.accountID, ActorID: sc.actorID, ActorRole: sc.role,
		OfficeUuid: sc.office, GroupUuid: sc.group, Q: search,
	})
}

// perProfile folds the rows into metrics per profile id (0 = not attributed) and the scope total.
func (r rows) perProfile() (map[int64]*model.Metrics, model.Metrics, map[string]int64) {
	out := map[int64]*model.Metrics{}
	var total model.Metrics
	events := map[string]int64{}
	get := func(id int64) *model.Metrics {
		if m, ok := out[id]; ok {
			return m
		}
		m := &model.Metrics{}
		out[id] = m
		return m
	}
	for _, row := range r.byProfile {
		get(row.ProfileID).AddEvent(row.SubjectType, row.RouteType, row.EventType, row.Total)
		total.AddEvent(row.SubjectType, row.RouteType, row.EventType, row.Total)
		events[row.EventType] += row.Total
	}
	for _, row := range r.leads {
		get(row.ProfileID).Leads += row.Total
		total.Leads += row.Total
	}
	return out, total, events
}

func iso(t time.Time) string { return t.Format("2006-01-02") }

// Summary returns the totals of the scope, its top event types and how many profiles are active.
func (s *Service) Summary(ctx context.Context, p authctx.Principal, f model.Filter) (*model.Summary, error) {
	out := &model.Summary{TopEvents: []model.EventCount{}}
	w, err := s.read(ctx, p, f, func(q *db.Queries, sc scope) error {
		r, err := sc.byProfile(ctx, q)
		if err != nil {
			return err
		}
		perProfile, total, events := r.perProfile()
		out.Totals = total
		for k, v := range events {
			out.TopEvents = append(out.TopEvents, model.EventCount{EventType: k, Count: v})
		}
		sort.Slice(out.TopEvents, func(i, j int) bool {
			a, b := out.TopEvents[i], out.TopEvents[j]
			return a.Count > b.Count || (a.Count == b.Count && a.EventType < b.EventType)
		})
		if len(out.TopEvents) > 10 {
			out.TopEvents = out.TopEvents[:10]
		}
		profiles, err := sc.profiles(ctx, q, "")
		if err != nil {
			return err
		}
		out.Profiles = int64(len(profiles))
		for _, pr := range profiles {
			if m := perProfile[pr.ID]; m != nil && (m.TotalEvents > 0 || m.Leads > 0) {
				out.ActiveProfiles++
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	out.From, out.To, out.HistoryDays = iso(w.From), iso(w.To), w.HistoryDays
	return out, nil
}

// TimeSeries returns one point per UTC day of the range; days without events are zero.
func (s *Service) TimeSeries(ctx context.Context, p authctx.Principal, f model.Filter) (*model.TimeSeries, error) {
	byDay := map[string]*model.Metrics{}
	get := func(t time.Time) *model.Metrics {
		k := iso(t.UTC())
		if m, ok := byDay[k]; ok {
			return m
		}
		m := &model.Metrics{}
		byDay[k] = m
		return m
	}
	w, err := s.read(ctx, p, f, func(q *db.Queries, sc scope) error {
		daily, err := q.CompanyAnalyticsDaily(ctx, db.CompanyAnalyticsDailyParams{
			AccountID: sc.accountID, ActorID: sc.actorID, ActorRole: sc.role, OfficeUuid: sc.office,
			GroupUuid: sc.group, AllScope: sc.all, FromDate: sc.from, ToDate: sc.to,
		})
		if err != nil {
			return err
		}
		for _, row := range daily {
			get(row.BucketDate.Time).AddEvent(row.SubjectType, row.RouteType, row.EventType, row.Total)
		}
		leads, err := q.CompanyAnalyticsLeads(ctx, db.CompanyAnalyticsLeadsParams{
			AccountID: sc.accountID, ActorID: sc.actorID, ActorRole: sc.role, OfficeUuid: sc.office,
			GroupUuid: sc.group, AllScope: sc.all, FromDate: sc.from, ToDate: sc.to,
		})
		if err != nil {
			return err
		}
		for _, row := range leads {
			get(row.Day.Time).Leads += row.Total
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	out := &model.TimeSeries{From: iso(w.From), To: iso(w.To), HistoryDays: w.HistoryDays, Points: []model.Point{}}
	for d := w.From; !d.After(w.To); d = d.Add(day) {
		pt := model.Point{Date: iso(d)}
		if m := byDay[pt.Date]; m != nil {
			pt.Metrics = *m
		}
		out.Points = append(out.Points, pt)
	}
	return out, nil
}

// Breakdown returns the top values of a visitor dimension over the scope.
func (s *Service) Breakdown(ctx context.Context, p authctx.Principal, f model.Filter) (*model.Breakdown, error) {
	out := &model.Breakdown{Dimension: f.Dimension, Items: []model.BreakdownItem{}}
	_, err := s.read(ctx, p, f, func(q *db.Queries, sc scope) error {
		var eventType pgtype.Text
		if f.EventType != "" {
			eventType = pgtype.Text{String: f.EventType, Valid: true}
		}
		rows, err := q.CompanyAnalyticsBreakdown(ctx, db.CompanyAnalyticsBreakdownParams{
			AccountID: sc.accountID, ActorID: sc.actorID, ActorRole: sc.role, OfficeUuid: sc.office,
			GroupUuid: sc.group, AllScope: sc.all, FromDate: sc.from, ToDate: sc.to,
			Dimension: f.Dimension, EventType: eventType, LimitCount: f.TopN,
		})
		if err != nil {
			return err
		}
		for _, row := range rows {
			out.Total = row.GrandTotal
			pct := 0.0
			if row.GrandTotal > 0 {
				pct = float64(row.Count) / float64(row.GrandTotal) * 100
			}
			out.Items = append(out.Items, model.BreakdownItem{Key: row.Key, Count: row.Count, Percentage: pct})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Employees returns the per-profile comparison, sorted and paginated; share_percent is the
// profile's part of the scope's total events.
func (s *Service) Employees(ctx context.Context, p authctx.Principal, f model.Filter) ([]model.Employee, int64, error) {
	var items []model.Employee
	_, err := s.read(ctx, p, f, func(q *db.Queries, sc scope) error {
		r, err := sc.byProfile(ctx, q)
		if err != nil {
			return err
		}
		perProfile, total, _ := r.perProfile()
		profiles, err := sc.profiles(ctx, q, strings.TrimSpace(f.Q))
		if err != nil {
			return err
		}
		items = make([]model.Employee, 0, len(profiles))
		for _, pr := range profiles {
			e := model.Employee{
				ProfileUUID: pr.Uuid, ProfileName: pr.DisplayName, Title: pr.Title,
				MemberUUID: publicUUID(pr.MemberUuid), MemberName: pr.MemberName, MemberEmail: pr.MemberEmail,
				OfficeUUID: publicUUID(pr.OfficeUuid), OfficeName: pr.OfficeName,
				GroupUUID: publicUUID(pr.GroupUuid), GroupName: pr.GroupName,
			}
			if m := perProfile[pr.ID]; m != nil {
				e.Metrics = *m
			}
			if total.TotalEvents > 0 {
				e.SharePercent = float64(e.Metrics.TotalEvents) / float64(total.TotalEvents) * 100
			}
			items = append(items, e)
		}
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	sortEmployees(items, f.Sort)
	n := int64(len(items))
	start := min(int(f.Offset), len(items))
	end := min(start+int(f.Limit), len(items))
	return items[start:end], n, nil
}

func label(e model.Employee) string {
	if e.ProfileName != "" {
		return strings.ToLower(e.ProfileName)
	}
	return strings.ToLower(e.MemberName)
}

func sortEmployees(items []model.Employee, sortKey string) {
	desc := strings.HasPrefix(sortKey, "-")
	field := strings.TrimPrefix(sortKey, "-")
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if field != "name" {
			va, vb := a.Metrics.SortValue(field), b.Metrics.SortValue(field)
			if va != vb {
				if desc {
					return va > vb
				}
				return va < vb
			}
			// Ties: alphabetical, independent of the direction.
			return label(a) < label(b) || (label(a) == label(b) && a.ProfileUUID.String() < b.ProfileUUID.String())
		}
		la, lb := label(a), label(b)
		if la != lb {
			if desc {
				return la > lb
			}
			return la < lb
		}
		return a.ProfileUUID.String() < b.ProfileUUID.String()
	})
}
