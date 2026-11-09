package adapters

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	leadsmodel "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/leads/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/i18n"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/ioengine"
	"github.com/jackc/pgx/v5/pgtype"
)

const ResourceAppLeads = "app.leads"

// leadsExportCap bounds one export; the app list is paginated, the file is not.
const leadsExportCap = 10000

// LeadsAdapter exports the leads and contact exchanges of one account (TEC-552, TEC-567). The export route sets `account_id`
// from the caller's active account and `viewer_user_id` from the caller: owners and admins get
// the whole account, other members only their own profile's and card's leads (TEC-940). A query
// without both exports nothing. Locked leads (TEC-941) are masked as in the app. Export-only.
type LeadsAdapter struct {
	q *db.Queries
}

func NewLeads(q *db.Queries) *LeadsAdapter { return &LeadsAdapter{q: q} }

func (a *LeadsAdapter) Resource() string { return ResourceAppLeads }

func (a *LeadsAdapter) ExportColumns() []ioengine.Column {
	return []ioengine.Column{
		{Key: "uuid", LabelKey: "leads.uuid", Type: ioengine.ColumnTypeUUID},
		{Key: "name", LabelKey: "leads.name", Type: ioengine.ColumnTypeString},
		{Key: "email", LabelKey: "leads.email", Type: ioengine.ColumnTypeString},
		{Key: "phone", LabelKey: "leads.phone", Type: ioengine.ColumnTypeString},
		{Key: "company", LabelKey: "leads.company", Type: ioengine.ColumnTypeString},
		{Key: "job_title", LabelKey: "leads.job_title", Type: ioengine.ColumnTypeString},
		{Key: "message", LabelKey: "leads.message", Type: ioengine.ColumnTypeString},
		{Key: "kind", LabelKey: "leads.kind", Type: ioengine.ColumnTypeEnum},
		{Key: "source_type", LabelKey: "leads.source_type", Type: ioengine.ColumnTypeEnum},
		{Key: "route_path", LabelKey: "leads.route_path", Type: ioengine.ColumnTypeString},
		{Key: "consent_version", LabelKey: "leads.consent_version", Type: ioengine.ColumnTypeString},
		{Key: "consent_at", LabelKey: "leads.consent_at", Type: ioengine.ColumnTypeDatetime},
		{Key: "read", LabelKey: "leads.read", Type: ioengine.ColumnTypeBoolean},
		{Key: "locked", LabelKey: "leads.locked", Type: ioengine.ColumnTypeBoolean},
		{Key: "created_at", LabelKey: "leads.created_at", Type: ioengine.ColumnTypeDatetime},
	}
}

func (a *LeadsAdapter) Export(ctx context.Context, query ioengine.ExportQuery, _ i18n.Locale) (ioengine.Dataset, error) {
	accountID, err := strconv.ParseInt(strings.TrimSpace(query["account_id"]), 10, 64)
	if err != nil || accountID <= 0 {
		return ioengine.Dataset{}, fmt.Errorf("account_id is required")
	}
	viewerID, err := strconv.ParseInt(strings.TrimSpace(query["viewer_user_id"]), 10, 64)
	if err != nil || viewerID <= 0 {
		return ioengine.Dataset{}, fmt.Errorf("viewer_user_id is required")
	}
	params := db.ListAccountLeadsParams{
		AccountID: accountID, ViewerUserID: viewerID,
		Q: textArg(query["q"]), SourceType: textArg(query["source_type"]),
		Kind: textArg(query["kind"]), LimitCount: leadsExportCap,
	}
	if u := strings.TrimSpace(query["unread"]); u != "" {
		params.Unread = pgtype.Bool{Bool: u == "true" || u == "1", Valid: true}
	}
	rows, err := a.q.ListAccountLeads(ctx, params)
	if err != nil {
		return ioengine.Dataset{}, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		if r.Locked {
			l := leadsmodel.Lead{Name: r.Name, Email: r.Email, Phone: r.Phone, Message: r.Message}
			l.Mask()
			r.Name, r.Email, r.Phone, r.Message = l.Name, l.Email, l.Phone, l.Message
		}
		out = append(out, map[string]any{
			"uuid": r.Uuid.String(), "name": r.Name, "email": r.Email, "phone": r.Phone,
			"company": r.Company, "job_title": r.JobTitle, "message": r.Message, "kind": r.Kind,
			"locked": r.Locked, "source_type": r.SourceType, "route_path": r.RoutePath,
			"consent_version": r.ConsentVersion, "consent_at": r.ConsentAt.Time,
			"read": r.ReadAt.Valid, "created_at": r.CreatedAt.Time,
		})
	}
	return ioengine.Dataset{Resource: ResourceAppLeads, Columns: a.ExportColumns(), Rows: out}, nil
}

func (a *LeadsAdapter) ImportSchema() []ioengine.ImportField { return nil }

func (a *LeadsAdapter) ApplyRow(_ context.Context, _ map[string]any, _ map[string]any) (ioengine.RowResult, error) {
	return ioengine.RowResult{OK: false, Error: "import not supported"}, nil
}

func (a *LeadsAdapter) RevertRow(_ context.Context, _, _ string, _ map[string]any) error {
	return fmt.Errorf("import not supported")
}
