package providers

import (
	"context"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/google/uuid"
)

type capturePublisher struct {
	channel string
	data    any
}

func (c *capturePublisher) Publish(_ context.Context, channel string, data any) error {
	c.channel = channel
	c.data = data
	return nil
}

func deliverRealtime(t *testing.T, payload []byte) map[string]any {
	t.Helper()
	pub := &capturePublisher{}
	user := uuid.New()
	n := db.Notification{Uuid: uuid.New(), Channel: "realtime", Status: "queued", Priority: "normal", Title: "Export ready", Payload: payload}
	if _, err := (RealtimeProvider{Pub: pub}).Deliver(context.Background(), n, &user); err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if pub.channel != "user:"+user.String() {
		t.Fatalf("channel = %q", pub.channel)
	}
	event, ok := pub.data.(map[string]any)
	if !ok || event["type"] != "notification.created" {
		t.Fatalf("event = %#v", pub.data)
	}
	notification, ok := event["notification"].(map[string]any)
	if !ok {
		t.Fatalf("notification = %#v", event["notification"])
	}
	return notification
}

func TestRealtimeProviderForwardsPayload(t *testing.T) {
	notification := deliverRealtime(t, []byte(`{"file":"leads.csv","rows":12}`))
	extra, ok := notification["payload"].(map[string]any)
	if !ok {
		t.Fatalf("payload missing: %#v", notification)
	}
	if extra["file"] != "leads.csv" || extra["rows"] != float64(12) {
		t.Fatalf("payload = %#v", extra)
	}
	if notification["title"] != "Export ready" {
		t.Fatalf("title = %#v", notification["title"])
	}
}

func TestRealtimeProviderOmitsEmptyPayload(t *testing.T) {
	for _, raw := range [][]byte{nil, []byte(`{}`), []byte(`not json`)} {
		if _, ok := deliverRealtime(t, raw)["payload"]; ok {
			t.Fatalf("payload set for %q", raw)
		}
	}
}
