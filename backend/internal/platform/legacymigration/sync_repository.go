package legacymigration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	profiles "github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/modules/profiles/model"
	"github.com/furkanmeclis/nextjs-go-boilerplate/backend/internal/platform/publicpath"
	"github.com/jackc/pgx/v5"
)

type SyncResult struct {
	ID, Status, Username, Canonical, Warning string
	// Rule is the canonical username rule applied (see Canonical* constants); Previous is
	// the canonical a promotion replaces.
	Rule, Previous string
	// Sector is the matched sector slug, or "unmatched:<icon>" for an unknown ikon2 name.
	Sector string
	// Claim is the route that already holds this card's printed hash, if any.
	Claim   *HashClaim
	Changed bool
}

// Canonical username rules reported in the sync CSV (TEC-956 option E).
const (
	// CanonicalOwnHash: a new profile whose username is its own hash gets it as canonical.
	CanonicalOwnHash = "own_hash"
	// CanonicalOwnHashPromote: an imported profile moves from legacy-<uuid> to its hash.
	CanonicalOwnHashPromote = "own_hash_promote"
	// CanonicalOwnHashPending: as above, but the run did not pass --promote-own-hash.
	CanonicalOwnHashPending = "own_hash_pending"
)

// HashClaim is a route that took a printed legacy hash before the card was synced. The
// printed card wins: the resolver serves the card there, so the owner must pick a new name.
type HashClaim struct {
	LegacyHash, Path, RouteType, Status, ProfileUUID string
	AccountID, OwnerUserID                           int64
}

// Notify reports whether the claim is a live username whose owner should rename it.
func (c HashClaim) Notify() bool {
	return c.RouteType == "canonical_profile" && (c.Status == "active" || c.Status == "locked") && c.OwnerUserID != 0
}

func (c HashClaim) String() string {
	return c.RouteType + ":" + c.Status + ":" + c.ProfileUUID
}

type syncRules struct {
	PromoteOwnHash bool
	Sectors        sectorCatalog
}

func fallbackUsername(profileUUID string) string {
	return "legacy-" + strings.ReplaceAll(profileUUID, "-", "")
}

