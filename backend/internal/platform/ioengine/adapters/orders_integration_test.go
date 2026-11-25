package adapters_test

// Platform order export (TEC-596) against Postgres: the columns, the list filters and the values
// operators read (item count, total, account, customer).

import (
	"context"
	"testing"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/i18n"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/ioengine"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/ioengine/adapters"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/testutil/pgtest"
)

func TestIntegrationOrdersExport(t *testing.T) {
	d := pgtest.Open(t)
	ctx := context.Background()
	a := adapters.NewOrders(d.Queries)

	wantCols := []string{
		"order_number", "order_type", "status", "payment_status", "fulfillment_status",
		"account_name", "account_type", "customer_name", "customer_email", "items_count",
		"grand_total", "currency", "tracking_number", "created_at", "updated_at", "uuid",
	}
	cols := a.ExportColumns()
	if len(cols) != len(wantCols) {
		t.Fatalf("columns: %d, want %d", len(cols), len(wantCols))
	}
	for i, c := range cols {
		if c.Key != wantCols[i] || c.LabelKey == "" {
			t.Fatalf("column %d = %+v, want key %s", i, c, wantCols[i])
		}
		// Every header is translated in both PDF/XLSX languages.
		for _, loc := range []i18n.Locale{i18n.LocaleTR, i18n.LocaleEN} {
			if got := i18n.Translate(loc, c.LabelKey); got == c.LabelKey {
				t.Fatalf("%s: no %s label for %s", c.Key, loc, c.LabelKey)
			}
		}
	}

	u := d.CreateUser(t)
	var email string
	if err := d.Pool.QueryRow(ctx, `SELECT email FROM users WHERE id = $1`, u.ID).Scan(&email); err != nil {
		t.Fatal(err)
	}
	tag := pgtest.Unique()
	insert := func(suffix, status, payment, orderType string, total string, items int) {
		t.Helper()
		var id int64
		err := d.Pool.QueryRow(ctx, `
			INSERT INTO orders (order_number, user_id, account_id, status, payment_status, order_type,
			                    grand_total, currency, tracking_number)
			VALUES ($1, $2, $3, $4, $5, $6, $7::numeric, 'TRY', $8) RETURNING id`,
			"EXP-"+tag+"-"+suffix, u.ID, u.AccountID, status, payment, orderType, total, "TRK-"+tag+"-"+suffix,
		).Scan(&id)
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < items; i++ {
			if _, err := d.Pool.Exec(ctx, `INSERT INTO order_items (order_id, product_name, quantity) VALUES ($1, 'Kart', 1)`, id); err != nil {
				t.Fatal(err)
			}
		}
	}
	insert("A", "pending", "unpaid", "standard", "1250.5", 2)
	insert("B", "processing", "paid", "standard", "99", 1)
	insert("C", "processing", "paid", "bulk", "10.25", 0)
	// A deleted order never appears, as in the list.
	insert("D", "processing", "paid", "standard", "1", 0)
	if _, err := d.Pool.Exec(ctx, `UPDATE orders SET deleted_at = NOW() WHERE order_number = $1`, "EXP-"+tag+"-D"); err != nil {
		t.Fatal(err)
	}

	export := func(q ioengine.ExportQuery) map[string]map[string]any {
		t.Helper()
		q["q"] = "EXP-" + tag
		ds, err := a.Export(ctx, q, i18n.LocaleTR)
		if err != nil {
			t.Fatal(err)
		}
		if ds.Resource != adapters.ResourceOrders {
			t.Fatalf("resource %q", ds.Resource)
		}
		out := map[string]map[string]any{}
		for _, r := range ds.Rows {
			out[r["order_number"].(string)] = r
		}
		return out
	}

	all := export(ioengine.ExportQuery{})
	if len(all) != 3 {
		t.Fatalf("q filter: %d rows, want 3 (deleted order excluded)", len(all))
	}
	a1 := all["EXP-"+tag+"-A"]
	if a1["items_count"] != "2" || a1["grand_total"] != "1250.50" || a1["currency"] != "TRY" ||
		a1["status"] != "pending" || a1["payment_status"] != "unpaid" || a1["customer_email"] != email ||
		a1["account_type"] != "personal" || a1["account_name"] == "" || a1["tracking_number"] != "TRK-"+tag+"-A" {
		t.Fatalf("row values: %+v", a1)
	}
	for _, k := range wantCols {
		if _, ok := a1[k]; !ok {
			t.Fatalf("row misses column %s", k)
		}
	}

	if got := export(ioengine.ExportQuery{"status": "processing"}); len(got) != 2 || got["EXP-"+tag+"-A"] != nil {
		t.Fatalf("status filter: %v", keys(got))
	}
	if got := export(ioengine.ExportQuery{"payment_status": "unpaid"}); len(got) != 1 || got["EXP-"+tag+"-A"] == nil {
		t.Fatalf("payment_status filter: %v", keys(got))
	}
	if got := export(ioengine.ExportQuery{"order_type": "bulk"}); len(got) != 1 || got["EXP-"+tag+"-C"] == nil {
		t.Fatalf("order_type filter: %v", keys(got))
	}
	if got := export(ioengine.ExportQuery{"fulfillment_status": "shipped"}); len(got) != 0 {
		t.Fatalf("fulfillment_status filter: %v", keys(got))
	}

	// Export-only: the import side refuses.
	if len(a.ImportSchema()) != 0 {
		t.Fatal("orders must not be importable")
	}
	if res, _ := a.ApplyRow(ctx, map[string]any{}, nil); res.OK {
		t.Fatal("import row accepted")
	}
}

func keys(m map[string]map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
