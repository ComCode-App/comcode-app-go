# Legacy migration CLI (COM-560 / formerly TEC-560)

`sync` repeatedly imports the verified v1 MariaDB export into account-owned profiles.
The older normalized `import` / `verify` commands remain available for reviewed existing
targets. Sync never creates login users, copies passwords, sends invitations, or changes DNS.
Apply migration 000051 before syncing. It permits migration-owned profiles/bank accounts
without a creator user and adds durable identity/content mappings and hit-ID deduplication.

## Repeatable sync until cutover

Run from `backend/`, explicitly load the canonical repo-root environment. Use a privileged
migration database identity and the normal `STORAGE_DRIVER=s3`, `S3_*` backend configuration.
Keep state, reports, downloaded source and tokens private. The default `./var/legacy-sync`
is gitignored. Never run two sync processes for the same source/database concurrently.

```sh
go build -o bin/migrate-legacy ./cmd/migrate-legacy
export LEGACY_EXPORT_URL=https://comcode.co/aktarim-RANDOM/export.php
# Set LEGACY_EXPORT_TOKEN from your password manager; do not put it in shell history.
bin/migrate-legacy sync --dry-run --report /private/tmp/legacy-plan.csv
bin/migrate-legacy sync
# Repeat the same command any number of times until cutover.
```

Offline input (plain CSV or gzip; unnormalized users table export):

```sh
bin/migrate-legacy sync --users-csv /secure/users.csv.gz \
  --main-link-types-csv /secure/main_link_types.csv.gz \
  --hits-csv /secure/hits.csv.gz --media-dir /secure/extracted-media \
  --state-dir ./var/legacy-sync --dry-run --report /private/tmp/offline-plan.csv
# Remove --dry-run and supply a fresh --report to apply.
```

Extract the owner's `comcode-legacy-media-2026-10-19.zip` outside the repository first.
Both `public_html/uploads/...` and `uploads/...` layouts work; likewise `usercard`.
Symlink media is rejected. No extracted directory means referenced media is reported
missing unless an online exporter is configured. Missing media does not block profiles;
review `MediaMissing` and rerun with the complete source before cutover.

Flags: `--export-url`, `--export-token` (prefer env), `--state-dir`, `--users-csv`,
`--hits-csv`, `--main-link-types-csv`, `--media-dir`, `--match-owners`, `--account`,
`--dry-run`, `--report`, `--promote-own-hash`. `--match-owners` uses the existing active personal-account
email match; fallback is the platform-owned `legacy` holding account. Mapped profiles
retain their current account and creator on every subsequent sync, including after `owners`.
No end-user access is granted to the holding account. Owner email is private provenance.

`--dry-run` uses read-only profile transactions, fetches source/manifest, and writes only
its explicitly selected CSV report and adjacent `.summary.json`. It does not create
state directories or upload media. Reports refuse overwrites and use mode 0600. Summary
counts include profile creates/updates/skips/blocked, username claimability/conflicts,
inactive profiles, media references/uploads/missing, historical hits, own-hash usernames
(new/promoted/pending), hash claims and sector matches. Report columns: `legacy_user_id`,
`legacy_hash`, `profile_uuid`, `status`, `requested_username`, `canonical_username`,
`username_status`, `warning`, `canonical_rule`, `previous_canonical`, `hash_claimed_by`,
`sector`. No column carries an email.

Database `legacy_sync_users` is authoritative for identity and content hashes. Local
`state.json` records matching hashes, media manifest, upload cache and last hits timestamp.
The state file is written atomically only after success; profile transactions and hit-ID
ledger allow safe retries after partial failure or loss of state. State is bound to its
database. Do not copy it between databases. A failed run may have committed earlier
profiles; rerun the same source. Sync does not delete users absent from a later snapshot.
Changed sync-owned profile lists are replaced; do not edit imported profiles in ComCode
while still treating legacy as authoritative. Media is deduplicated by SHA1 per profile,
using deterministic API-owned object keys; changed bytes get a new key, protecting cached
URLs. Old unused objects are retained for later reviewed cleanup.

### Mapping decisions

- One deterministic profile UUID per legacy `user_id`. `uniqid_id` is immutable printed
  identity; changing it fails. Usernames are claimed when valid, unreserved, free and not
  another source user's printed hash. Conflicts use `legacy-<profile UUID without hyphens>` canonical
  paths and are reported; original hash routes still reach the profile. Existing canonical
  names are retained on later syncs. Legacy routes are created once; only status changes.