func (p *Postgres) syncUser(ctx context.Context, t *Transformed, opts Options, rules syncRules, media *MediaSync, reserved map[string]string) (SyncResult, error) {
	result := SyncResult{ID: t.ID, Status: "created", Username: t.Username, Warning: strings.Join(t.Warnings, "; ")}
	txOpts := pgx.TxOptions{}
	if opts.DryRun {
		txOpts.AccessMode = pgx.ReadOnly
	}
	tx, err := p.pool.BeginTx(ctx, txOpts)
	if err != nil {
		return result, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if !opts.DryRun {
		if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(560560)`); err != nil {
			return result, err
		}
	}
	key := t.Row.Key()
	// A legacy-bound word (comcode) stays reserved for everyone else but opens its own card.
	if publicpath.IsReserved(key) && !publicpath.IsLegacyBound(key) {
		result.Status = "blocked"
		result.Warning = BlockedAppPath
		return result, nil
	}
	if publicpath.IsLegacyBound(key) {
		result.Warning = joinWarning(result.Warning, "LEGACY_BOUND_RESERVED:"+key)
	}
	var profileID, accountID int64
	var oldHash, hash, canonical, oldAvatar, oldCover string
	var sectorID *int64
	var oldMetadata []byte
	err = tx.QueryRow(ctx, `SELECT s.profile_id,p.account_id,s.content_hash,s.legacy_hash,p.avatar_url,p.cover_url,s.metadata,p.sector_id FROM legacy_sync_users s JOIN profiles p ON p.id=s.profile_id WHERE s.legacy_user_id=$1 AND p.deleted_at IS NULL`, t.ID).Scan(&profileID, &accountID, &oldHash, &hash, &oldAvatar, &oldCover, &oldMetadata, &sectorID)
	exists := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return result, err
	}
	if exists && hash != key {
		return result, errors.New("printed legacy identity changed; manual review required")
	}
	var rootType string
	rootErr := tx.QueryRow(ctx, `SELECT route_type FROM routes WHERE path=$1`, key).Scan(&rootType)
	if rootErr != nil && !errors.Is(rootErr, pgx.ErrNoRows) {
		return result, rootErr
	}
	var aliasOccupied bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM routes WHERE path=$1)`, fallbackPrefix+key).Scan(&aliasOccupied); err != nil {
		return result, err
	}
	if !exists && (rootType == "legacy_route" || aliasOccupied) {
		result.Status = "blocked"
		result.Warning = "LEGACY_IDENTITY_ALREADY_IMPORTED_OR_ALIAS_OCCUPIED: manual review required"
		return result, nil
	}
	var legacyRouteID int64
	var legacyPath string
	if exists {
		err = tx.QueryRow(ctx, `SELECT id,path FROM routes WHERE profile_id=$1 AND route_type='legacy_route' AND is_immutable AND path IN ($2,$3) AND deleted_at IS NULL`, profileID, key, fallbackPrefix+key).Scan(&legacyRouteID, &legacyPath)
		if errors.Is(err, pgx.ErrNoRows) {
			return result, errors.New("mapped legacy identity missing or altered; manual review required")
		}
		if err != nil {
			return result, err
		}
	}
	// The printed card wins: list whoever took the hash first so they can rename (TEC-955).
	var claim HashClaim
	err = tx.QueryRow(ctx, `SELECT r.path,r.route_type,r.status,COALESCE(p.uuid::text,''),COALESCE(r.account_id,0),COALESCE(r.owner_user_id,p.user_id,0) FROM routes r LEFT JOIN profiles p ON p.id=r.profile_id WHERE lower(trim(r.path))=$1 AND r.deleted_at IS NULL AND r.route_type IN ('canonical_profile','custom_smart_link','card_route') AND ($2::bigint=0 OR r.profile_id IS DISTINCT FROM $2)`, key, profileID).Scan(&claim.Path, &claim.RouteType, &claim.Status, &claim.ProfileUUID, &claim.AccountID, &claim.OwnerUserID)
	if err == nil {
		claim.LegacyHash = t.Row.LegacyHash
		result.Claim = &claim
		result.Warning = joinWarning(result.Warning, "HASH_CLAIMED_BY:"+claim.String())
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return result, err
	}
	// Mapped profiles retain their current account and acting member after owner assignment.
	own := owner{accountID: accountID}
	if exists {
		if err = tx.QueryRow(ctx, `SELECT user_id FROM profiles WHERE id=$1`, profileID).Scan(&own.userID); err != nil {
			return result, err
		}
	} else {
		own, err = p.owner(ctx, tx, t.Row, opts)
		if err != nil {
			return result, err
		}
	}
	if opts.MatchOwners && own.userID == nil {
		result.Warning = joinWarning(result.Warning, "OWNER_UNMATCHED")
	}
	var canonicalID int64
	if exists {
		if err = tx.QueryRow(ctx, `SELECT id,path FROM routes WHERE profile_id=$1 AND route_type='canonical_profile' AND deleted_at IS NULL AND status IN ('active','locked')`, profileID).Scan(&canonicalID, &canonical); err != nil {
			return result, err
		}
	}
	claimable := publicpath.ValidateUsername(t.Username) == nil
	// Every printed hash counts as taken for every other card's username.
	if id, ok := reserved[t.Username]; ok && id != t.ID {
		claimable = false
	}
	var occupied bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM routes WHERE lower(path)=lower($1) AND ($2::bigint=0 OR profile_id IS DISTINCT FROM $2))`, t.Username, profileID).Scan(&occupied); err != nil {
		return result, err
	}
	if occupied {
		claimable = false
	}
	// TEC-956 option E: a username equal to the card's own hash becomes its canonical address.
	// The legacy route then lives at its alias, which the resolver skips for its own username.
	ownHash := claimable && t.Username == key
	// Only the sync's own fallback is replaced, never a name the owner chose in the panel.
	promote := ownHash && exists && canonical == fallbackUsername(t.Row.TargetValue) && legacyPath == key
	switch {
	case ownHash && !exists:
		result.Rule = CanonicalOwnHash
	case promote && rules.PromoteOwnHash:
		result.Rule = CanonicalOwnHashPromote
		result.Previous = canonical
	case promote:
		result.Rule = CanonicalOwnHashPending
		promote = false
		claimable = false
	case ownHash && canonical != t.Username:
		claimable = false
	}
	if !claimable {
		result.Username = "conflict"
		if result.Rule == CanonicalOwnHashPending {
			result.Warning = joinWarning(result.Warning, "USERNAME_OWN_HASH_PENDING:"+t.Username)
		} else {
			result.Warning = joinWarning(result.Warning, "USERNAME_CONFLICT:"+t.Username)
		}
	}
	username := t.Username
	if canonical != "" && !promote {
		username = canonical
	} else if !claimable {
		username = fallbackUsername(t.Row.TargetValue)
	}
	result.Canonical = username
	// Sectors only fill an empty link, so a later choice in the panel survives re-syncs.
	var linkSector *int64
	if s, ok := rules.Sectors.match(t.SectorIcons); ok {
		result.Sector = s.Slug
		if sectorID == nil {
			linkSector = &s.ID
		}
	} else if t.SectorIcon != "" {
		result.Sector = "unmatched:" + t.SectorIcon
		result.Warning = joinWarning(result.Warning, "SECTOR_UNMATCHED:"+t.SectorIcon)
	}
	// Reserve the generated canonical path too, including tombstones.
	if canonical == "" {
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM routes WHERE path=$1)`, username).Scan(&occupied); err != nil {
			return result, err
		}
		if occupied {
			result.Status = "blocked"
			result.Warning = joinWarning(result.Warning, "FALLBACK_USERNAME_OCCUPIED")
			return result, nil
		}
	}
	for _, ref := range t.Media {
		beforeMissing := media.Missing
		mediaURL, err := media.Put(ctx, t.Row.TargetValue, ref)
		if err != nil {
			return result, err
		}
		if media.Missing > beforeMissing {
			result.Warning = joinWarning(result.Warning, "MEDIA_MISSING:"+ref.Path)
		}
		if mediaURL == "" {
			if ref.Field == "avatar" {
				t.Profile.AvatarURL = oldAvatar
			}
			if ref.Field == "user_card_img" {
				t.Profile.CoverURL = oldCover
			}
			previous := map[string]any{}
			if err := json.Unmarshal(oldMetadata, &previous); err == nil {
				if value, ok := previous["media_"+ref.Field]; ok {
					t.Metadata["media_"+ref.Field] = value
				}
			}
			continue
		}
		switch ref.Field {
		case "avatar":
			t.Profile.AvatarURL = mediaURL
		case "user_card_img":
			t.Profile.CoverURL = mediaURL
		default:
			t.Metadata["media_"+ref.Field] = mediaURL
		}
	}
	// Media identity participates in the hash independently of user fields.
	mediaHashes := map[string]string{}
	for _, ref := range t.Media {
		if e, ok := media.Manifest[ref.Path]; ok {
			mediaHashes[ref.Path] = e.SHA1
		}
	}
	hashJSON, _ := json.Marshal(mediaHashes)
	t.ContentHash += "|" + string(hashJSON)
	unchanged := exists && oldHash == t.ContentHash
	switch {
	case unchanged:
		result.Status = "skipped"
	case exists:
		result.Status = "updated"
	}
	// Unchanged rows still get the targeted promotion and sector link writes.
	if unchanged && !promote && linkSector == nil {
		return result, nil
	}
	result.Changed = true
	if opts.DryRun {
		return result, nil
	}
	if !exists {
		if err = tx.QueryRow(ctx, `INSERT INTO profiles(uuid,account_id,user_id) VALUES($1,$2,$3) RETURNING id`, t.Row.TargetValue, own.accountID, own.userID).Scan(&profileID); err != nil {
			return result, err
		}
	}
	if linkSector != nil {
		if _, err = tx.Exec(ctx, `UPDATE profiles SET sector_id=$2 WHERE id=$1 AND sector_id IS NULL`, profileID, *linkSector); err != nil {
			return result, err
		}
	}
	if promote {
		if err = promoteOwnHash(ctx, tx, accountID, canonicalID, canonical, legacyRouteID, key); err != nil {
			return result, err
		}
	}
	if unchanged {
		return result, tx.Commit(ctx)
	}
	if err = writeSyncProfile(ctx, tx, profileID, t); err != nil {
		return result, err
	}
	status := "active"
	if !t.Published {
		status = "locked"
	}
	presentationMap := map[string]any{"enabled": t.Published, "visible_modules": profiles.PublicModules, "theme": t.Theme, "layout_mode": t.Layout}
	if len(t.Background) > 0 {
		bgID := syncBackgroundUUID(t.ID)
		if _, err = tx.Exec(ctx, `INSERT INTO backgrounds(uuid,account_id,name,kind,document) VALUES($1,$2,'Legacy profile background','solid',$3) ON CONFLICT(uuid) DO UPDATE SET document=excluded.document`, bgID, own.accountID, t.Background); err != nil {
			return result, err
		}
		presentationMap["background_uuid"] = bgID
	}
	presentation, _ := json.Marshal(presentationMap)
	if canonical == "" {
		if _, err = tx.Exec(ctx, `INSERT INTO routes(path,route_type,status,target_type,profile_id,account_id,owner_user_id,presentation_config) VALUES($1,'canonical_profile',$2,'profile',$3,$4,$5,$6)`, username, status, profileID, own.accountID, own.userID, presentation); err != nil {
			return result, err
		}
	} else {
		if _, err = tx.Exec(ctx, `UPDATE routes SET status=$2,presentation_config=$3 WHERE profile_id=$1 AND route_type='canonical_profile' AND status IN ('active','locked') AND deleted_at IS NULL`, profileID, status, presentation); err != nil {
			return result, err
		}
	}
	// Immutable source identity is inserted once; mutable profile state lives separately.
	if !exists {
		var routeExists bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM routes WHERE path=$1)`, key).Scan(&routeExists); err != nil {
			return result, err
		}
		routePath := key
		if routeExists {
			routePath = fallbackPrefix + routePath
			if result.Rule != CanonicalOwnHash {
				result.Warning = joinWarning(result.Warning, "LEGACY_ALIAS_COLLISION")
			}
		}
		metadata, _ := json.Marshal(map[string]any{"legacy_system": "v1_legacy_platform", "original_hash": t.Row.LegacyHash, "legacy_user_id": t.ID, "source_row": t.Row, "migration_batch": opts.BatchID, "owner_email": t.Row.OwnerEmail})
		if _, err = tx.Exec(ctx, `INSERT INTO routes(path,route_type,status,is_immutable,target_type,profile_id,account_id,owner_user_id,legacy_metadata) VALUES($1,'legacy_route',$2,true,'profile',$3,$4,$5,$6)`, routePath, status, profileID, own.accountID, own.userID, metadata); err != nil {
			return result, err
		}
	} else {
		if _, err = tx.Exec(ctx, `UPDATE routes SET status=$2 WHERE profile_id=$1 AND route_type='legacy_route'`, profileID, status); err != nil {
			return result, err
		}
	}
	metadata, err := json.Marshal(t.Metadata)
	if err != nil {
		return result, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO legacy_sync_users(legacy_user_id,profile_id,legacy_hash,content_hash,metadata) VALUES($1,$2,$3,$4,$5) ON CONFLICT(legacy_user_id) DO UPDATE SET content_hash=excluded.content_hash,metadata=excluded.metadata,updated_at=now()`, t.ID, profileID, key, t.ContentHash, metadata); err != nil {
		return result, err
	}
	if err = tx.Commit(ctx); err != nil {
		return result, err
	}
	return result, nil
}

