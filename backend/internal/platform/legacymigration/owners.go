package legacymigration

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"os"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type OwnersOptions struct {
	UsersCSV, OverridesCSV string
	Client                 *ExportClient
	DryRun                 bool
}

type OwnersSummary struct {
	Batch        string `json:"batch"`
	Individuals  int    `json:"individuals"`
	Companies    int    `json:"companies"`
	CompanyUsers int    `json:"company_users"`
	Skipped      int    `json:"skipped"`
	Assigned     int    `json:"assigned"`
	// RolesAssigned counts created legacy users that received the signup default role in this run,
	// including users from earlier batches that predate role assignment.
	RolesAssigned int `json:"roles_assigned"`
}

func readOwnerOverrides(path string) (map[string]string, error) {
	result := map[string]string{}
	if path == "" {
		return result, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		return nil, err
	}
	if len(header) != 2 || header[0] != "legacy_user_id" || header[1] != "group" {
		return nil, errors.New("overrides header must be legacy_user_id,group")
	}
	for {
		row, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		id, g := strings.TrimSpace(row[0]), strings.TrimSpace(row[1])
		if id == "" || g == "" {
			return nil, errors.New("empty override ID/group")
		}
		if _, ok := result[id]; ok {
			return nil, fmt.Errorf("duplicate override ID %s", id)
		}
		result[id] = g
	}
	return result, nil
}

