package adapters

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/i18n"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/ioengine"
	"github.com/jackc/pgx/v5/pgtype"
)

const ResourceOrders = "platform.orders"

// ordersExportCap bounds one export; the platform list is paginated, the file is not.
const ordersExportCap = 10000

// OrdersAdapter exports the platform order list (TEC-596). It reads the same query as
// GET /v1/platform/orders, so the file honors the list's filters: q (order number, customer
// email/name/surname, tracking number), status, payment_status, fulfillment_status and
// order_type, newest first. The list has no sort or date-range parameters, so neither does the
// export. Platform scope: every account's orders, like the list. Export-only; the production
// manifest of one order is OrderManifestAdapter.
type OrdersAdapter struct {
	q *db.Queries
}

func NewOrders(q *db.Queries) *OrdersAdapter { return &OrdersAdapter{q: q} }

func (a *OrdersAdapter) Resource() string { return ResourceOrders }

func (a *OrdersAdapter) ExportColumns() []ioengine.Column {
	return []ioengine.Column{
		{Key: "order_number", LabelKey: "orders.fields.order_number", Type: ioengine.ColumnTypeString},
		{Key: "order_type", LabelKey: "orders.fields.order_type", Type: ioengine.ColumnTypeEnum},
		{Key: "status", LabelKey: "orders.fields.status", Type: ioengine.ColumnTypeEnum},
		{Key: "payment_status", LabelKey: "orders.fields.payment_status", Type: ioengine.ColumnTypeEnum},
		{Key: "fulfillment_status", LabelKey: "orders.fields.fulfillment_status", Type: ioengine.ColumnTypeEnum},
		{Key: "account_name", LabelKey: "orders.fields.account", Type: ioengine.ColumnTypeString},
		{Key: "account_type", LabelKey: "orders.fields.account_type", Type: ioengine.ColumnTypeEnum},
		{Key: "customer_name", LabelKey: "orders.fields.customer", Type: ioengine.ColumnTypeString},
		{Key: "customer_email", LabelKey: "orders.fields.customer_email", Type: ioengine.ColumnTypeString},
		{Key: "items_count", LabelKey: "orders.fields.items_count", Type: ioengine.ColumnTypeString},
		{Key: "grand_total", LabelKey: "orders.fields.grand_total", Type: ioengine.ColumnTypeString},
		{Key: "currency", LabelKey: "orders.fields.currency", Type: ioengine.ColumnTypeString},
		{Key: "tracking_number", LabelKey: "orders.fields.tracking_number", Type: ioengine.ColumnTypeString},
		{Key: "created_at", LabelKey: "orders.fields.created_at", Type: ioengine.ColumnTypeDatetime},
		{Key: "updated_at", LabelKey: "orders.fields.updated_at", Type: ioengine.ColumnTypeDatetime},
		{Key: "uuid", LabelKey: "orders.fields.uuid", Type: ioengine.ColumnTypeUUID},
	}
}

func (a *OrdersAdapter) Export(ctx context.Context, query ioengine.ExportQuery, _ i18n.Locale) (ioengine.Dataset, error) {
	rows, err := a.q.ListOrders(ctx, db.ListOrdersParams{
		Q:                 textArg(query["q"]),
		Status:            textArg(query["status"]),
		PaymentStatus:     textArg(query["payment_status"]),
		FulfillmentStatus: textArg(query["fulfillment_status"]),
		OrderType:         textArg(query["order_type"]),
		LimitCount:        ordersExportCap,
	})
	if err != nil {
		return ioengine.Dataset{}, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{
			"order_number":       r.OrderNumber,
			"order_type":         r.OrderType,
			"status":             r.Status,
			"payment_status":     r.PaymentStatus,
			"fulfillment_status": r.FulfillmentStatus,
			"account_name":       r.AccountName,
			"account_type":       string(r.AccountType),
			"customer_name":      strings.TrimSpace(r.CustomerName + " " + r.CustomerSurname),
			"customer_email":     r.CustomerEmail,
			"items_count":        strconv.Itoa(int(r.ItemsCount)),
			"grand_total":        moneyString(r.GrandTotal),
			"currency":           r.Currency,
			"tracking_number":    r.TrackingNumber,
			"created_at":         r.CreatedAt.Time,
			"updated_at":         r.UpdatedAt.Time,
			"uuid":               r.Uuid.String(),
		})
	}
	return ioengine.Dataset{Resource: ResourceOrders, Columns: a.ExportColumns(), Rows: out}, nil
}

// moneyString prints a NUMERIC amount with two decimals ("1250.50"), the precision orders keep.
func moneyString(n pgtype.Numeric) string {
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return ""
	}
	return strconv.FormatFloat(f.Float64, 'f', 2, 64)
}

func (a *OrdersAdapter) ImportSchema() []ioengine.ImportField { return nil }

func (a *OrdersAdapter) ApplyRow(_ context.Context, _ map[string]any, _ map[string]any) (ioengine.RowResult, error) {
	return ioengine.RowResult{OK: false, Error: "import not supported"}, nil
}

func (a *OrdersAdapter) RevertRow(_ context.Context, _, _ string, _ map[string]any) error {
	return fmt.Errorf("import not supported")
}