// promoteOwnHash moves an imported profile from legacy-<uuid> to its own printed hash, like a
// panel rename: the old name is retired (it keeps redirecting) and recorded in route_history.
// The immutable legacy route moves to its alias, the one path change migration 000066 allows.
func promoteOwnHash(ctx context.Context, tx pgx.Tx, accountID, canonicalID int64, oldPath string, legacyRouteID int64, key string) error {
	if _, err := tx.Exec(ctx, `UPDATE routes SET path=$2 WHERE id=$1 AND route_type='legacy_route'`, legacyRouteID, fallbackPrefix+key); err != nil {
		return fmt.Errorf("move legacy route to alias: %w", err)
	}
	var status string
	var presentation []byte
	if err := tx.QueryRow(ctx, `SELECT status,presentation_config FROM routes WHERE id=$1`, canonicalID).Scan(&status, &presentation); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE routes SET status='retired' WHERE id=$1 AND route_type='canonical_profile'`, canonicalID); err != nil {
		return fmt.Errorf("retire fallback username: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO route_history(route_id,route_type,account_id,old_path,new_path,reason) VALUES($1,'canonical_profile',$2,$3,$4,'username_change')`, canonicalID, accountID, oldPath, key); err != nil {
		return fmt.Errorf("route history: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO routes(path,route_type,status,target_type,profile_id,account_id,owner_user_id,presentation_config) SELECT $2,'canonical_profile',$3,'profile',profile_id,account_id,owner_user_id,$4 FROM routes WHERE id=$1`, canonicalID, key, status, presentation); err != nil {
		return fmt.Errorf("own hash username: %w", err)
	}
	return nil
}