// Owners plans and applies a complete assignment in one transaction, without notifications or quotas.
func Owners(ctx context.Context, repo *Postgres, opts OwnersOptions, report io.Writer) (OwnersSummary, error) {
	summary := OwnersSummary{}
	overrides, err := readOwnerOverrides(opts.OverridesCSV)
	if err != nil {
		return summary, err
	}
	users := []ownerCandidate{}
	seen := map[string]bool{}
	err = readCSV(ctx, opts.Client, opts.UsersCSV, "users", "", func(row map[string]string) error {
		c := ownerSource(row)
		if c.ID == "" || seen[c.ID] {
			return errors.New("missing or duplicate legacy user ID")
		}
		seen[c.ID] = true
		users = append(users, c)
		return nil
	})
	if err != nil {
		return summary, err
	}
	for id := range overrides {
		if !seen[id] {
			return summary, fmt.Errorf("unknown override ID %s", id)
		}
	}
	sort.SliceStable(users, func(i, j int) bool { return legacyIDLess(users[i].ID, users[j].ID) })
	txOpts := pgx.TxOptions{}
	if opts.DryRun {
		txOpts.AccessMode = pgx.ReadOnly
	}
	tx, err := repo.pool.BeginTx(ctx, txOpts)
	if err != nil {
		return summary, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if !opts.DryRun {
		if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(560560)`); err != nil {
			return summary, err
		}
	}
	var holding int64
	err = tx.QueryRow(ctx, `SELECT a.id FROM accounts a WHERE a.uuid=$1 AND a.slug='legacy'
  AND a.name='Legacy holding' AND a.type='company' AND a.status='active'`, HoldingUUID).Scan(&holding)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return summary, err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		var conflict bool
		queryErr := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM accounts WHERE uuid=$1 OR slug='legacy')`,
			HoldingUUID).Scan(&conflict)
		if queryErr != nil {
			return summary, queryErr
		}
		if conflict {
			return summary, errors.New("holding account identity/slug is already in use")
		}
	}
	for i := range users {
		c := &users[i]
		var account int64
		err = tx.QueryRow(ctx, `SELECT p.id,p.account_id FROM legacy_sync_users s
            JOIN profiles p ON p.id=s.profile_id
            WHERE s.legacy_user_id=$1 AND p.deleted_at IS NULL`, c.ID).Scan(&c.ProfileID, &account)
		if errors.Is(err, pgx.ErrNoRows) {
			c.Skip = "not_imported"
			continue
		}
		if err != nil {
			return summary, err
		}
		if account != holding {
			c.Skip = "already_assigned"
		}
	}
	groups := classifyOwners(users)
	// Overrides move candidates after both classifier refinements; skipped users remain skipped.
	for k, ids := range groups {
		keep := []int{}
		for _, i := range ids {
			if overrides[users[i].ID] == "" {
				keep = append(keep, i)
			}
		}
		groups[k] = keep
	}
	for i := range users {
		c := &users[i]
		g := overrides[c.ID]
		if g == "" || c.Skip != "" {
			continue
		}
		switch g {
		case "holding":
			c.Skip = "override_holding"
		case "solo":
			groups["solo:"+c.ID] = append(groups["solo:"+c.ID], i)
		default:
			groups["override:"+g] = append(groups["override:"+g], i)
		}
	}
	keys := []string{}
	for k, ids := range groups {
		if len(ids) > 0 {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		ids := groups[k]
		sort.Slice(ids, func(i, j int) bool { return legacyIDLess(users[ids[i]].ID, users[ids[j]].ID) })
		for _, i := range ids {
			c := &users[i]
			c.Group = k
			addr, parseErr := mail.ParseAddress(c.Email)
			if parseErr != nil || addr.Address != c.Email || len(c.Email) > 255 {
				return summary, fmt.Errorf("legacy user %s has invalid login email", c.ID)
			}
		}
		if len(ids) == 1 {
			summary.Individuals++
		} else {
			summary.Companies++
			summary.CompanyUsers += len(ids)
		}
		summary.Assigned += len(ids)
	}
	for _, c := range users {
		if c.Skip != "" {
			summary.Skipped++
		}
	}
	if !opts.DryRun && summary.Assigned > 0 {
		summary.Batch = uuid.NewString()
		if _, err = tx.Exec(ctx, `INSERT INTO legacy_owner_batches(id,mode) VALUES($1,'apply')`, summary.Batch); err != nil {
			return summary, err
		}
		for _, k := range keys {
			group := ownerGroup{
				Batch: summary.Batch, Key: k, Holding: holding, IDs: groups[k], Users: users,
			}
			if err = assignOwnerGroup(ctx, tx, group); err != nil {
				return summary, err
			}
		}
		data, marshalErr := json.Marshal(summary)
		if marshalErr != nil {
			return summary, marshalErr
		}
		if _, err = tx.Exec(
			ctx,
			`UPDATE legacy_owner_batches SET summary=$2 WHERE id=$1`,
			summary.Batch,
			data,
		); err != nil {
			return summary, err
		}
	}
	if summary.RolesAssigned, err = assignOwnerDefaultRoles(ctx, tx, opts.DryRun); err != nil {
		return summary, err
	}
	w := csv.NewWriter(report)
	if err = w.Write([]string{"legacy_user_id", "profile_id", "email", "group", "kind", "reason"}); err != nil {
		return summary, err
	}
	for _, c := range users {
		kind := "individual"
		if len(groups[c.Group]) >= 2 {
			kind = "company"
		}
		if c.Skip != "" {
			kind = "skipped"
		}
		if err = w.Write([]string{c.ID, fmt.Sprint(c.ProfileID), c.Email, c.Group, kind, c.Skip}); err != nil {
			return summary, err
		}
	}
	w.Flush()
	if err = w.Error(); err != nil {
		return summary, err
	}
	if opts.DryRun {
		return summary, nil
	}
	return summary, tx.Commit(ctx)
}

// ownerRolesMissing selects users created by applied batches that still lack any platform role.
// The role mirrors signup: auth_settings.default_role_id, else the seeded subscriber role.
const ownerRolesMissing = `WITH r AS (
  SELECT COALESCE((SELECT default_role_id FROM auth_settings WHERE id=1),
                  (SELECT id FROM roles WHERE slug='subscriber')) AS id)
SELECT DISTINCT i.user_id, r.id
FROM legacy_owner_items i
JOIN legacy_owner_batches b ON b.id=i.batch_id AND b.mode='apply'
JOIN users u ON u.id=i.user_id AND u.deleted_at IS NULL
CROSS JOIN r
WHERE i.created_user AND r.id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM user_roles ur WHERE ur.user_id=i.user_id)`

// assignOwnerDefaultRoles gives created legacy users the same default role as self-signup. Rollback
// needs no journal for it: deleting a created user cascades its user_roles rows.
func assignOwnerDefaultRoles(ctx context.Context, tx pgx.Tx, dryRun bool) (int, error) {
	if dryRun {
		var n int
		err := tx.QueryRow(ctx, `SELECT count(*) FROM (`+ownerRolesMissing+`) m`).Scan(&n)
		return n, err
	}
	tag, err := tx.Exec(ctx, `INSERT INTO user_roles(user_id, role_id) `+ownerRolesMissing+` ON CONFLICT DO NOTHING`)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}
