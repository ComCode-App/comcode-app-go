package ingestion_test

import (
	"context"
	"net"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/device"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/domain"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/ingestion"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/analytics/privacy"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/geoip"
)

type mockQueuePublisher struct {
	published []domain.Event
}

func (m *mockQueuePublisher) PublishEvent(ctx context.Context, event domain.Event) error {
	m.published = append(m.published, event)
	return nil
}

type mockDirectStore struct {
	stored []domain.Event
}

func (m *mockDirectStore) StoreEvent(ctx context.Context, event domain.Event) error {
	m.stored = append(m.stored, event)
	return nil
}

type staticRegistry struct{}

func (s *staticRegistry) Get(ctx context.Context, key string) (*domain.EventDefinition, error) {
	return &domain.EventDefinition{
		Key:              key,
		Enabled:          true,
		IsPublic:         true,
		SchemaVersion:    1,
		AggregateEnabled: true,
	}, nil
}

func TestCollector_Ingest(t *testing.T) {
	reg := &staticRegistry{}
	validator := domain.NewValidator(reg)
	mockGeo := geoip.NewMock(geoip.GeoLocation{CountryCode: "TR", City: "Istanbul"})
	hasher := privacy.NewRotatingSaltHasher("test-salt", 7)
	uaParser := device.NewParser()
	queuePub := &mockQueuePublisher{}
	directStore := &mockDirectStore{}

	collector := ingestion.NewCollector(validator, mockGeo, hasher, uaParser, queuePub, directStore, nil)
	ctx := context.Background()

	t.Run("async ingestion with context enrichment", func(t *testing.T) {
		req := ingestion.IngestRequest{
			Type: "card.view",
			Subject: domain.EntityReference{
				Type: "card",
				ID:   "c_987",
			},
			SessionID:  "sess_123",
			Properties: map[string]any{"source": "qr"},
		}

		httpCtx := ingestion.HTTPContext{
			ClientIP:  net.ParseIP("212.252.128.1"),
			UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15",
			Referer:   "https://example.com/qr",
			RequestID: "req_xyz",
		}

		event, err := collector.Ingest(ctx, req, httpCtx, true, "async")
		if err != nil {
			t.Fatalf("ingest failed: %v", err)
		}

		if event.UUID == "" {
			t.Fatal("expected generated UUID")
		}
		if event.Context.CountryCode != "TR" || event.Context.City != "Istanbul" {
			t.Fatalf("unexpected geo context: %+v", event.Context)
		}
		if event.Context.DeviceType != "mobile" || event.Context.OS != "iOS" {
			t.Fatalf("unexpected device context: %+v", event.Context)
		}
		if event.Context.IPHash == "" {
			t.Fatal("expected non-empty ip_hash")
		}

		if len(queuePub.published) != 1 {
			t.Fatalf("expected 1 queued event, got %d", len(queuePub.published))
		}
	})

	t.Run("sync ingestion mode", func(t *testing.T) {
		req := ingestion.IngestRequest{
			Type: "card.view",
			Subject: domain.EntityReference{
				Type: "card",
				ID:   "c_987",
			},
		}

		httpCtx := ingestion.HTTPContext{
			ClientIP: net.ParseIP("127.0.0.1"),
		}

		_, err := collector.Ingest(ctx, req, httpCtx, true, "sync")
		if err != nil {
			t.Fatalf("ingest failed: %v", err)
		}

		if len(directStore.stored) != 1 {
			t.Fatalf("expected 1 direct stored event, got %d", len(directStore.stored))
		}
	})
}