func writeSyncProfile(ctx context.Context, tx pgx.Tx, id int64, t *Transformed) error {
	if _, err := tx.Exec(ctx, `UPDATE profiles SET avatar_url=$2,cover_url=$3,gender=$4,birthday=$5::date WHERE id=$1`, id, t.Profile.AvatarURL, t.Profile.CoverURL, t.Profile.Gender, t.Profile.Birthday); err != nil {
		return err
	}
	for _, table := range []string{"profile_translations", "profile_addresses", "profile_contact_details", "profile_social_links"} {
		if _, err := tx.Exec(ctx, `DELETE FROM `+table+` WHERE profile_id=$1`, id); err != nil {
			return err
		}
	}
	for _, tr := range t.Profile.Translations {
		if _, err := tx.Exec(ctx, `INSERT INTO profile_translations(profile_id,locale,first_name,last_name,company_name,title,bio,note,is_default) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, id, tr.Locale, tr.FirstName, tr.LastName, tr.CompanyName, tr.Title, tr.Bio, tr.Note, tr.IsDefault); err != nil {
			return err
		}
	}
	for i, a := range t.Profile.Addresses {
		if _, err := tx.Exec(ctx, `INSERT INTO profile_addresses(profile_id,type,street,city,country,is_primary,display_order) VALUES($1,$2,$3,$4,$5,$6,$7)`, id, a.Type, a.Street, a.City, a.Country, a.IsPrimary, i); err != nil {
			return err
		}
	}
	for i, c := range t.Profile.ContactDetails {
		if _, err := tx.Exec(ctx, `INSERT INTO profile_contact_details(profile_id,category,type,label,value,is_primary,display_order) VALUES($1,$2,$3,$4,$5,$6,$7)`, id, c.Category, c.Type, c.Label, c.Value, c.IsPrimary, i); err != nil {
			return err
		}
	}
	for i, l := range t.Profile.SocialLinks {
		if _, err := tx.Exec(ctx, `INSERT INTO profile_social_links(profile_id,platform_id,platform,label,url,display_order,is_active) VALUES($1,(SELECT id FROM social_platforms WHERE slug=$2 AND deleted_at IS NULL LIMIT 1),$2,$3,$4,$5,$6)`, id, l.Platform, l.Label, l.URL, i, l.IsActive); err != nil {
			return err
		}
	}
	// Remove only prior sync-owned banks that disappeared from the source snapshot.
	var oldBankCount int
	if err := tx.QueryRow(ctx, `SELECT COALESCE((SELECT (metadata->>'bank_count')::int FROM legacy_sync_users WHERE legacy_user_id=$1),0)`, t.ID).Scan(&oldBankCount); err != nil {
		return err
	}
	for i := len(t.Banks); i < oldBankCount; i++ {
		if _, err := tx.Exec(ctx, `DELETE FROM user_bank_accounts WHERE uuid=$1 AND account_id=(SELECT account_id FROM profiles WHERE id=$2)`, syncBankUUID(t.ID, i), id); err != nil {
			return err
		}
	}
	// Imported bank rows have deterministic UUIDs, so unrelated owner bank accounts survive.
	for i, b := range t.Banks {
		if b.IBAN == "" {
			if _, err := tx.Exec(ctx, `DELETE FROM user_bank_accounts WHERE uuid=$1 AND account_id=(SELECT account_id FROM profiles WHERE id=$2)`, syncBankUUID(t.ID, i), id); err != nil {
				return err
			}
			continue
		}
		bankUUID := syncBankUUID(t.ID, i)
		var bankID int64
		if err := tx.QueryRow(ctx, `INSERT INTO user_bank_accounts(uuid,account_id,user_id,account_holder,iban,title) SELECT $1,account_id,user_id,$3,$4,$5 FROM profiles WHERE id=$2 ON CONFLICT(uuid) DO UPDATE SET account_holder=excluded.account_holder,iban=excluded.iban,title=excluded.title RETURNING id`, bankUUID, id, short(b.Holder, 255), short(b.IBAN, 64), short(b.Bank, 150)).Scan(&bankID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO profile_bank_accounts(profile_id,bank_account_id,display_order) VALUES($1,$2,$3) ON CONFLICT(profile_id,bank_account_id) DO UPDATE SET display_order=excluded.display_order`, id, bankID, i); err != nil {
			return err
		}
	}
	return nil
}

