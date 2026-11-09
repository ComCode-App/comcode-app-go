package adapters

import (
	"context"
	"fmt"
	"strings"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/i18n"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/ioengine"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/qr"
	"github.com/google/uuid"
)

const ResourceOrderManifest = "platform.order_manifest"

// OrderManifestAdapter exports the production manifest of one order (TEC-581): every provisioned
// card with its serial, the NFC identity URL, marked QR payload URL, and the
// activation code. Export-only; the route requires platform.orders.read and step-up.
type OrderManifestAdapter struct {
	q           *db.Queries
	frontendURL string
}

func NewOrderManifest(q *db.Queries, frontendURL string) *OrderManifestAdapter {
	return &OrderManifestAdapter{q: q, frontendURL: strings.TrimRight(frontendURL, "/")}
}

func (a *OrderManifestAdapter) Resource() string { return ResourceOrderManifest }

func (a *OrderManifestAdapter) ExportColumns() []ioengine.Column {
	return []ioengine.Column{
		{Key: "order_number", LabelKey: "manifest.order_number", Type: ioengine.ColumnTypeString},
		{Key: "product", LabelKey: "manifest.product", Type: ioengine.ColumnTypeString},
		{Key: "variant", LabelKey: "manifest.variant", Type: ioengine.ColumnTypeString},
		{Key: "sku", LabelKey: "manifest.sku", Type: ioengine.ColumnTypeString},
		{Key: "recipient", LabelKey: "manifest.recipient", Type: ioengine.ColumnTypeString},
		{Key: "card_name", LabelKey: "manifest.card_name", Type: ioengine.ColumnTypeString},
		{Key: "serial", LabelKey: "manifest.serial", Type: ioengine.ColumnTypeString},
		{Key: "smart_code", LabelKey: "manifest.smart_code", Type: ioengine.ColumnTypeString},
		{Key: "payload_url", LabelKey: "manifest.payload_url", Type: ioengine.ColumnTypeString},
		{Key: "qr_payload_url", LabelKey: "manifest.qr_payload_url", Type: ioengine.ColumnTypeString},
		{Key: "activation_code", LabelKey: "manifest.activation_code", Type: ioengine.ColumnTypeString},
		{Key: "card_status", LabelKey: "manifest.card_status", Type: ioengine.ColumnTypeEnum},
	}
}

func (a *OrderManifestAdapter) Export(ctx context.Context, query ioengine.ExportQuery, _ i18n.Locale) (ioengine.Dataset, error) {
	orderUUID, err := uuid.Parse(strings.TrimSpace(query["order_uuid"]))
	if err != nil {
		return ioengine.Dataset{}, fmt.Errorf("order_uuid: %w", err)
	}
	rows, err := a.q.ListOrderManifest(ctx, orderUUID)
	if err != nil {
		return ioengine.Dataset{}, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{
			"order_number": r.OrderNumber, "product": r.ProductName, "variant": r.VariantName,
			"sku": r.Sku, "recipient": r.RecipientName, "card_name": r.CardName,
			"serial": r.CardUid.String, "smart_code": r.SmartCode,
			"payload_url":     CardPayloadURL(a.frontendURL, r.SmartCode),
			"qr_payload_url":  qr.CardPayloadURL(a.frontendURL, r.SmartCode),
			"activation_code": r.ActivationCode.String, "card_status": r.CardStatus,
		})
	}
	return ioengine.Dataset{Resource: ResourceOrderManifest, Columns: a.ExportColumns(), Rows: out}, nil
}

// CardPayloadURL preserves the existing NFC payload URL.
func CardPayloadURL(frontendURL, smartCode string) string {
	return qr.CardIdentityURL(frontendURL, smartCode)
}

func (a *OrderManifestAdapter) ImportSchema() []ioengine.ImportField { return nil }

func (a *OrderManifestAdapter) ApplyRow(_ context.Context, _ map[string]any, _ map[string]any) (ioengine.RowResult, error) {
	return ioengine.RowResult{OK: false, Error: "import not supported"}, nil
}

func (a *OrderManifestAdapter) RevertRow(_ context.Context, _, _ string, _ map[string]any) error {
	return fmt.Errorf("import not supported")
}
