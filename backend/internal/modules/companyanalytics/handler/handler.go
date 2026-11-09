// Package handler exposes company analytics through the standard API envelope.
package handler

import (
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyanalytics/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/companyanalytics/usecase"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/authctx"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/apiquery"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
	"github.com/google/uuid"
)

// HistoryHeader tells the client how many days back its plan may query (absent = unlimited).
const HistoryHeader = "X-Analytics-History-Days"

const (
	defaultDays = 30
	// maxDays bounds one request (a year plus a leap day).
	maxDays     = 366
	defaultTopN = 10
	maxTopN     = 50
)

var eventTypePattern = regexp.MustCompile(`^[a-z0-9._-]{1,100}$`)

type Handler struct{ svc *usecase.Service }

func New(svc *usecase.Service) *Handler { return &Handler{svc: svc} }

type validation []apiquery.Detail

func (v *validation) add(field, msg, code string) {
	*v = append(*v, apiquery.Detail{Field: field, Message: msg, Code: code})
}

func parseDate(raw string) (time.Time, bool) {
	if t, err := time.Parse("2006-01-02", raw); err == nil {
		return t.UTC(), true
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.UTC().Truncate(24 * time.Hour), true
	}
	return time.Time{}, false
}

// ParseFilter reads the shared query: from/to (YYYY-MM-DD or RFC 3339, inclusive UTC days,
// default the last 30 days ending today), office_uuid, group_uuid and the per-endpoint params.
func ParseFilter(v url.Values, today time.Time, employees, breakdown bool) (model.Filter, error) {
	var errs validation
	if bad := apiquery.ForbiddenParams(v); len(bad) > 0 {
		for _, k := range bad {
			errs.add(k, "Unsupported query parameter; use limit, offset and q", "forbidden")
		}
	}
	f := model.Filter{To: today, From: today.AddDate(0, 0, -(defaultDays - 1))}
	if raw := strings.TrimSpace(v.Get("to")); raw != "" {
		if t, ok := parseDate(raw); ok {
			f.To = t
		} else {
			errs.add("to", "Expected a date (YYYY-MM-DD)", "invalid")
		}
	}
	if raw := strings.TrimSpace(v.Get("from")); raw != "" {
		if t, ok := parseDate(raw); ok {
			f.From = t
		} else {
			errs.add("from", "Expected a date (YYYY-MM-DD)", "invalid")
		}
	} else if v.Get("to") != "" {
		f.From = f.To.AddDate(0, 0, -(defaultDays - 1))
	}
	if f.From.After(f.To) {
		errs.add("from", "from must not be after to", "invalid")
	} else if int(f.To.Sub(f.From).Hours()/24)+1 > maxDays {
		errs.add("from", "The range may span at most 366 days", "invalid")
	}
	for _, field := range []struct {
		name string
		dest **uuid.UUID
	}{{"office_uuid", &f.OfficeUUID}, {"group_uuid", &f.GroupUUID}} {
		if raw := strings.TrimSpace(v.Get(field.name)); raw != "" {
			id, err := uuid.Parse(raw)
			if err != nil || id == uuid.Nil {
				errs.add(field.name, "Expected a UUID", "invalid")
				continue
			}
			*field.dest = &id
		}
	}
	if employees {
		q := apiquery.Parse(v)
		f.Q, f.Limit, f.Offset = q.Q, q.Limit, q.Offset
		f.Sort = strings.TrimSpace(v.Get("sort"))
		if f.Sort == "" {
			f.Sort = "-total_events"
		}
		if !slices.Contains(model.EmployeeSorts, strings.TrimPrefix(f.Sort, "-")) {
			errs.add("sort", "Expected one of "+strings.Join(model.EmployeeSorts, ", ")+" (prefix - for descending)", "unknown")
		}
	}
	if breakdown {
		f.Dimension = strings.TrimSpace(v.Get("dimension"))
		if f.Dimension == "" {
			f.Dimension = "country"
		}
		if !slices.Contains(model.Dimensions, f.Dimension) {
			errs.add("dimension", "Expected one of "+strings.Join(model.Dimensions, ", "), "unknown")
		}
		f.EventType = strings.TrimSpace(v.Get("event_type"))
		if f.EventType != "" && !eventTypePattern.MatchString(f.EventType) {
			errs.add("event_type", "Invalid event type", "invalid")
		}
		f.TopN = defaultTopN
		if raw := strings.TrimSpace(v.Get("limit")); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n < 1 || n > maxTopN {
				errs.add("limit", "Expected 1-50", "invalid")
			} else {
				f.TopN = int32(n)
			}
		}
	}
	if len(errs) > 0 {
		return f, &apiquery.ValidationError{Details: errs}
	}
	return f, nil
}

func write(w http.ResponseWriter, r *http.Request, out any, err error) {
	var invalid *apiquery.ValidationError
	switch {
	case err == nil:
		response.JSON(w, r, http.StatusOK, out)
	case errors.As(err, &invalid):
		details := make([]response.Detail, 0, len(invalid.Details))
		for _, d := range invalid.Details {
			details = append(details, response.Detail{Field: d.Field, Message: d.Message, Code: d.Code})
		}
		response.ValidationError(w, r, details)
	case errors.Is(err, model.ErrForbidden):
		response.Forbidden(w, r, "Company analytics is available to company owners, admins and office managers")
	default:
		response.InternalErr(w, r, err, "company analytics query failed")
	}
}

func historyHeader(w http.ResponseWriter, days *int32) {
	if days != nil {
		w.Header().Set(HistoryHeader, strconv.Itoa(int(*days)))
	}
}

func (h *Handler) filter(w http.ResponseWriter, r *http.Request, employees, breakdown bool) (model.Filter, bool) {
	f, err := ParseFilter(r.URL.Query(), h.svc.Today(), employees, breakdown)
	if err != nil {
		write(w, r, nil, err)
		return f, false
	}
	return f, true
}

// Summary: GET /v1/company/analytics/summary
func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	f, ok := h.filter(w, r, false, false)
	if !ok {
		return
	}
	out, err := h.svc.Summary(r.Context(), authctx.MustPrincipal(r.Context()), f)
	if err == nil {
		historyHeader(w, out.HistoryDays)
	}
	write(w, r, out, err)
}

// TimeSeries: GET /v1/company/analytics/timeseries
func (h *Handler) TimeSeries(w http.ResponseWriter, r *http.Request) {
	f, ok := h.filter(w, r, false, false)
	if !ok {
		return
	}
	out, err := h.svc.TimeSeries(r.Context(), authctx.MustPrincipal(r.Context()), f)
	if err == nil {
		historyHeader(w, out.HistoryDays)
	}
	write(w, r, out, err)
}

// Breakdown: GET /v1/company/analytics/breakdown
func (h *Handler) Breakdown(w http.ResponseWriter, r *http.Request) {
	f, ok := h.filter(w, r, false, true)
	if !ok {
		return
	}
	out, err := h.svc.Breakdown(r.Context(), authctx.MustPrincipal(r.Context()), f)
	write(w, r, out, err)
}

// Employees: GET /v1/company/analytics/employees
func (h *Handler) Employees(w http.ResponseWriter, r *http.Request) {
	f, ok := h.filter(w, r, true, false)
	if !ok {
		return
	}
	items, total, err := h.svc.Employees(r.Context(), authctx.MustPrincipal(r.Context()), f)
	write(w, r, apiquery.NewPage(items, total, f.Limit, f.Offset), err)
}