func (p *Postgres) syncHit(ctx context.Context, h map[string]string, dry bool) (bool, error) {
	eventType := map[string]string{"profile": "legacy.profile.view", "main_link": "legacy.main_link.click", "social": "legacy.social.click", "contact": "legacy.contact.click"}[h["type"]]
	if eventType == "" || h["id"] == "" || h["date"] == "" {
		return false, nil
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var subject string
	err = tx.QueryRow(ctx, `SELECT p.uuid::text FROM legacy_sync_users s JOIN profiles p ON p.id=s.profile_id WHERE s.legacy_user_id=$1`, h["user_id"]).Scan(&subject)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if dry {
		return true, nil
	}
	tag, err := tx.Exec(ctx, `INSERT INTO legacy_sync_hit_ids(legacy_hit_id) VALUES($1::bigint) ON CONFLICT DO NOTHING`, h["id"])
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	// Old profile page hits are "eski kart" entries of the profile (TEC-953).
	source := ""
	if eventType == "legacy.profile.view" {
		source = "legacy"
	}
	_, err = tx.Exec(ctx, `INSERT INTO event_aggregates_daily(bucket_date,subject_type,subject_id,event_type,total_count,source) VALUES($1::date,'profile',$2,$3,1,$4) ON CONFLICT(bucket_date,subject_type,subject_id,event_type,source) DO UPDATE SET total_count=event_aggregates_daily.total_count+1,updated_at=now()`, h["date"][:10], subject, eventType, source)
	if err != nil {
		return false, fmt.Errorf("historical aggregate: %w", err)
	}
	return true, tx.Commit(ctx)
}
