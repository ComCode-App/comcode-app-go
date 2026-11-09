package legacymigration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/database/db"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/model"
	routerepo "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/routes/repository"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/publicpath"
)

// HoldingUUID identifies the platform-owned holding account. Platform staff may join it to review
// imported profiles, so membership never disqualifies it. A matching slug
// alone is insufficient: an unrelated customer's account must never receive legacy data.
const HoldingUUID = "a5b661de-d20d-5d62-a29a-14e262f84e05"
const fallbackPrefix = "r/legacy/"

type Postgres struct{ pool *pgxpool.Pool }

func NewPostgres(pool *pgxpool.Pool) *Postgres { return &Postgres{pool: pool} }

type owner struct {
	accountID int64
	userID    *int64
}
type target struct {
	kind, direct, expected string
	profileID              *int64
}

func (p *Postgres) Batch(ctx context.Context, rows []Row, opts Options) ([]Result, error) {
	txOpts := pgx.TxOptions{}
	if opts.DryRun {
		txOpts.AccessMode = pgx.ReadOnly
	}
	tx, err := p.pool.BeginTx(ctx, txOpts)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	// Serialize importers, including the case-insensitive check/insert and holding account creation.
	if !opts.DryRun {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(560560)`); err != nil {
			return nil, err
		}
	}
	results := make([]Result, 0, len(rows))
	// Track identical input repeats in a dry-run batch as well as in real imports.
	seen := make(map[string]Row)
	seenResults := make(map[string]Result)
	for _, row := range rows {
		if old, ok := seen[row.Key()]; ok {
			if !reflect.DeepEqual(old, row) {
				return nil, fmt.Errorf("hash %q has conflicting source rows", row.LegacyHash)
			}
			res := seenResults[row.Key()]
			if res.Status != BlockedAppPath {
				res.Status = "skipped"
			}
			results = append(results, res)
			continue
		}
		res, err := p.importRow(ctx, tx, row, opts)
		if err != nil {
			return nil, fmt.Errorf("hash %q: %w", row.LegacyHash, err)
		}
		results = append(results, res)
		seen[row.Key()] = row
		seenResults[row.Key()] = res
	}
	if opts.DryRun {
		return results, nil
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return results, nil
}

func (p *Postgres) importRow(ctx context.Context, tx pgx.Tx, row Row, opts Options) (Result, error) {
	res := Result{Hash: row.LegacyHash, Status: "created"}
	// Static pages never enter the public resolver. Do not create an unreachable
	// alias, even if a previous preparation already inserted one. Legacy-bound words
	// (comcode) are the exception: the printed card keeps its address (TEC-955).
	if publicpath.IsReserved(row.Key()) && !publicpath.IsLegacyBound(row.Key()) {
		res.Status = BlockedAppPath
		res.Warning = "operator decision required; no route imported"
		return res, nil
	}
	rr := routerepo.NewPostgres(db.New(tx))
	root, err := rr.GetRouteByPath(ctx, row.Key())
	if err != nil {
		return res, err
	}
	if root != nil && root.RouteType == model.RouteTypeSystemReserved {
		res.Status = BlockedAppPath
		res.Warning = "operator decision required; no route imported"
		return res, nil
	}
	alias, err := rr.GetRouteByPath(ctx, fallbackPrefix+row.Key())
	if err != nil {
		return res, err
	}
	existing := root
	if alias != nil {
		if alias.RouteType != model.RouteTypeLegacyRoute {
			return res, errors.New("legacy fallback path occupied by non-legacy route")
		}
		existing = alias
	}
	if existing != nil && existing.RouteType == model.RouteTypeLegacyRoute {
		raw, err := json.Marshal(existing.LegacyMetadata["source_row"])
		if err != nil {
			return res, err
		}
		var old Row
		if err := json.Unmarshal(raw, &old); err != nil {
			return res, errors.New("existing legacy route lacks normalized provenance")
		}
		if err := old.validate(); err != nil {
			return res, err
		}
		if !reflect.DeepEqual(old, row) {
			return res, errors.New("existing immutable legacy hash has different source data; refusing overwrite")
		}
		if !existing.IsImmutable {
			return res, errors.New("existing legacy route is not immutable")
		}
		res.Status = "skipped"
		if row.TargetType == "unknown" {
			res.Warning = "UNKNOWN_TARGET: cutover blocked until mapped"
		}
		if existing.Path != row.Key() {
			res.Warning = joinWarning(res.Warning, "COLLISION: preserve printed /"+row.LegacyHash+" via legacy resolver; stored at "+existing.Path)
		}
		return res, nil
	}
	path := row.Key()
	slugCollision := false
	if root == nil {
		for _, prefix := range []string{"s/", "c/"} {
			existing, err := rr.GetRouteByPath(ctx, prefix+row.Key())
			if err != nil {
				return res, err
			}
			if existing != nil {
				slugCollision = true
			}
		}
	}
	if root != nil || slugCollision {
		path = fallbackPrefix + row.Key()
		res.Warning = "COLLISION: preserve printed /" + row.LegacyHash + " via legacy resolver; stored at " + path
	}
	own, err := p.owner(ctx, tx, row, opts)
	if err != nil {
		return res, err
	}
	if opts.MatchOwners && row.OwnerEmail != "" && own.userID == nil {
		res.Warning = joinWarning(res.Warning, "OWNER_UNMATCHED: using holding/explicit account")
	}
	t, err := resolveTarget(ctx, tx, row, own, opts.FrontendURL)
	if err != nil {
		return res, err
	}
	if selfTarget(row, t.expected, opts.FrontendURL) {
		return res, errors.New("target loops back to the printed legacy URL")
	}
	if row.TargetType == "unknown" {
		res.Warning = joinWarning(res.Warning, "UNKNOWN_TARGET: cutover blocked until mapped")
	}
	if opts.DryRun {
		res.Status = "would_create"
		return res, nil
	}
	migrated := time.Now().UTC()
	metadata, err := json.Marshal(map[string]any{
		"legacy_system": "v1_legacy_platform", "original_hash": row.LegacyHash,
		"original_qr_url": "https://comcode.co/" + row.LegacyHash,
		"migration_batch": opts.BatchID, "migrated_at": migrated, "is_locked_legacy": true,
		"source_row": row, "resolver_path": row.Key(), "collision": path != row.Key(),
	})
	if err != nil {
		return res, err
	}
	var created *time.Time
	if row.CreatedAt != "" {
		parsed, err := time.Parse(time.RFC3339, row.CreatedAt)
		if err != nil {
			return res, err
		}
		created = &parsed
	}
	status := model.RouteStatusActive
	if row.TargetType == "unknown" {
		status = model.RouteStatusReserved
	}
	_, err = tx.Exec(ctx, `INSERT INTO routes
 (path,route_type,status,is_immutable,target_type,profile_id,direct_url,account_id,owner_user_id,legacy_metadata,created_at)
 VALUES ($1,'legacy_route',$2,true,$3,$4,$5,$6,$7,$8,COALESCE($9,now()))`,
		path, status, t.kind, t.profileID, t.direct, own.accountID, own.userID, metadata, created)
	return res, err
}

func joinWarning(a, b string) string {
	if a == "" {
		return b
	}
	return a + "; " + b
}

func (p *Postgres) owner(ctx context.Context, tx pgx.Tx, row Row, opts Options) (owner, error) {
	var o owner
	if opts.MatchOwners && row.OwnerEmail != "" {
		var userID, matches int64
		err := tx.QueryRow(ctx, `SELECT a.id,u.id,count(*) OVER() FROM users u JOIN accounts a ON a.personal_user_id=u.id
   WHERE lower(u.email)=lower($1) AND u.deleted_at IS NULL AND u.status='active' AND a.status='active'`, row.OwnerEmail).Scan(&o.accountID, &userID, &matches)
		if err == nil {
			if matches != 1 {
				return o, errors.New("ambiguous email matches multiple active users")
			}
			o.userID = &userID
			return o, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return o, err
		}
	}
	if opts.Account != "" && opts.Account != "holding" {
		id, err := uuid.Parse(opts.Account)
		if err != nil {
			return o, errors.New("account must be holding or an existing account UUID")
		}
		err = tx.QueryRow(ctx, `SELECT id FROM accounts WHERE uuid=$1 AND status='active'`, id).Scan(&o.accountID)
		return o, err
	}
	err := tx.QueryRow(ctx, `SELECT a.id FROM accounts a WHERE a.uuid=$1 AND a.slug='legacy'
  AND a.name='Legacy holding' AND a.type='company' AND a.status='active'`, HoldingUUID).Scan(&o.accountID)
	if err == nil {
		return o, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return o, err
	}
	var conflict bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM accounts WHERE slug='legacy' OR uuid=$1)`, HoldingUUID).Scan(&conflict); err != nil {
		return o, err
	}
	if conflict {
		return o, errors.New("holding account identity/slug is already in use; choose --account explicitly")
	}
	if opts.DryRun {
		return o, nil
	}
	err = tx.QueryRow(ctx, `INSERT INTO accounts (uuid,type,name,slug) VALUES ($1,'company','Legacy holding','legacy') RETURNING id`, HoldingUUID).Scan(&o.accountID)
	return o, err
}

