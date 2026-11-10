# Profiles (owner side)

How an account's members create and edit profiles. The visitor-facing page, usernames, visible
modules and backgrounds are in [PUBLIC_PROFILES.md](./PUBLIC_PROFILES.md) and are not repeated.
Code: `backend/internal/modules/profiles` (routes, handler, `usecase/service.go`, `repository`),
UI in `frontend/src/features/profiles`. Scope rules: [TENANCY_AND_ACCESS.md](./TENANCY_AND_ACCESS.md).

## Who can do what

All routes act on the active account; rows of other accounts are 404.

| Action | Route | Needs |
|---|---|---|
| List, get, meta, vCard download/parse | `GET /v1/profiles`, `/{uuid}`, `/meta`, `/{uuid}/vcard`, `POST /vcard/parse` | `app.profiles.read` |
| Create, update, delete, set primary, upload avatar/cover, public page write | `POST /v1/profiles`, `PUT/DELETE /{uuid}`, `POST /{uuid}/primary`, `/avatar`, `/cover`, `PUT /{uuid}/public-page` | `app.profiles.write` **and** owner/admin account role |
| Create | `POST /v1/profiles` | also the `profiles.max` plan quota ([ENTITLEMENTS.md](./ENTITLEMENTS.md)); over quota is 403 `PLAN_LIMIT_EXCEEDED` |

## Data model

Migration 000026 (+ `account_id` from the tenancy migrations):

| Table | Content |
|---|---|
| `profiles` | `account_id`, `user_id` (creator, not owner), `primary_locale` (`tr` or `en`), `birthday`, `gender`, `avatar_url`, `cover_url`, `is_primary`, `completion_rate`, `deleted_at` (soft delete) |
| `profile_translations` | One row per locale: prefix, first/middle/last name, suffix, nickname, company, department, title, role, `bio`, `note`, `is_default` |
| `profile_addresses`, `profile_contact_details`, `profile_social_links` | Ordered lists (`display_order`, `is_primary`); contacts have `category` (phone, whatsapp, email, ...) and `type`; social links reference `social_platforms` |
| `profile_bank_accounts` | Bank accounts published on the public page (migration 000040) |

## Create and update

`POST /v1/profiles` and `PUT /v1/profiles/{uuid}` take one `UpsertProfileInput`: locale, birthday,
gender, avatar/cover URLs, `is_primary`, and the full lists `translations`, `addresses`,
`contact_details`, `social_links`.