- `comcode` stays a reserved word, but the card printed with that hash is not blocked: its
  legacy route takes `/comcode` and opens its own profile (`publicpath.IsLegacyBound`,
  TEC-955). Its username stays `legacy-<uuid>` because `comcode` is not claimable.
- Username = own printed hash (TEC-956 option E): the hash becomes the canonical username
  (`canonical_rule=own_hash`) and the legacy route lives at its alias `r/legacy/{hash}`. The
  resolver renders `/{hash}` in place instead of redirecting a card to its own username.
  Profiles imported before this rule keep `legacy-<uuid>` (`own_hash_pending`) until a run
  with `--promote-own-hash` (`own_hash_promote`): the old name is retired like a panel rename
  (it redirects; `route_history`), the legacy route moves to its alias (the only path move
  migration 000066 allows) and the route cache is evicted, so a real run needs `REDIS_ADDR`
  (+ `REDIS_PASSWORD`, `REDIS_DB`, `APP_ENV`). Only the sync's own fallback is replaced,
  never a name chosen in the panel. Preview with `--dry-run --promote-own-hash`.
- The printed card wins (TEC-955): every printed hash is taken for other cards' usernames
  and, once imported, for panel usernames (`PathOwner` checks the root and the alias). A
  route that held a hash before its card was synced is listed in `hash_claimed_by`
  (`route_type:status:profile_uuid`); the card is served there through its alias. Library
  callers may pass `SyncOptions.Notifier` to ask live username owners to rename; it is a
  panel (in-app) hook only, never email, and is not wired in this CLI.
- Sector icons (TEC-955): `ikon2` (then the other company image fields) is matched to the
  TEC-571 catalog by slugified file name against the sector slug, Turkish and English names
  (`teknoloji.png`, `İnşaat.png`) plus a few aliases (`bilisim`, `yazilim`, `danismanlik`,
  `nakliye`, `saglik-hizmetleri`). The match sets `profiles.sector_id` only when empty, so a
  later choice survives; unchanged rows are linked too. An unmatched `ikon2` stays empty and
  is reported as `unmatched:<file>` / `SECTOR_UNMATCHED`.
- vCard names/title/company, description, contact details, addresses, buttons and main
  action populate existing profile lists. Platform handles map to URLs; unknown keys with
  valid HTTP(S) URLs become labelled generic links. Unsafe/unresolved values are retained
  as inactive links for review. Blank values are omitted. Login email stays private unless
  explicitly present in public vCard contact fields. Double-encoded cp1252/latin1 text and
  HTML entities are repaired conservatively. Bank accounts use the existing bank module.
- `deactivated=1` is treated as inactive, `2` as enabled; `active=0` is also inactive.
  The verified dataset contains 20/1719 respectively (all `active=1`). This is an inference
  from the legacy flag name/distribution; confirm representative cards with the owner.
  Both routes are locked and public-page enabled=false, yielding the existing inactive
  card behavior. Printed identities remain registered.
- Avatar becomes avatar; custom `usercard/*` front becomes cover. Back and company images
  are stored under the profile prefix and their API-owned URLs retained in private sync
  metadata: gallery/files modules are not implemented. `logo4` without an image extension
  remains text. Shared template images are never uploaded per profile. Legacy one/two/three
  layouts/background values are retained in metadata. Layout two maps to compact_card/compact;
  one/three map to classic_light/standard. Valid hex-color backgrounds map to account-owned
  solid Background Studio documents; other backgrounds use the default. `vcard_*` translations stay in metadata and produce report warnings (TEC-550).
- Historical hits populate `event_aggregates_daily` per profile/date with explicit
  `legacy.profile.view`, `legacy.main_link.click`, `legacy.social.click`,
  `legacy.contact.click` event types, separate from modern event semantics. No raw events,
  IPs, sessions or inferred uniques are inserted. The ledger stores only source hit IDs.
  Orphan/invalid hits are skipped and counted. Source dates retain the legacy calendar day. CSV reports identify the source ID/hash,
  deterministic profile UUID, requested/canonical usernames and per-row warnings.
  Online incremental reads overlap one second to handle tied timestamps; old/backdated
  rows require a periodic full offline hits replay (the ledger prevents double counts).
  Do not combine histories from different legacy installations in one mapping database.

