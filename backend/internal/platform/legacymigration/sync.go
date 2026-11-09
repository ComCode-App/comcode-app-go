package legacymigration

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"golang.org/x/sys/unix"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type SyncOptions struct {
	Options
	UsersCSV, HitsCSV, MainTypesCSV, StateDir string
	Client                                    *ExportClient
	Media                                     *MediaSync
	// PromoteOwnHash moves imported profiles whose username is their own hash from
	// legacy-<uuid> to that hash (TEC-956 option E). Without it the report lists them only.
	PromoteOwnHash bool
	// RouteCache is invalidated for promoted profiles; a stale cached legacy route would
	// otherwise redirect /{hash} to the retired name and back. Required with PromoteOwnHash.
	RouteCache RouteCacheInvalidator
	// Notifier hears about live usernames that hold a printed hash. Optional; dry runs never call it.
	Notifier HashClaimNotifier
}

type RouteCacheInvalidator interface {
	InvalidateProfile(ctx context.Context, profileUUID uuid.UUID)
}

// HashClaimNotifier asks a username owner to pick a new name because a printed legacy card
// owns that address (TEC-955). Implementations send panel (in-app) notifications only, never
// email, and must dedupe: every sync run reports every claim that still stands.
type HashClaimNotifier interface {
	HashClaimed(ctx context.Context, claim HashClaim) error
}
type SyncState struct {
	Version   int
	Database  string
	HitsSince string
	Users     map[string]string
	Media     MediaSnapshot
	Uploads   map[string]MediaUpload
}
type SyncSummary struct {
	Profiles, Creates, Updates, Skips, Blocked, Claimable, Conflicting, Inactive, MediaReferenced, MediaFiles, MediaUpload, MediaMissing, HitsRead, HitsImported, HitsSkipped int
	OwnHash, OwnHashPromoted, OwnHashPending, HashClaims, SectorMatched, SectorUnmatched                                                                                      int
}

func syncBackgroundUUID(id string) uuid.UUID {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte("comcode:legacy:background:"+id))
}

func syncBankUUID(id string, index int) uuid.UUID {
	return uuid.NewSHA1(uuid.NameSpaceURL, fmt.Appendf(nil, "comcode:legacy:bank:%s:%d", id, index))
}