func resolveTarget(ctx context.Context, tx pgx.Tx, row Row, own owner, frontend string) (target, error) {
	t := target{}
	switch row.TargetType {
	case "unknown":
		t.kind = model.TargetTypeCustomModules
		return t, nil
	case "url":
		parsed, err := url.Parse(row.TargetValue)
		if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Hostname() == "" || parsed.User != nil {
			return t, errors.New("url target must be an absolute HTTP(S) URL without credentials")
		}
		t.kind, t.direct, t.expected = model.TargetTypeDirectURL, row.TargetValue, row.TargetValue
		return t, nil
	case "smart_link":
		if !strings.HasPrefix(row.TargetValue, "s/") {
			return t, errors.New("smart_link target must be an existing s/slug path")
		}
		link, err := routerepo.NewPostgres(db.New(tx)).GetRouteByPath(ctx, row.TargetValue)
		if err != nil {
			return t, err
		}
		if link == nil || link.RouteType != model.RouteTypeCustomSmartLink || link.Status != model.RouteStatusActive || link.AccountID == nil || *link.AccountID != own.accountID {
			return t, errors.New("smart_link must be active and belong to the mapped account")
		}
		// Snapshot its target; never create a recursive resolver chain or copy passcodes/lead capture.
		if link.TargetType == model.TargetTypeDirectURL {
			row.TargetType, row.TargetValue = "url", link.DirectURL
		} else if (link.TargetType == model.TargetTypeProfile || link.TargetType == model.TargetTypeVCardDownload) && link.ProfileUUID != nil {
			row.TargetType, row.TargetValue = "profile", link.ProfileUUID.String()
			if link.TargetType == model.TargetTypeVCardDownload {
				row.TargetType = "vcard"
			}
		} else {
			return t, errors.New("unsupported smart_link target")
		}
		return resolveTarget(ctx, tx, row, own, frontend)
	case "profile", "vcard":
		id, err := uuid.Parse(row.TargetValue)
		if err != nil {
			return t, errors.New("profile/vcard target_value must be an existing profile UUID")
		}
		var profileID int64
		var username string
		err = tx.QueryRow(ctx, `SELECT p.id,r.path FROM profiles p JOIN routes r ON r.profile_id=p.id
   AND r.route_type='canonical_profile' AND r.status='active' AND r.deleted_at IS NULL
   WHERE p.uuid=$1 AND p.account_id=$2 AND p.deleted_at IS NULL`, id, own.accountID).Scan(&profileID, &username)
		if err != nil {
			return t, fmt.Errorf("profile must have an active public username in mapped account: %w", err)
		}
		var shadowed bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM routes WHERE path=$1 AND route_type='legacy_route' AND deleted_at IS NULL)`, fallbackPrefix+username).Scan(&shadowed); err != nil {
			return t, err
		}
		if shadowed {
			return t, errors.New("profile username is shadowed by a legacy hash; choose a new canonical username before cutover")
		}
		t.profileID = &profileID
		t.kind = model.TargetTypeProfile
		t.expected = strings.TrimRight(frontend, "/") + "/" + username
		if row.TargetType == "vcard" {
			t.kind = model.TargetTypeVCardDownload
			t.expected = strings.TrimRight(frontend, "/") + "/api/v1/public/p/" + username + "/vcard"
		}
		return t, nil
	}
	return t, errors.New("unsupported target")
}

// Expected derives the target from normalized input, never from the imported legacy route.
func (p *Postgres) Expected(ctx context.Context, row Row, opts Options) (string, error) {
	if publicpath.IsReserved(row.Key()) {
		return "", errors.New(BlockedAppPath)
	}
	opts.DryRun = true
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if row.TargetType == "unknown" {
		return "", errors.New("unknown target requires mapping")
	}
	own, err := p.owner(ctx, tx, row, opts)
	if err != nil {
		return "", err
	}
	t, err := resolveTarget(ctx, tx, row, own, opts.FrontendURL)
	if err == nil && selfTarget(row, t.expected, opts.FrontendURL) {
		return "", errors.New("target loops back to the printed legacy URL")
	}
	return t.expected, err
}

func selfTarget(row Row, expected, frontend string) bool {
	u, err := url.Parse(expected)
	if err != nil {
		return false
	}
	base, err := url.Parse(frontend)
	if err != nil {
		return false
	}
	return (strings.EqualFold(u.Host, base.Host) || strings.EqualFold(u.Host, "comcode.co")) && strings.Trim(strings.ToLower(u.Path), "/") == row.Key()
}