### Cutover day checklist

1. Keep exporter available through rehearsals; inspect every warning/conflict/missing asset.
2. Stop legacy writes, take a final users/media snapshot and full hits export, run final sync.
3. Confirm inactive cards, claimed usernames, representative Turkish text/vCards and media.
4. Verify resolver behavior and real printed URLs at the deployed edge, with the owner.
5. Invalidate affected route/profile/vCard caches (CLI does not connect to Redis), or wait
   their configured TTL; check DNS/TLS and redirects before accepting traffic.
6. Switch traffic only after review; keep a rollback snapshot and private run reports.
7. Delete exporter/random FTP folder and external hash cache, revoke token/DB reader,
   and verify exporter URL returns 404. Cutover is not performed by this command.

## Reversible owner assignment (COM-560)

Apply migration **000053_legacy_owner_assignments** before running `owners` (000052 is
reserved by another change). Run after sync with the same unnormalized users export.
Only mapped, non-deleted profiles still in the verified holding account (staff members allowed)
are candidates. Inactive/deactivated users stay in holding; blocked/unmapped source rows
are reported as skipped. Already assigned profiles are skipped on repeated runs. New
source users enter holding through sync and can be assigned by a later owners batch.

```sh
bin/migrate-legacy owners --users-csv /secure/users.csv.gz --dry-run \
  --report /secure/owners-plan.csv
bin/migrate-legacy owners --users-csv /secure/users.csv.gz \
  --overrides /secure/owners-overrides.csv --report /secure/owners-applied.csv
# Online: omit --users-csv and use LEGACY_EXPORT_URL / LEGACY_EXPORT_TOKEN.
bin/migrate-legacy owners rollback --batch <batch-uuid>
bin/migrate-legacy owners rollback --all
```

Reports require a fresh path, use mode 0600, and include private login emails, group keys,
classification and skip reasons. JSON stdout contains the batch UUID and counts. Dry-run
uses a read-only transaction: it produces the same classification report and counts as
apply, with no batch UUID and no database/state/storage writes. An apply is one database
transaction, serialized with sync/import/rollback by the existing advisory lock. Report
write errors abort apply. Invalid candidate login emails or duplicate source IDs fail
before any assignments commit. Treat reports and summaries as private.

The classifier repairs text with the existing sync repair, then picks the first usable
key: work-email domain brand, login-email brand, normalized organization, work-URL brand.
It follows the reference free-mail/block lists, suffix stripping, Turkish folding, legal
and sector stopwords. A brand group with at least four named users splits when removing
shared organization tokens yields two subgroups of at least three users. Remaining users
join the largest subgroup (ties follow source ID order). Organization-keyed groups merge
into a brand group with the same dominant normalized organization. Groups of two or more
become companies; singletons become individuals. The reference dump counts are a review
baseline, not an assertion for arbitrary exports; skipped inactive rows affect totals.

Overrides apply after both refinements and never unskip inactive/unmapped/assigned rows:

```csv
legacy_user_id,group
123,solo
124,holding
125,reviewed-company
126,reviewed-company
```

`solo` forces an individual, `holding` skips, and any other exact key forces a group.
Duplicate/unknown override IDs and blank values are rejected. IDs sort numerically when
numeric, otherwise lexically, giving deterministic ties and company-owner selection.

New users have normalized login emails, repaired vCard names (legacy name split fallback),
status pending, unverified email and an Argon2id hash of an unretained random 32-byte
secret. No notifications, email or invitations are emitted. Non-deleted users with a
matching email are reused without changing their status, credentials or roles. Created
users get the same platform role as self-signup (`auth_settings.default_role_id`, else
`subscriber`); every run also backfills created users from earlier batches that lack a
role (`roles_assigned` in the summary). Rollback removes those roles with the user. Individuals
reuse signup's trigger-created personal account and owner membership. Companies get a
unique `legacy-<UUID>` slug, dominant repaired organization name (group key fallback),
active status and NULL sector. New company members' empty trigger-created personal
accounts are removed within the same transaction; reused users' personal accounts remain.
Exactly one membership is owner: lowest source ID matching a leadership title, otherwise
lowest ID; others are members. Duplicate legacy logins share a user/membership. A reused
company-only user assigned as an individual gets a personal account using signup’s same
SQL naming, slug and membership rules. Ambiguous existing case-insensitive email matches
fail the batch.