// Sync checkpoints complete runs; database mappings also make partial retries safe.
func Sync(ctx context.Context, repo *Postgres, opts SyncOptions, report io.Writer) (SyncSummary, error) {
	var summary SyncSummary
	if opts.StateDir == "" {
		opts.StateDir = "./var/legacy-sync"
	}
	if opts.PromoteOwnHash && !opts.DryRun && opts.RouteCache == nil {
		return summary, errors.New("promoting own-hash usernames requires the route cache")
	}
	state := SyncState{Version: 1, HitsSince: "1970-01-01 00:00:00", Users: map[string]string{}, Media: MediaSnapshot{}, Uploads: map[string]MediaUpload{}}
	statePath := filepath.Join(opts.StateDir, "state.json")
	if err := repo.pool.QueryRow(ctx, `SELECT current_database() || ':' || (SELECT system_identifier::text FROM pg_control_system())`).Scan(&state.Database); err != nil {
		return summary, err
	}
	database := state.Database

	if !opts.DryRun {
		if err := os.MkdirAll(opts.StateDir, 0700); err != nil {
			return summary, err
		}
		lock, err := os.OpenFile(filepath.Join(opts.StateDir, "sync.lock"), os.O_CREATE|os.O_RDWR, 0600)
		if err != nil {
			return summary, err
		}
		defer func() { _ = lock.Close() }()
		if err = unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
			return summary, errors.New("another sync holds the state lock")
		}
	}
	if data, err := os.ReadFile(statePath); err == nil {
		if err = json.Unmarshal(data, &state); err != nil {
			return summary, err
		}
		if state.Version != 1 || state.Database != database {
			return summary, errors.New("state belongs to another database/version; use a new state dir")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return summary, err
	}
	if opts.Media == nil {
		opts.Media = &MediaSync{}
	}
	opts.Media.Client = opts.Client
	opts.Media.DryRun = opts.DryRun
	opts.Media.Uploads = state.Uploads
	if err := opts.Media.Load(ctx); err != nil {
		return summary, err
	}
	mainTypes := map[string]string{}
	if opts.MainTypesCSV != "" || (opts.Client != nil && opts.Client.URL != "") {
		if err := readCSV(ctx, opts.Client, opts.MainTypesCSV, "main_link_types", "", func(row map[string]string) error { mainTypes[row["id"]] = row["content"]; return nil }); err != nil {
			return summary, err
		}
	}
	sectors, err := loadSectorCatalog(ctx, repo.pool)
	if err != nil {
		return summary, err
	}
	rules := syncRules{PromoteOwnHash: opts.PromoteOwnHash, Sectors: sectors}
	users := []Transformed{}
	reserved := map[string]string{}
	seenIDs := map[string]bool{}
	err = readCSV(ctx, opts.Client, opts.UsersCSV, "users", "", func(row map[string]string) error {
		t, err := Transform(row, mainTypes)
		if err != nil {
			return err
		}
		if seenIDs[t.ID] {
			return errors.New("duplicate legacy user ID")
		}
		seenIDs[t.ID] = true
		if _, ok := reserved[t.Row.Key()]; ok {
			return errors.New("duplicate printed legacy identity")
		}
		reserved[t.Row.Key()] = t.ID
		users = append(users, t)
		return nil
	})
	if err != nil {
		return summary, err
	}
	w := csv.NewWriter(report)
	defer w.Flush()
	if err = w.Write([]string{"legacy_user_id", "legacy_hash", "profile_uuid", "status", "requested_username", "canonical_username", "username_status", "warning", "canonical_rule", "previous_canonical", "hash_claimed_by", "sector"}); err != nil {
		return summary, err
	}
	importedIDs := map[string]bool{}
	mediaFiles := map[string]bool{}
	for i := range users {
		t := &users[i]
		res, err := repo.syncUser(ctx, t, opts.Options, rules, opts.Media, reserved)
		if err != nil {
			return summary, fmt.Errorf("legacy user %s: %w", t.ID, err)
		}
		switch res.Rule {
		case CanonicalOwnHash:
			summary.OwnHash++
		case CanonicalOwnHashPromote:
			summary.OwnHashPromoted++
			if !opts.DryRun {
				opts.RouteCache.InvalidateProfile(ctx, uuid.MustParse(t.Row.TargetValue))
			}
		case CanonicalOwnHashPending:
			summary.OwnHashPending++
		}
		switch {
		case strings.HasPrefix(res.Sector, "unmatched:"):
			summary.SectorUnmatched++
		case res.Sector != "":
			summary.SectorMatched++
		}
		claimedBy := ""
		if res.Claim != nil {
			summary.HashClaims++
			claimedBy = res.Claim.String()
			if !opts.DryRun && opts.Notifier != nil && res.Claim.Notify() {
				if err = opts.Notifier.HashClaimed(ctx, *res.Claim); err != nil {
					res.Warning = joinWarning(res.Warning, "HASH_CLAIM_NOTIFY_FAILED")
				}
			}
		}
		summary.Profiles++
		summary.MediaReferenced += len(t.Media)
		for _, ref := range t.Media {
			mediaFiles[ref.Path] = true
		}
		if !t.Published {
			summary.Inactive++
		}
		switch res.Status {
		case "created":
			summary.Creates++
		case "updated":
			summary.Updates++
		case "skipped":
			summary.Skips++
		case "blocked":
			summary.Blocked++
		}
		if res.Status != "blocked" {
			importedIDs[t.ID] = true
			if res.Username == "conflict" {
				summary.Conflicting++
			} else {
				summary.Claimable++
			}
			state.Users[t.ID] = t.ContentHash
		}
		claimStatus := "claimable"
		if res.Username == "conflict" {
			claimStatus = "conflict"
		}
		if res.Status == "blocked" {
			claimStatus = "blocked"
		}
		if err = w.Write([]string{res.ID, t.Row.LegacyHash, t.Row.TargetValue, res.Status, t.Username, res.Canonical, claimStatus, res.Warning, res.Rule, res.Previous, claimedBy, res.Sector}); err != nil {
			return summary, err
		}
	}
	summary.MediaFiles = len(mediaFiles)
	knownHits := map[string]bool{}
	if opts.DryRun {
		rows, err := repo.pool.Query(ctx, `SELECT legacy_hit_id::text FROM legacy_sync_hit_ids`)
		if err != nil {
			return summary, err
		}
		for rows.Next() {
			var id string
			if err = rows.Scan(&id); err != nil {
				rows.Close()
				return summary, err
			}
			knownHits[id] = true
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return summary, err
		}
	}
	since := state.HitsSince
	if parsed, err := time.Parse("2006-01-02 15:04:05", since); err == nil {
		since = parsed.Add(-time.Second).Format("2006-01-02 15:04:05")
	}
	if opts.HitsCSV != "" || (opts.Client != nil && opts.Client.URL != "") {
		err = readCSV(ctx, opts.Client, opts.HitsCSV, "hits", since, func(h map[string]string) error {
			summary.HitsRead++
			_, idErr := strconv.ParseInt(h["id"], 10, 64)
			validType := h["type"] == "profile" || h["type"] == "main_link" || h["type"] == "social" || h["type"] == "contact"
			if idErr != nil || !validType || knownHits[h["id"]] {
				summary.HitsSkipped++
				return nil
			}
			date, err := time.Parse("2006-01-02 15:04:05", h["date"])
			if err != nil {
				summary.HitsSkipped++
				return nil
			}
			if date.Format("2006-01-02 15:04:05") > state.HitsSince {
				state.HitsSince = date.Format("2006-01-02 15:04:05")
			}
			if !importedIDs[h["user_id"]] {
				summary.HitsSkipped++
				return nil
			}
			if opts.DryRun {
				summary.HitsImported++
				knownHits[h["id"]] = true
				return nil
			}
			done, err := repo.syncHit(ctx, h, false)
			if err != nil {
				return err
			}
			if done {
				summary.HitsImported++
			} else {
				summary.HitsSkipped++
			}
			return nil
		})
		if err != nil {
			return summary, err
		}
	}
	summary.MediaUpload = opts.Media.Count
	summary.MediaMissing = opts.Media.Missing
	if err = w.Write([]string{"summary", "", "", fmt.Sprint(summary), "", "", "", "", "", "", "", ""}); err != nil {
		return summary, err
	}
	w.Flush()
	if err = w.Error(); err != nil {
		return summary, err
	}
	if opts.DryRun {
		return summary, nil
	}
	state.Media = opts.Media.Manifest
	state.Uploads = opts.Media.Uploads
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return summary, err
	}
	temp, err := os.CreateTemp(opts.StateDir, ".state-*")
	if err != nil {
		return summary, err
	}
	defer func() { _ = os.Remove(temp.Name()) }()
	if _, err = temp.Write(data); err != nil {
		_ = temp.Close()
		return summary, err
	}
	if err = temp.Sync(); err != nil {
		_ = temp.Close()
		return summary, err
	}
	if err = temp.Close(); err != nil {
		return summary, err
	}
	return summary, os.Rename(temp.Name(), statePath)
}
