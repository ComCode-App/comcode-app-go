package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/application"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/domain"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
)

// Handler handles analytics query endpoints and configuration.
type Handler struct {
	svc    *application.AnalyticsService
	access Access
}

// New creates an analytics Handler. access scopes the account analytics queries.
func New(svc *application.AnalyticsService, access Access) *Handler {
	return &Handler{svc: svc, access: access}
}

// scopedFilter parses the query and applies account scoping; false means a response was written.
func (h *Handler) scopedFilter(w http.ResponseWriter, r *http.Request) (domain.AnalyticsFilter, bool) {
	filter, err := parseFilter(r)
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, err.Error())
		return filter, false
	}
	switch err := h.access.authorize(w, r, &filter); {
	case err == nil:
		return filter, true
	case errors.Is(err, errSubjectRequired):
		response.BadRequest(w, r, response.CodeValidationError, err.Error())
	case errors.Is(err, errSubjectNotFound):
		response.NotFound(w, r, err.Error())
	default:
		response.InternalErr(w, r, err, "failed to authorize analytics query")
	}
	return filter, false
}

// Summary returns high-level metric summaries for a subject.
func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	filter, ok := h.scopedFilter(w, r)
	if !ok {
		return
	}

	res, err := h.svc.GetSummary(r.Context(), filter)
	if err != nil {
		response.InternalErr(w, r, err, "failed to get analytics summary")
		return
	}

	response.JSON(w, r, http.StatusOK, res)
}

// Sources returns the views, QR scans and entry source breakdown (QR, NFC, link, legacy) of a
// subject together with the routes and cards that lead to it (TEC-953).
func (h *Handler) Sources(w http.ResponseWriter, r *http.Request) {
	filter, ok := h.scopedFilter(w, r)
	if !ok {
		return
	}
	if filter.SubjectType == "" || filter.SubjectID == "" {
		response.BadRequest(w, r, response.CodeValidationError, errSubjectRequired.Error())
		return
	}
	res, err := h.svc.GetSources(r.Context(), filter)
	if err != nil {
		response.InternalErr(w, r, err, "failed to get analytics sources")
		return
	}
	response.JSON(w, r, http.StatusOK, res)
}

// TimeSeries returns timeline data points.
func (h *Handler) TimeSeries(w http.ResponseWriter, r *http.Request) {
	filter, ok := h.scopedFilter(w, r)
	if !ok {
		return
	}

	interval := r.URL.Query().Get("interval")
	if interval != "" {
		filter.Interval = interval
	}

	res, err := h.svc.GetTimeSeries(r.Context(), filter)
	if err != nil {
		response.InternalErr(w, r, err, "failed to get time series")
		return
	}

	response.JSON(w, r, http.StatusOK, res)
}

// Breakdown returns distribution by dimension (country, city, device, browser, os, referer).
func (h *Handler) Breakdown(w http.ResponseWriter, r *http.Request) {
	filter, ok := h.scopedFilter(w, r)
	if !ok {
		return
	}

	dimension := r.URL.Query().Get("dimension")
	if dimension == "" {
		dimension = "country"
	}

	res, err := h.svc.GetBreakdown(r.Context(), filter, dimension)
	if err != nil {
		response.InternalErr(w, r, err, "failed to get breakdown")
		return
	}

	response.JSON(w, r, http.StatusOK, res)
}

// Events returns bounded paginated raw events.
func (h *Handler) Events(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	var subType, subID, evType, country *string
	if st := q.Get("subject_type"); st != "" {
		subType = &st
	}
	if si := q.Get("subject_id"); si != "" {
		subID = &si
	}
	if et := q.Get("event_type"); et != "" {
		evType = &et
	}
	if c := q.Get("country"); c != "" {
		country = &c
	}

	var from, to *time.Time
	if f := q.Get("from"); f != "" {
		if t, err := time.Parse(time.RFC3339, f); err == nil {
			from = &t
		}
	}
	if tStr := q.Get("to"); tStr != "" {
		if t, err := time.Parse(time.RFC3339, tStr); err == nil {
			to = &t
		}
	}

	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))

	filter := domain.RawEventFilter{
		SubjectType: subType,
		SubjectID:   subID,
		EventType:   evType,
		CountryCode: country,
		DateFrom:    from,
		DateTo:      to,
		Limit:       limit,
		Offset:      offset,
	}

	res, err := h.svc.GetRawEvents(r.Context(), filter)
	if err != nil {
		response.InternalErr(w, r, err, "failed to list events")
		return
	}

	response.JSON(w, r, http.StatusOK, res)
}

// PlatformOverview returns the platform-wide analytics KPIs from the aggregate tables (TEC-859).
// tz is the viewer's IANA time zone; days are its local calendar days (default UTC).
func (h *Handler) PlatformOverview(w http.ResponseWriter, r *http.Request) {
	loc, err := application.ParseTimeZone(r.URL.Query().Get("tz"))
	if err != nil {
		response.ValidationError(w, r, []response.Detail{{Field: "tz", Message: err.Error(), Code: "invalid"}})
		return
	}
	res, err := h.svc.GetPlatformOverview(r.Context(), loc, time.Now())
	if err != nil {
		response.InternalErr(w, r, err, "failed to get platform analytics overview")
		return
	}
	response.JSON(w, r, http.StatusOK, res)
}

// GetConfig returns current analytics subsystem settings.
func (h *Handler) GetConfig(w http.ResponseWriter, r *http.Request) {
	st, err := h.svc.GetSettings(r.Context())
	if err != nil {
		response.InternalErr(w, r, err, "failed to get analytics config")
		return
	}
	response.JSON(w, r, http.StatusOK, st)
}

// PatchConfig updates analytics subsystem settings.
func (h *Handler) PatchConfig(w http.ResponseWriter, r *http.Request) {
	var in application.AnalyticsSettings
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}

	updated, err := h.svc.UpdateSettings(r.Context(), in)
	if errors.Is(err, application.ErrInvalidSettings) {
		response.ValidationError(w, r, []response.Detail{{Field: "ip_hash_salt_rotation_days", Message: err.Error(), Code: "out_of_range"}})
		return
	}
	if err != nil {
		response.InternalErr(w, r, err, "failed to update analytics config")
		return
	}

	response.JSON(w, r, http.StatusOK, updated)
}

func parseFilter(r *http.Request) (domain.AnalyticsFilter, error) {
	q := r.URL.Query()
	subType := q.Get("subject_type")
	subID := q.Get("subject_id")

	var evType *string
	if et := q.Get("event_type"); et != "" {
		evType = &et
	}

	now := time.Now().UTC()
	from := now.AddDate(0, 0, -30)
	to := now

	if f := q.Get("from"); f != "" {
		if parsed, err := time.Parse(time.RFC3339, f); err == nil {
			from = parsed.UTC()
		} else if parsedDate, err := time.Parse("2006-01-02", f); err == nil {
			from = parsedDate.UTC()
		}
	}
	if tStr := q.Get("to"); tStr != "" {
		if parsed, err := time.Parse(time.RFC3339, tStr); err == nil {
			to = parsed.UTC()
		} else if parsedDate, err := time.Parse("2006-01-02", tStr); err == nil {
			to = time.Date(parsedDate.Year(), parsedDate.Month(), parsedDate.Day(), 23, 59, 59, 999999999, time.UTC)
		}
	}

	return domain.AnalyticsFilter{
		SubjectType: subType,
		SubjectID:   subID,
		EventType:   evType,
		DateFrom:    from,
		DateTo:      to,
	}, nil
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := dec.Decode(dst); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid body")
		return err
	}
	return nil
}