This privileged offline repository path bypasses app plan quotas intentionally, like sync.
It does not grant members general app write permissions or bypass account role checks in
the application. Remaining creator fields are repaired only when NULL or a holding member.

### Ownership table audit

| Tables | Assignment behavior and reason |
|---|---|
| `profiles` | Move each mapped holding profile's `account_id`; repair `user_id` acting creator. |
| `routes` | Move holding routes linked by `profile_id`, including canonical usernames, legacy identities and profile/vCard smart-link targets; repair `owner_user_id` when unset/holding-only. |
| `user_bank_accounts` | Move holding bank rows linked through `profile_bank_accounts`; repair `user_id`. Sync creates these rows with deterministic UUIDs per legacy user/index. |
| `backgrounds` | Move only the deterministic imported background UUID for that legacy user. Repair unset/holding-only `created_by`; gallery presets and unrelated saved designs stay put. |
| `profile_translations`, `profile_addresses`, `profile_contact_details`, `profile_social_links`, `profile_bank_accounts` | No `account_id`; ownership follows the existing profile or bank FK. vCards are generated from profile data, with no separate owned vCard table. |
| `events`, `event_aggregates_hourly`, `event_aggregates_daily`, `event_processing_log` | No account ownership column; subjects retain the same profile/route UUIDs. Sync historical aggregates remain attached without rewriting counts. |
| Storage metadata (`storage_*`) and object bytes | No account ownership column for imported media; API-owned profile object keys and URLs remain unchanged. Profile metadata retains media references. |
| `route_history`, `leads`, `nfc_cards`, `orders`, `order_notification_events` | Sync creates none. Historical audits, visitor leads, manufactured cards and commerce data are not legacy import assets and are deliberately left. |
| `subscriptions`, `payment_checkout_sessions`, `payment_saved_cards`, `payment_card_sessions`, `payment_provider_customers` | Billing belongs to the existing account; legacy sync creates none. No billing/entitlement migration. |
| `bulk_jobs`, `import_jobs`, `company_offices`, `company_groups`, `company_member_assignments`, `account_member_scopes`, `company_member_profiles` | Not created by legacy sync; account workflows and company hierarchy stay unchanged. |
| `accounts`, `account_members`, `users`, `refresh_tokens.active_account_id` | Create/reuse only the explicit target resources; retain holding and existing memberships/session selection. Never transfer sessions. |
| `legacy_sync_users`, `legacy_sync_hit_ids`, `legacy_owner_batches`, `legacy_owner_items` | Mapping/deduplication/audit records stay unchanged in identity; journal survives resource deletion. No resource FKs on journal identifiers, intentionally. |

### Rollback and conflicts

Each batch stores one item per assigned profile, resource creation flags and a JSON list
of every `{table, pk, column, old, new}` change, including creator changes. Rollback runs
as one transaction and conditionally restores only values still equal to the journal's
new value. Missing or subsequently edited rows are reported as conflicts and preserved. It then deletes created memberships/accounts and only deletes
created users whose status is still pending, last login is NULL, and memberships are gone.
Reused users/accounts/memberships are never deleted.

Accounts with remaining data or additional memberships are **retained and reported**,
along with their created memberships/users. The cleanup guard checks every live public
table with `account_id`, including future tables. This prevents cascading deletion of
later non-legacy data. `--force` explicitly allows deletion of created accounts containing
later data; row-conflict accounts remain protected even with force. Database FK restrictions
can still refuse force; an error aborts the whole transaction. A logged-in or activated
created user survives even if its created account is removed. Resolve retained resources
manually after review; a completed rollback is marked `rolled_back`, records its outcome,
and subsequent rollback is a no-op. `--all` processes newest batches first in one transaction.

Sync still updates mutable imported content in the current assigned account; it never
moves profiles back to holding or recreates mapped profiles. New linked bank/background
rows created by a later changed sync are not retroactively journaled: they cause account
retention on rollback rather than unreviewed deletion. Pause legacy edits for a clean
apply/rollback rehearsal and take a database backup before production operations.

## Input contract

UTF-8 CSV (RFC 4180, optional BOM) with exactly this header/order, or `.jsonl` with
one JSON object per line (max 4 MiB). Unknown JSON fields are rejected.

```csv
legacy_hash,target_type,target_value,owner_email,owner_name,created_at,extra_json
Abc560,url,https://example.com/card,owner@example.com,Example Owner,2020-01-02T03:04:05Z,"{""old_card_id"":560}"
```

