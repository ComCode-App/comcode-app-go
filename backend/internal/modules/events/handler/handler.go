package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/domain"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/ingestion"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/registry"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/clientip"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/pkg/response"
)

// Handler handles public event ingestion and event definition administrative endpoints.
// EventLimiter caps public event ingestion per client IP (ratelimit.Limiter).
type EventLimiter interface {
	AllowEvent(ctx context.Context, ip string) (bool, time.Duration)
}

type Handler struct {
	collector *ingestion.Collector
	registry  registry.Store
	limiter   EventLimiter
}

// SetRateLimiter enables per-IP limits on POST /v1/events.
func (h *Handler) SetRateLimiter(l EventLimiter) { h.limiter = l }

// New creates an events Handler.
func New(collector *ingestion.Collector, registry registry.Store) *Handler {
	return &Handler{
		collector: collector,
		registry:  registry,
	}
}

// Ingest handles incoming public event telemetry.
func (h *Handler) Ingest(w http.ResponseWriter, r *http.Request) {
	if h.limiter != nil {
		if ok, retry := h.limiter.AllowEvent(r.Context(), clientip.String(r)); !ok {
			if retry > 0 {
				w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())+1))
			}
			response.TooManyRequests(w, r, "Too many events. Try again later.")
			return
		}
	}
	var req ingestion.IngestRequest
	if err := decodeJSON(w, r, &req); err != nil {
		return
	}

	clientIP := extractIP(r)
	httpCtx := ingestion.HTTPContext{
		ClientIP:  clientIP,
		UserAgent: r.UserAgent(),
		Referer:   r.Referer(),
		RequestID: r.Header.Get("X-Request-ID"),
	}

	event, err := h.collector.Ingest(r.Context(), req, httpCtx, true, "async")
	if err != nil {
		if errors.Is(err, domain.ErrInvalidEventType) ||
			errors.Is(err, domain.ErrInvalidSubject) ||
			errors.Is(err, domain.ErrEventDisabled) ||
			errors.Is(err, domain.ErrEventInternalOnly) ||
			errors.Is(err, domain.ErrMissingProperty) ||
			errors.Is(err, domain.ErrPayloadTooLarge) ||
			errors.Is(err, domain.ErrPayloadTooDeep) ||
			errors.Is(err, domain.ErrFutureTimestamp) ||
			errors.Is(err, domain.ErrDefinitionNotFound) {
			response.BadRequest(w, r, response.CodeValidationError, err.Error())
			return
		}
		response.InternalErr(w, r, err, "failed to ingest event")
		return
	}

	response.JSON(w, r, http.StatusAccepted, map[string]any{
		"uuid":       event.UUID,
		"type":       event.Type,
		"status":     "accepted",
		"created_at": event.CreatedAt,
	})
}

// ListDefinitions returns all registered event definitions.
func (h *Handler) ListDefinitions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var category *string
	if c := q.Get("category"); c != "" {
		category = &c
	}
	var enabled *bool
	if e := q.Get("enabled"); e != "" {
		val := e == "true" || e == "1"
		enabled = &val
	}
	var isPublic *bool
	if p := q.Get("is_public"); p != "" {
		val := p == "true" || p == "1"
		isPublic = &val
	}

	defs, err := h.registry.List(r.Context(), category, enabled, isPublic)
	if err != nil {
		response.InternalErr(w, r, err, "failed to list event definitions")
		return
	}

	response.JSON(w, r, http.StatusOK, map[string]any{
		"items": defs,
		"total": len(defs),
	})
}

// GetDefinition retrieves a single event definition.
func (h *Handler) GetDefinition(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid id")
		return
	}

	def, err := h.registry.GetByID(r.Context(), id)
	if err != nil {
		response.NotFound(w, r, "event definition not found")
		return
	}

	response.JSON(w, r, http.StatusOK, def)
}

// CreateDefinition creates a new event definition.
func (h *Handler) CreateDefinition(w http.ResponseWriter, r *http.Request) {
	var in domain.EventDefinition
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}

	created, err := h.registry.Create(r.Context(), in)
	if errors.Is(err, domain.ErrDefinitionExists) {
		response.Conflict(w, r, response.CodeConflict, "event definition already exists")
		return
	}
	if err != nil {
		response.InternalErr(w, r, err, "failed to create event definition")
		return
	}

	response.JSON(w, r, http.StatusCreated, created)
}

// PatchDefinition updates an existing event definition.
func (h *Handler) PatchDefinition(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid id")
		return
	}

	var in domain.EventDefinitionPatch
	if err := decodeJSON(w, r, &in); err != nil {
		return
	}
	in.ID = id

	updated, err := h.registry.Update(r.Context(), in)
	if err != nil {
		response.InternalErr(w, r, err, "failed to update event definition")
		return
	}

	response.JSON(w, r, http.StatusOK, updated)
}

// DeleteDefinition removes an event definition.
func (h *Handler) DeleteDefinition(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid id")
		return
	}

	if err := h.registry.Delete(r.Context(), id); err != nil {
		response.InternalErr(w, r, err, "failed to delete event definition")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func extractIP(r *http.Request) net.IP {
	return clientip.IP(r)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := dec.Decode(dst); err != nil {
		response.BadRequest(w, r, response.CodeValidationError, "invalid body")
		return err
	}
	return nil
}
