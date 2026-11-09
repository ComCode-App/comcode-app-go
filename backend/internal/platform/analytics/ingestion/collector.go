package ingestion

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/device"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/domain"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/privacy"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/geoip"
	"github.com/google/uuid"
)

// IngestRequest represents the payload accepted by the event ingestion API.
type IngestRequest struct {
	Type          string                  `json:"type"`
	SchemaVersion int                     `json:"schema_version,omitempty"`
	Subject       domain.EntityReference  `json:"subject"`
	Actor         *domain.EntityReference `json:"actor,omitempty"`
	SessionID     string                  `json:"session_id,omitempty"`
	OccurredAt    *time.Time              `json:"occurred_at,omitempty"`
	Properties    map[string]any          `json:"properties,omitempty"`
	Metadata      map[string]any          `json:"metadata,omitempty"`
}

// HTTPContext holds server-extracted connection metadata.
type HTTPContext struct {
	Source    string // Explicit public identity scan marker, forwarded by the BFF.
	ClientIP  net.IP
	UserAgent string
	Referer   string
	RequestID string
}

// QueuePublisher enqueues raw events for asynchronous background worker processing.
type QueuePublisher interface {
	PublishEvent(ctx context.Context, event domain.Event) error
}

// DirectEventStore immediately writes raw events and aggregates when sync/hybrid mode is active.
type DirectEventStore interface {
	StoreEvent(ctx context.Context, event domain.Event) error
}

// Collector processes incoming event submissions, creates envelopes, and publishes to queue or storage.
type Collector struct {
	validator   domain.EventValidator
	geoip       geoip.GeoIPResolver
	hasher      privacy.Hasher
	uaParser    device.UserAgentParser
	queue       QueuePublisher
	directStore DirectEventStore
	log         *slog.Logger
}

// NewCollector creates an event collector.
func NewCollector(
	validator domain.EventValidator,
	geo geoip.GeoIPResolver,
	hasher privacy.Hasher,
	uaParser device.UserAgentParser,
	queue QueuePublisher,
	directStore DirectEventStore,
	log *slog.Logger,
) *Collector {
	if log == nil {
		log = slog.Default()
	}
	if uaParser == nil {
		uaParser = device.NewParser()
	}
	if hasher == nil {
		hasher = privacy.NewRotatingSaltHasher("default-salt", privacy.DefaultRotationDays)
	}

	return &Collector{
		validator:   validator,
		geoip:       geo,
		hasher:      hasher,
		uaParser:    uaParser,
		queue:       queue,
		directStore: directStore,
		log:         log,
	}
}

// Ingest processes a client-submitted or server-internal event.
func (c *Collector) Ingest(ctx context.Context, req IngestRequest, httpCtx HTTPContext, isPublic bool, deliveryMode string) (*domain.Event, error) {
	now := time.Now().UTC()
	occurredAt := now
	if req.OccurredAt != nil && !req.OccurredAt.IsZero() {
		occurredAt = req.OccurredAt.UTC()
	}

	props := req.Properties
	if props == nil {
		props = make(map[string]any)
	}

	event := &domain.Event{
		UUID:          uuid.NewString(),
		Type:          strings.TrimSpace(req.Type),
		SchemaVersion: req.SchemaVersion,
		Subject:       req.Subject,
		Actor:         req.Actor,
		SessionID:     strings.TrimSpace(req.SessionID),
		RequestID:     strings.TrimSpace(httpCtx.RequestID),
		OccurredAt:    occurredAt,
		CreatedAt:     now,
		Properties:    props,
		Metadata: domain.EventMetadata{
			Source:        "http_api",
			SchemaVersion: req.SchemaVersion,
			DeliveryMode:  deliveryMode,
			IngestedAt:    now.Format(time.RFC3339Nano),
		},
	}

	// 1. Validation
	if err := c.validator.Validate(ctx, event, isPublic); err != nil {
		return nil, fmt.Errorf("validate event: %w", err)
	}

	// 2. Server-side context extraction (Client cannot spoof IP or Geo)
	eventCtx := domain.EventContext{
		UserAgent: httpCtx.UserAgent,
		Referer:   httpCtx.Referer,
	}

	if httpCtx.ClientIP != nil {
		eventCtx.IPHash = c.hasher.HashIP(httpCtx.ClientIP, occurredAt)
		if c.geoip != nil {
			if loc, err := c.geoip.Resolve(ctx, httpCtx.ClientIP); err == nil && loc != nil {
				eventCtx.CountryCode = loc.CountryCode
				eventCtx.CountryName = loc.CountryName
				eventCtx.City = loc.City
				eventCtx.Timezone = loc.Timezone
				eventCtx.Latitude = loc.Latitude
				eventCtx.Longitude = loc.Longitude
			}
		}
	}

	if httpCtx.UserAgent != "" && c.uaParser != nil {
		dev := c.uaParser.Parse(httpCtx.UserAgent)
		eventCtx.DeviceType = dev.DeviceType
		eventCtx.DeviceBrand = dev.DeviceBrand
		eventCtx.DeviceModel = dev.DeviceModel
		eventCtx.Browser = dev.Browser
		eventCtx.BrowserVersion = dev.BrowserVersion
		eventCtx.OS = dev.OS
		eventCtx.OSVersion = dev.OSVersion
	}

	event.Context = eventCtx

	// 3. Delivery routing (async vs sync vs hybrid)
	if deliveryMode == "" {
		deliveryMode = "async"
	}

	switch deliveryMode {
	case "sync":
		if c.directStore != nil {
			if err := c.directStore.StoreEvent(ctx, *event); err != nil {
				return nil, fmt.Errorf("direct store event: %w", err)
			}
		}
	case "hybrid":
		if c.queue != nil {
			if err := c.queue.PublishEvent(ctx, *event); err != nil {
				c.log.Warn("queue_publish_failed_fallback_sync", "error", err)
				if c.directStore != nil {
					_ = c.directStore.StoreEvent(ctx, *event)
				}
			}
		} else if c.directStore != nil {
			_ = c.directStore.StoreEvent(ctx, *event)
		}
	default: // "async"
		if c.queue != nil {
			if err := c.queue.PublishEvent(ctx, *event); err != nil {
				return nil, fmt.Errorf("enqueue event: %w", err)
			}
		} else if c.directStore != nil {
			_ = c.directStore.StoreEvent(ctx, *event)
		}
	}

	return event, nil
}