```json
{"legacy_hash":"Abc560","target_type":"url","target_value":"https://example.com/card","owner_email":"owner@example.com","owner_name":"Example Owner","created_at":"2020-01-02T03:04:05Z","extra_json":{"old_card_id":560}}
```

The Go contract is `internal/platform/legacymigration.Row`:

| Column | Meaning |
|---|---|
| `legacy_hash` | Exact original single ASCII path segment, 1–119 chars, letters/digits plus `.`, `_`, `-`; no slash, whitespace or URL encoding. Source case is kept in provenance; lookup follows the existing case-insensitive resolver. Case variants with different source rows are conflicts. Unsupported hashes require operator investigation, never a silent rename. |
| `target_type` | `profile`, `url`, `vcard`, `smart_link`, `unknown` |
| `target_value` | `profile`/`vcard`: existing profile UUID with active public username in mapped account. `url`: absolute HTTP(S) URL, no embedded credentials. `smart_link`: existing active `s/slug` in mapped account; snapshots its current profile/vcard/URL target, not a dynamic chain or presentation/passcode settings. `unknown`: empty. |
| `owner_email` | Optional; active user matching only with `--match-owners`. Case-insensitive, ambiguous matches fail. |
| `owner_name` | Optional provenance only; never creates/renames a user. |
| `created_at` | Optional RFC3339 source time, used as route creation time; empty uses import time. |
| `extra_json` | JSON object of source details; empty CSV cell/omitted JSON field becomes `{}`. Do not include passwords/payment secrets. |

Unknown targets are stored as `reserved/custom_modules` with no destination. Their
verification always fails. Mapping these immutable source records requires a separately
reviewed remediation; do not import unresolved rows into production casually.

## Commands

Run from `backend/`. Load `DATABASE_URL` and `PUBLIC_FRONTEND_URL` from the canonical
**repo-root** `.env` into your shell explicitly (the CLI never loads a backend `.env`).
Use an administrative migration DB identity; this is an offline platform operation,
not an app API. These commands do not push, merge or deploy anything.

```sh
go build -o bin/migrate-legacy ./cmd/migrate-legacy
bin/migrate-legacy import --input normalized.csv --dry-run \
  --report dry-run.csv --batch-size 500 --batch-id legacy-review-001 \
  --frontend-url https://comcode.co
bin/migrate-legacy import --input normalized.csv --report import.csv \
  --batch-size 500 --batch-id legacy-production-001 --frontend-url https://comcode.co
bin/migrate-legacy verify --input normalized.csv --report verify.csv \
  --frontend-url https://comcode.co
# Existing owners: use the same ownership flags for import AND verify.
bin/migrate-legacy import --input normalized.jsonl --match-owners \
  --account holding --report owners-import.csv --frontend-url https://comcode.co
bin/migrate-legacy verify --input normalized.jsonl --match-owners \
  --account holding --report owners-verify.csv --frontend-url https://comcode.co
# Or attach all unmatched rows to an explicitly reviewed existing account UUID:
# --account 00000000-0000-0000-0000-000000000000
```

`--format csv|jsonl` overrides file extension. Reports are mandatory, created with
mode 0600, and **never overwritten**. Do not reuse report paths. Samples show only
the first three hashes/types/targets; treat input, reports and stdout as private.