- At least one translation is required, and the primary-locale (or default, or first) translation needs a first or last name.
- **Update replaces the lists**: translations, addresses, contacts and social links are deleted and re-created from the payload. Omit one and it is gone.
- Setting `is_primary` clears the flag on the account's other profiles; `POST /{uuid}/primary` does the same on its own. One primary profile per account.
- `completion_rate` (0-100) is computed on the server on every write; see [Completion score](#completion-score-and-suggestions-tec-549).
- Delete is a soft delete (`deleted_at`); the route cache is invalidated. A deleted profile stops counting towards `profiles.max`. The username route is not changed.
- Media cleanup (TEC-1176, KVKK): after the delete commits, the profile's objects are deleted from object storage, because the bucket serves anonymous GETs. Every profile delete goes through `Service.Delete` (owner route and the employee import rollback), so all paths are covered. See [Media](#media).

## Completion score and suggestions (TEC-549)

The score ("Profil Doluluğu") and the suggestions come from one rule table in
`backend/internal/modules/profiles/completion` (the only place the formula lives):

| Key | Points | Rule | Editor tab |
|---|---|---|---|
| `name` | 25 (15 with only first or last name) | first translation with a name | `identity` |
| `avatar` | 15 | avatar set | `media` |
| `title` | 10 | any translation has a title | `identity` |
| `company` | 10 | any translation has a company | `identity` |
| `phone` | 10 | a phone or WhatsApp contact with a value | `contacts` |
| `email` | 10 | an email contact with a value | `contacts` |
| `address` | 10 | the first address has a city | `addresses` |
| `social` | 10 | any social link | `social` |

- `GET/POST/PUT /v1/profiles…` and the avatar/cover uploads return `completion_rate` and
  `completion_suggestions` (`key`, `points` gained, `section` = editor tab, `i18n_key`), highest
  gain first. The score plus the open points is always 100. Responses recompute the score from the
  stored profile; `profiles.completion_rate` is what lists and sorting read.
- Avatar and cover uploads store the recomputed score (`SetCompletionRate`); before TEC-549 an
  avatar uploaded after save left the stored rate behind until the next edit.
- UI: `ProfileCompletionChecklist` on profile detail and above the editor. Each item links to
  `/profiles/{uuid}/edit?tab={section}`; the form opens that tab. Copy lives under
  `profiles.completion.suggestions.{key}` (a `pnpm test` check keeps it in step with the Go keys).
- To change a weight or add a rule: edit the table in `completion.go` (keep the total at 100),
  add the copy, and update this table. Existing rows keep their stored rate until their next write.

## Verified badge (TEC-549)

A platform-approved "verified profile" mark. Code: `backend/internal/modules/profileverifications`,
UI `features/profiles/components/profile-verification-card.tsx` (owner) and
`features/profile-verifications` (platform queue at `/platform/profile-verifications`).

| Action | Route | Needs |
|---|---|---|
| Status and latest request | `GET /v1/profiles/{uuid}/verification` | `app.profiles.read` |
| Request (optional `note`), withdraw a pending request | `POST`, `DELETE /v1/profiles/{uuid}/verification` | `app.profiles.write` **and** owner/admin account role |
| Review queue, one request | `GET /v1/platform/profile-verifications` (`status`, `q`), `/{uuid}` | `platform.profile_verifications.manage` |
| Approve; reject or revoke with a required `reason` | `POST /v1/platform/profile-verifications/{uuid}/approve`, `/reject`, `/revoke` | `platform.profile_verifications.manage` |

- Data (migration 000052): `profile_verification_requests`, one row per request with `status`
  (`pending`, `approved`, `rejected`, `revoked`, `cancelled`), `origin` (`owner` or `re_review`),
  `note`, requester, reviewer, `reviewed_at`, `review_reason`, `revoked_at`, `revoke_reason`.
  At most one open (pending or approved) request per profile (partial unique index).
- Profile status is the latest row: `pending` → pending, `approved` → verified, `rejected` →
  rejected, anything else (or no row) → none. `ProfileDetail.verification_status` carries it;
  the visitor API carries `verified`.
- Owner rules: profiles of other accounts are 404; a request is allowed only from `none` or
  `rejected` (409 otherwise). Rejected owners can fix the profile and ask again.
- **Identity changes.** When a verified profile changes its display name (first, middle or last
  name in any locale) or its organization (company name), `PUT /v1/profiles/{uuid}` revokes the
  approval (`revoke_reason` = `display_name_changed` and/or `organization_changed`, revoked by the
  editor) and opens a `re_review` pending request in the same transaction. The badge disappears at
  once and the profile goes back to the queue. Casing, extra spaces, Turkish dotted/dotless i, a
  new locale with the same name, and title/bio/contact edits do not count. The revoke runs before
  the profile write: if the write then fails, the badge stays revoked (fail closed). Bulk edits and
  imports go through the same service (also in the worker), so they behave the same.
- An admin revoke ends in `none` (no re-review); the owner may request again.
- Notifications (templates seeded by 000052, tr/en) go to the account's owners and admins:
  `profiles.verification.approved` and `.rejected` (in-app + email, with the reason),
  `.revoked` (in-app, admin reason) and `.re_review` (in-app, identity change).
- Grant: `super_admin` gets `platform.profile_verifications.manage` in the migration; give it to
  other platform roles from the roles screen.

## Media

- Avatar and cover are uploaded as multipart (`avatar` / `cover` fields; the form is parsed with a 5 MB limit) to `POST /v1/profiles/{uuid}/avatar|cover`; the API stores the object under the profile's prefix and writes the URL. The browser never builds storage keys.
- `avatar_url` / `cover_url` in an upsert payload are validated: empty, a staged temp avatar (`tmp/avatars/…`, produced by the vCard import), or media under this profile's own prefix. Anything else is 400. On create/update a staged avatar is copied to the permanent key and the temp object deleted.
- The vCard embeds the profile's own avatar read from object storage (capped size); it never fetches remote URLs.
- Deleted profiles (TEC-1176): `Service.Delete` collects the keys the row points at (avatar, cover, cached vCards) before the delete and, after it commits, schedules `app:profiles:media_cleanup` on the `maintenance` queue (10 retries, 30 s delay so a late vCard cache write is caught, one task per profile). With `QUEUE_ENABLED=false`, or if the enqueue fails, it runs inline, best effort. Code: `profiles/usecase/media_cleanup.go`.
  - Deleted: every object under `profiles/{uuid}/` (avatar, cover, old extensions, vCards, copied company logos, legacy media) and a staged `tmp/avatars/` object the row still pointed at.
  - Kept: company media library files (`accounts/{uuid}/media/`, owned by the company) and any key a live row still references (`CountStorageKeyReferences`: another profile's avatar/cover, `company_media`, background thumbnails). A later run deletes it once the reference is gone.
  - The job skips a profile that is still live and is idempotent (objects already gone are fine). Failures are logged with the request id and the profile uuid (no personal data), make Asynq retry, and never undo the delete.
  - Profiles deleted before TEC-1176 (TEC-1177): run the one-off [backfill](#backfill-for-profiles-deleted-before-tec-1176-tec-1177) once.

### Backfill for profiles deleted before TEC-1176 (TEC-1177)

`/app/profile-media-backfill` (backend image, `backend/cmd/profile-media-backfill`) walks every soft-deleted profile (`profiles.deleted_at IS NOT NULL`) in id order, collects its keys the same way `Service.Delete` does plus the `profiles/{uuid}/` listing, and applies the same reference check. Output is counts and ids only (logs carry profile uuids), no personal data.

- Default is a **dry run**: it prints the number of deleted profiles, candidate objects, deletable objects and objects skipped by the reference check, and deletes nothing.
- `-apply` enqueues `app:profiles:media_cleanup` for each deleted profile that still has a deletable object (maintenance queue, TaskID `profile-media-cleanup:{uuid}` dedupes against a queued task); the worker deletes the objects. With `QUEUE_ENABLED=false`, or with `-inline`, it cleans up in the process instead.
- `-batch-size N` (default 200) rows per query; `-limit N` stops after N profiles; `-after-id ID` resumes after the `last_id` a previous run printed. Safe to run twice: once the cleanup finished, a run enqueues and deletes nothing (only objects a live row still references stay as `skipped_referenced`).
- Exit status is 1 when any profile failed (see the logs); run it again, it only retries what is left.

In production (Dokploy: open a terminal on the backend container and run the same `/app/…` commands), on the host:

```sh
docker compose --env-file .env.server -f compose.prod.yml exec backend /app/profile-media-backfill          # dry run
docker compose --env-file .env.server -f compose.prod.yml exec backend /app/profile-media-backfill -apply   # enqueue
docker compose --env-file .env.server -f compose.prod.yml exec backend /app/profile-media-backfill          # check: deletable=0 once the worker is done
```

Locally: `cd backend && go run ./cmd/profile-media-backfill [-apply]` with the repo-root `.env` loaded.

## vCard

- `GET /v1/profiles/{uuid}/vcard?locale=&version=` generates the owner's vCard from the stored translations, contacts, addresses and social links. Generated files are cached in object storage per locale and re-built in the background after every update/avatar change.
- `POST /v1/profiles/vcard/parse` reads an uploaded `.vcf` and returns fields for the create form to autofill (`ParsedVCardProfile`); nothing is saved. The UI entry is `vcard-import-dialog.tsx`.
- The public vCard (`/v1/public/p/{username}/vcard`) honours visible modules; see PUBLIC_PROFILES.md.

## Public page, modules, background

Edited from profile detail, "Public page" (`GET/PUT /v1/profiles/{uuid}/public-page`). `visible_modules`,
`bank_account_uuids`, `enabled` and `background_uuid` are stored on the profile's canonical route
(`presentation_config`) and in `profile_bank_accounts`, not on `profiles`. Details: PUBLIC_PROFILES.md.
Creating a profile also creates its username route (`ensureUsername`); if that fails, the next
`GET .../public-page` retries it.

## Caches

Create-time username aside, update, avatar upload and delete call `routeCache.InvalidateProfile`, so
`/c/{code}`, `/s/{slug}` and `/{username}` resolution reflects the change without waiting for the 1h TTL.

## Not implemented

Profile modules beyond the five public ones (files, catalog: TEC-551), lead capture (TEC-552),
profile duplication, and moving a profile to another account.