Default `--account holding` lazily creates the platform-owned company account
`legacy` / `Legacy holding` with fixed UUID `a5b661de-d20d-5d62-a29a-14e262f84e05`.
No end user can access it through account membership. A conflicting slug/UUID,
suspended account or unexpected membership causes an error. `--account UUID` uses
an existing active account. `--match-owners` first matches an active user by email
to that user's active **personal** account; absent matches fall back to `--account`
and produce `OWNER_UNMATCHED`. Never guesses a company account or creates a user.
Profile/link targets must belong to the selected account (holding cannot reference
a customer's profile); choose the correct account or owner matching first.

Imports stream bounded batches of 1–10000 rows. Each batch is atomic and importers
serialize with an advisory transaction lock. A later failure retains earlier committed
batches; rerun the identical input to resume. The summary shows committed rows only.
`--dry-run` uses read-only transactions, does not insert an account or route, and
prints a summary/sample plus its report. Review `warnings` even on exit code 0.

Insert-if-absent is the immutable upsert policy: reruns skip identical source rows,
retain original batch/timestamps/ownership and do not mutate any route. Different
source data for an existing hash is an error, never an overwrite. `legacy_metadata`
stores `source_row`, exact `original_hash`, printed URL, import batch/time and collision
storage details. Provenance stays in account-scoped management data and is omitted from
the unauthenticated public resolver response. `extra_json` is nested inside `source_row`,
never flattened over metadata.

## Collision preservation and cutover

Normal hashes occupy the original root path. Existing username/profile or slug
collisions use the deterministic **internal** storage key `r/legacy/{hash}`.
Neither the printed URL nor the existing profile route is renamed or deleted.
Occupied non-legacy fallback keys abort the batch. Each imported collision gets
an operator warning with its original hash and storage key in the CSV report.

**blocked: collides with app path** is a separate operator report section/status.
Reserved words and static app paths (`login`, `leads`, `orders`, `account`, etc.)
are blocked in both dry-run and real imports, including repeated input rows.
No route or holding account is created for these rows. The CSV lists each blocked
hash with this status and ends with a named section containing its count; stdout
also prints the section/count. Review these cases individually with the owner;
the tool never silently imports them, renames their printed URLs, or intercepts
static pages. `verify` fails blocked rows. Import exit code 0 means processing
finished, not that all rows are safe for cutover: inspect blocked/warning counts.

`Service.Resolve(hash)` (also HTTP `GET /v1/r/{hash}`), already used by the existing
frontend `[username]` page via its existing `/public/p/{username}` call, checks the
internal legacy key on root cache misses. The backend returns a no-store 307 with
Location for legacy destinations; the page follows it without a second backend call.
Alias lookups use the normal route cache; warm normal paths and warm aliases add
no Postgres round trip. Explicit `c/...` and `s/...` keep their existing meaning.
The frontend proxy retains its original auth export and matcher; static app pages
do not call the legacy resolver. A legacy collision can shadow an existing profile:
give the displaced profile a new canonical username before cutover. A profile/vcard
target with a shadowed username and self-references to the printed URL are rejected.

Reserved-word validation applies to new usernames and renames, not reads of existing
rows or retaining the current username. Adding a reserved word does not revoke an
existing username in the backend. A frontend static page at that path still owns
the browser URL; handle such historical cases individually before cutover.

This preparation does not change production DNS/cPanel routing. The verifier proves
DB resolver behavior, not DNS, TLS, redirects on another host, external availability
or completeness. Verify real printed URLs through the deployed edge before cutover.

`verify` invokes the same in-process resolver used by the HTTP handler, without cache
or analytics side effects. It independently derives expected targets from input and
checks legacy route type, active/unlocked status and destination. CSV columns:
`hash,expected,actual,status`. Any missing/broken/unknown target returns non-zero.
It reads every row, including repeated hashes. Do not edit normalized input to make a
bad mapping appear correct. This tool does not invalidate Redis. Before cutover, evict the affected bare-hash
route cache keys (`app:<env>:route:path:<hash>`) using the normal operational Redis
access, or allow their configured TTL to expire (default one hour). An already
cached profile root remains cached until invalidation/expiry; after that the legacy
alias wins. Alias entries retain normal path/card/profile invalidation behavior.
Missing paths are not negative-cached.

## Tests

`TEST_DATABASE_URL` must point at a disposable real Postgres with create-database
privileges; pgtest applies the normal migrations in isolated package databases.

```sh
TEST_DATABASE_URL=postgres://test:test@127.0.0.1:PORT/postgres?sslmode=disable \
  go test ./internal/platform/legacymigration ./cmd/migrate-legacy
```

Türkçe kısa özet: Doğrulanmış users/hits verisini tekrarlanabilir `sync` ile alın;
normalize mevcut hedefler için `import`/`verify` komutları korunur. Dry-run raporunda `blocked: collides with app path` bölümünü ve sayısını
inceleyin; rezerv/sabit uygulama yolları içe aktarılmaz, sahibiyle tek tek kararlaştırılır.
Kullanıcı adı çakışmaları mevcut public resolver üzerinden legacy alias'a gider.
Geçişten önce etkilenen kök route cache kayıtlarını silin veya TTL dolmasını bekleyin.
Aynı sahiplik bayraklarıyla import ve verify çalıştırın; bloke/bilinmeyen hedef veya
doğrulama hatası varken DNS geçişi yapmayın.
