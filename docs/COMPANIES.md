# Companies

A company is an account of type `company`. It owns profiles, cards, smart links, bank accounts,
orders and a subscription exactly like a personal account does; members work inside it by
switching their active account. The account model, roles table, token claims and the fail-closed
scope are in [TENANCY_AND_ACCESS.md](./TENANCY_AND_ACCESS.md) and are not repeated here. Code:
`backend/internal/modules/accounts` (routes, use cases), `internal/platform/authctx`
(`Account` helpers), UI in `frontend/src/features/accounts` and `frontend/src/features/companies`.

## Personal vs company

| | Personal account | Company account |
|---|---|---|
| Created | By a DB trigger when the user is created | By a platform admin (`POST /v1/platform/accounts`) |
| Members | Exactly one, its user, as `owner`; adding members is rejected (`ErrPersonalAccount`) | Any number, roles `owner` / `admin` / `office_manager` / `member`; always at least one `owner` |
| Name | Follows the user's name; `PATCH /v1/account` is rejected | Renamed by owner/admin |
| Slug | Generated | `slug.FromName(slug or name)`; unique, a clash is 409 |
| Billing | Its user is the owner, so can subscribe | Owners only (checkout, cards, subscribe, cancel); admins and members can read the subscription |
| Status | `active` / `suspended` | Same; set by the platform |

Everything else (quotas, scoping, 404 for other accounts' rows, write = permission plus owner/admin)
behaves the same. Each account has its own plan, its own usage counts and its own subscription,
so a company on the free plan is limited independently of its members' personal plans
([ENTITLEMENTS.md](./ENTITLEMENTS.md), [BILLING.md](./BILLING.md)).

## Who can do what (member management)

All app routes act on the active account and are checked by account role, not by an `app.*`
permission; they need a session only.

| Action | Route | member | admin | owner |
|---|---|:-:|:-:|:-:|
| View account, members, plan and usage | `GET /v1/account`, `/members`, `/entitlements` | yes | yes | yes |
| Rename | `PATCH /v1/account` | no | yes | yes |
| Add a member (existing user, by email) | `POST /v1/account/members` | no | `admin`, `member` | any role |
| Invite any email, list/resend/revoke invites | `/v1/account/invites` (see below) | no | every role but `owner` | any role |
| Change a role | `PATCH /v1/account/members/{user_uuid}` | no | between `admin` and `member`; cannot touch an owner or grant `owner` | any |
| Remove a member | `DELETE /v1/account/members/{user_uuid}` | self only (leave) | non-owners, and self | any, and self |

Constraints that hold for everyone, including the platform:

- The last `owner` cannot be removed or demoted (DB check, surfaced as `ErrLastOwner`).
- A personal account never gets another member.
- Adding a user who is already a member is a conflict.

## Platform management

`/v1/platform/accounts/*`, permission `platform.accounts.read` / `.write`, not account-scoped:

| Route | Purpose |
|---|---|
| `GET /v1/platform/accounts`, `/meta`, `/{uuid}` | List (filter by `q`, `type`, `status`), detail with members |
| `POST /v1/platform/accounts` | Create a company: `name`, optional `slug`, required `owner_user_uuid` (becomes `owner`) |
| `PATCH /v1/platform/accounts/{uuid}` | Change `name` and/or `status` (`active`, `suspended`) |
| `POST/PATCH/DELETE /v1/platform/accounts/{uuid}/members[/{user_uuid}]` | Add, change role, remove, with any role and no account-role check; the data invariants above still apply |

Company accounts are indexed for the platform Cmd+K palette under `companies`. Member and account
changes are written to the activity log (`accounts.updated`, `accounts.member_added`, ...).

## Suspension

Suspending an account (`status = suspended`):

- switching to it is refused (`POST /v1/auth/accounts/switch` returns 403 "Account is suspended");
- the active account is resolved on every request (`resolveAccountByUUID`): a member whose active
  account is suspended falls back to their personal account immediately, so suspension is not
  delayed by token lifetime;
- its public profile pages answer 404 (`profiles/usecase/public_page.go`).

## Joining a company

Two ways in:

- **Add directly.** An owner/admin (or a platform admin) adds a user who **already has an
  account** by email (`POST /v1/account/members`); no email is sent and there is no acceptance
  step. The next time that user loads `/v1/auth/accounts` the company appears.
- **Email invite (TEC-945).** An owner/admin invites **any** address with a role. The person gets a
  link to `/platform/invite/{token}`, valid 7 days. A registered person logs in and accepts; an
  unregistered person signs up from the link (`POST /v1/auth/register` with `invite_token`, which
  works for the invited address even while self-registration is closed and marks the email
  verified) and accepts. Accepting needs the session's email to equal the invited address.

| Route | Who | Behavior |
|---|---|---|
| `GET /v1/account/invites` | `app.account_invites.read` + owner/admin | Paginated, newest first. No `status`: open invites (pending and expired). `status=pending\|expired\|accepted\|revoked`, `q` matches the email. |
| `POST /v1/account/invites` | `app.account_invites.write` + owner/admin | `email`, `role` (default `member`; `owner` only by owners). An existing member or an open invite for the address is 409. Sends the email. |
| `POST /v1/account/invites/{uuid}/resend` | same | New token (the old link stops working), new 7-day window, email again. Works on expired invites; accepted/revoked are 409 `INVITE_CLOSED`. |
| `DELETE /v1/account/invites/{uuid}` | same | Revokes an open invite. |
| `GET /v1/invites/{token}` | public | Account name, email, role, status (`expired` is derived from `expires_at`), inviter, `user_exists`. |
| `POST /v1/invites/{token}/accept` | session | Joins with the invited role (an existing member keeps theirs). 403 `INVITE_EMAIL_MISMATCH`, 410 `INVITE_EXPIRED`, 409 `INVITE_CLOSED`, 403 for a suspended account. |

Invites live in `account_invites` (migration 000063), carry `account_id`, and are 404 from other
accounts. Only the SHA-256 of the 32-byte random token is stored. One open invite per address and
account. The email is the `accounts.invite` notification template (tr/en), addressed to the
invited address only; nothing else emails people, so existing and legacy-migrated users are never
mailed unless someone explicitly invites them. Code: `accounts/usecase/invites.go`, UI: the
"E-posta davetleri" card on `/account` and `features/accounts/components/invite-accept-page.tsx`.

**Not implemented:** a company panel shell (TEC-572), transferring cards or profiles between
accounts, and customer-side deletion of a company account.

## Legacy

The old `companies` / `company_members` tables and the `/v1/platform/companies/*` routes were
replaced by company accounts in migration 000038; company uuids were kept as account uuids.

## Company hierarchy (TEC-571)

Company accounts optionally select a bilingual sector from the platform-managed catalog
(`GET /v1/sectors`; 12 initial sectors). `GET/PUT /v1/company/sector` reads/replaces
`sector_uuid`; null clears it. Personal accounts cannot have company structure.

Offices belong to the active company and carry name, city, address, phone and sort order.
Groups are `department` or `team`, optionally attached to an office; a null office makes a
group company-wide. A member has at most one assignment (nullable office and group).
An office-bound group's office must match the assignment's office. Company-wide groups
may be assigned in any office. Assignments reference existing account members by user UUID.

| Routes | Behavior |
|---|---|
| `GET/POST /v1/company/offices`, `GET/PUT/DELETE /v1/company/offices/{uuid}` | Office list and CRUD |
| `GET/POST /v1/company/groups`, `GET/PUT/DELETE /v1/company/groups/{uuid}` | Group list and CRUD; the list takes `office_id=<office uuid>` (that office's groups) or `office_id=none` (company-wide groups) |
| `GET /v1/company/assignments` | Paginated assignments of the members visible to the caller (null office/group when unplaced); same read rules as the per-member read |
| `GET/PUT /v1/company/members/{id}/assignment` | Read/replace a member's assignment; `id` is the user UUID |
| `GET/PUT /v1/company/members/{id}/scopes` | Read/replace an office manager's `office_uuids` grants; non-admins read only their own |
| `GET/POST /v1/platform/sectors`, `GET/PUT/DELETE /v1/platform/sectors/{uuid}`, `GET /v1/platform/sectors/meta` | Platform catalog, `platform.sectors.read/write` |
| `GET /v1/platform/accounts/{uuid}/offices`, `.../groups` | Separate administrative reads, `platform.accounts.read` |

Company reads require `app.company_structure.read`. Writes require
`app.company_structure.write` plus owner/admin, with the following scoped exception:
`office_manager` (set through existing member role endpoints) may create/edit/delete groups
and change member assignments within granted offices. Scope grants themselves, offices and
sector selection remain owner/admin-only.
An office manager with no grants cannot write structure. Existing general account permissions,
billing and other product writes still require their original owner/admin roles.

**Visibility (TEC-948).** Reads follow the account role, loaded from the database per request:

| Role | Offices | Groups | Assignments (members) |
|---|---|---|---|
| owner/admin | all | all | all |
| office_manager | granted offices (+ own office) | groups of granted offices (+ own group) | members of granted offices, unassigned plain members (the pool) and themselves |
| member | own office | own group | only themselves |

Lists omit what the caller cannot see; single reads of a hidden office, group or member return
404. Manager grants (`/scopes`) are readable by owners/admins, and by each member for themselves.

Office managers must control both the source and destination office for a move. They may claim
an unassigned plain member (no office, role `member`) into a granted office. They cannot manage
company-wide groups, assign company-wide groups, claim unassigned owners/admins/managers, clear an
assignment's office or move a member out of scope. Every assignment change is written to the
activity log in the same transaction (`company.assignment.updated`, resource `company_member`,
resource UUID = the member's user UUID, payload with the account and from/to office and group). Removing/changing the office-manager role clears grants; later promotion needs
explicit new grants. Scopes are loaded from the database on each operation, not embedded in JWTs.

PUT uses replacement semantics. Null assignment references clear the assignment for owners/admins;
null group clears only the group while retaining the supplied office. Lists use `limit`/`offset`;
offices sort by sort order/name/id, groups by name/id and sectors by slug. Sector lists accept `q`.
The matching fixed `sort` (`sort_order,name`, `name`, or `slug`) is accepted; other sorts are rejected. Cross-account UUIDs return 404. Same-account foreign keys prevent
cross-company links even through direct SQL. Used offices/groups and selected sectors cannot be
deleted (409); clear/reassign references first. Scope grants alone do not block office deletion.
Structural operations serialize on the company row so scope revocation cannot race a write.
Rollback 000046 deletes hierarchy data and demotes office managers to members; PostgreSQL's
unused enum value is retained because individual enum labels cannot be removed safely.

## Bulk operations (TEC-573)

Company → "Toplu işlemler" (`/company/bulk`, `frontend/src/features/company-bulk`) changes many
employee profiles at once and imports employees. Code: `backend/internal/modules/companyprofiles`
(adapters, member-profile links), job routes in `modules/bulk` and `modules/imports`.
The sidebar item is shown only to owners/admins (nav `accountRoles`, TEC-948); the API rules below
are unchanged.

**Employee profile link.** `profiles.user_id` is the creator, so which profile is a member's card is
stored explicitly in `company_member_profiles` (one profile per member, one member per profile, same
account, enforced by FKs; migration 000050). The import creates links; owners/admins can also set or
clear one (`PUT /v1/company/members/{id}/profile`, `profile_uuid: null` unlinks, 409 if the profile is
linked elsewhere). `GET /v1/company/member-profiles` lists members with office, group and linked
profile (filters `q`, `office_uuid`, `group_uuid`, `has_profile`); `.../meta` returns the bulk actions.

**Bulk actions** (`POST /v1/company/profiles/bulk`, resource `app.company_profiles`, permission
`app.company_profiles.bulk`). `target.ids` are profile UUIDs; `target.query` (`office_uuid`,
`group_uuid`, `q`) selects the linked profiles of matching members. `params` per action:

| Action | Params | Effect |
|---|---|---|
| `set_contact` | `company_name`, `address` or `use_office_address`, `phone`, `email`, `website` | Omitted fields stay; `""` removes the work entry. Company name goes to every translation; the address replaces the `work` addresses and becomes primary; phone/email/website replace the `work` contact of that category. `use_office_address` copies the member's office address, city (and phone unless given); members without an office fail. |
| `set_logo` | `source` (`logo`/`cover`), `target_field` (`avatar`/`cover`) | Sets each target's avatar or cover to the company branding logo or cover (TEC-946). Nothing is copied: the profiles reference the library file, which lists them as its usages. Fails validation while the branding has no such file. |
| `set_theme` | `theme`, `layout_mode` | Allow-list of TEC-601 (`model.PublicThemes`, `PublicLayoutModes`); `""` clears, omitted keeps. |
| `add_links` | `links[]` (`platform` slug or `custom`, `label`, `url`), `replace_same_platform` | Links whose URL the profile already has are skipped; with `replace_same_platform` an existing link of the same platform gets the new URL. |

Writes go through the profiles service (validation, completion rate, vCard and route cache refresh).
Every item stores the previous values of exactly the parts it changed; rollback
(`POST /v1/company/bulk/{uuid}/rollback`, within `BULK_ROLLBACK_HOURS`) restores them (and removes the
per-profile copies that logo jobs made before TEC-946) and reports `rolled_back_partial` when an item could not be restored. Later edits of the same
fields are overwritten by a rollback. Unlike platform bulk, every app run is a persisted job, also
under `BULK_SYNC_MAX` (answered inline with 200 and the job); larger runs are queued (202). The job
summary lists the first 50 failed items with an error code (`not_allowed`, `not_found`, `no_office`,
or a validation message). `GET /v1/company/bulk[/{uuid}]` reads the account's jobs.

**Who.** Owners/admins act on every live profile of the company and see every job. Office managers
act only on profiles linked to members assigned to one of their granted offices (other items fail with
`not_allowed`), see and roll back only their own jobs. Role and grants are reloaded from the database
for every item, also during rollback, so a revoked grant takes effect at once. Plain members and
personal accounts cannot run bulk actions. Jobs carry `account_id`; another account's jobs are 404.

**Employee import** (resource `app.company_employees`, permission `app.company_profiles.import`,
owners/admins only). `POST /v1/company/employees/import` (multipart CSV/TSV/XLSX/JSON), sample at
`.../import/sample`, then the usual wizard on `/v1/company/imports/{uuid}` (mapping, preview,
confirm, rollback). Columns: `name`, `surname`, `email` (required), `title`, `office`, `group`,
`phone`. A row names a person by email. For a **registered** user:

- the user becomes a `member` if not yet one;
- `office`/`group` are matched by name (case-insensitive). A group bound to the row's office wins over
  a company-wide one; a group of another office or an ambiguous name is a row error. An empty office
  keeps the current one; a new office drops a current group that does not fit it;
- the linked profile is updated (name, surname, title on the primary translation, work email/phone),
  or a profile is created (company name = account name, locale `tr`) and linked. Plan quotas apply.

For an email **without a user** (TEC-945) the profile is created at once in the company account,
so the card works right away, and an invite (role `member`, source `import`) is sent that carries
the profile and the row's office/group. On acceptance the profile becomes the new member's linked
employee profile and the member is placed in that office/group (unless already linked or placed).
An open invite for the address is reused: its profile is updated, no second email goes out (an
expired one is renewed and resent). Once the person is registered, later imports treat the row as
a member row.

Preview is a real dry run: every row is checked against the account without writing, with row errors
as codes (`user_not_found`, `office_not_found`, `group_other_office`, ...) and `create`/`update` per
valid row. A row that fails part-way undoes its own steps. Rollback (24 h) deletes created profiles,
restores updated profiles and assignments, removes members the import added (if still plain
members) and deletes invites it created while still open (their links stop working).

## Branding and media library (TEC-946)

Module `backend/internal/modules/companybranding`, UI `/company/branding`
(`frontend/src/features/company-branding`). Company accounts only; reads need
`app.company_branding.read`, writes `app.company_branding.write` **and** an owner/admin role.

**Media library** (`company_media`): files the company owns, under
`accounts/{account_uuid}/media/{uuid}.{ext}` in object storage. `kind` is `logo`, `cover`,
`image` (JPEG/PNG/WebP, 5 MiB) or `document` (PDF, 10 MiB); the type is sniffed from the bytes,
never taken from the upload. At most 500 files per company.

- `GET /v1/company/media?kind=` lists files with their API-owned `url` and `usage`: the
  branding logo/cover and live profiles whose avatar or cover points at the file.
- `POST /v1/company/media` (multipart `file`, `kind`, `name`) uploads.
- `DELETE /v1/company/media/{uuid}` refuses a file that is still used (409, `error.details`
  lists the usages).
- Profiles of the company may use a library file as avatar or cover (the profiles service
  accepts `accounts/{active account}/media/` URLs); other accounts cannot.

**Branding** (`account_branding`, one row per company): `theme`, `layout_mode` (public page
allow-lists, `""` = unset), `primary_color` (`#rrggbb`), a logo and a cover picked from the
library (images only), and `locked`. `GET`/`PUT /v1/company/branding`; `PUT` replaces the whole
row. Employee public pages inherit it ([PUBLIC_PROFILES.md](./PUBLIC_PROFILES.md#themes-tec-601)):
unlocked it is their default, locked the company's set fields override the employee's theme,
layout and cover. Deleting a library file clears the branding pick (FK `ON DELETE SET NULL`),
but the API refuses that delete first.

## Cards & orders (TEC-572)

Company asset APIs live in `backend/internal/modules/companyassets`. They require an active
company account and the existing `app.cards.read/write` or `app.orders.read/write` permissions;
no new permission slugs or migration are needed (000052/000053 remain untouched).

| Route | Contract |
|---|---|
| `GET /v1/company/cards`, `GET /v1/company/cards/meta` | Card summary with lifecycle status, activity/lock flags, immutable route/path/status, linked profile, employee user UUID/name and office/group UUID/name. Activation codes and passcodes are excluded. |
| `GET /v1/company/orders`, `GET /v1/company/orders/meta` | Company-account order summaries: order number, status/payment/fulfillment, exact decimal total as a string, currency, placer UUID and creation time. No addresses, assets or recipient details. |
| `PUT /v1/company/cards/{uuid}/assignment` | `{ "member_uuid": "user UUID" }` assigns/reassigns to that member's live `company_member_profiles` link. Both card and card-route profile/target change atomically. |
| `PUT /v1/company/cards/{uuid}/active` | `{ "is_active": false/true }` reversibly disables/enables the card. Reactivation checks `cards.max`; an already active card is an idempotent no-op for quota purposes. |
| `POST /v1/company/orders/reorder` | `member_uuids` (1–100 distinct user UUIDs), `product_uuid`, optional `variant_uuid`, `shipping_address`, `billing_address`, `customer_note`. One current-price item per selected linked employee profile. |

Lists follow the `items/total/limit/offset` contract, default 20 and maximum 100. Filters are `q`,
`status`, `office_uuid`, `group_uuid`, plus `is_active` for cards. Sort accepts **one** of
`created_at`, `status`, `name` (cards) / `order_number` (orders), optionally prefixed with `-`;
default `-created_at`, stable ID tie breaker. Unknown/multiple sorts and forbidden query aliases
(`page`, `page_size`, `per_page`, `search`) return 400. Metadata advertises reads, search, filters
and sorting; it does not advertise bulk/export/import.

**Employee ownership and visibility.** A card belongs to the member whose linked employee profile
is its target, independent of `nfc_cards.user_id` (creator/activator). Owner/admin sees all cards
in the company, including unassigned cards. Office managers see only cards linked to employees
in their currently granted offices. Members see only cards linked to their own employee profile.
A card targeting an unlinked profile is unassigned for this panel.

Only `orders.account_id = active company` orders qualify: a member's personal-account orders
are never inferred to be company purchases. An order is associated with its placer (if still a
company member) and with members whose linked profiles occur in its items. Office/group filters
and member/office-manager visibility must match the **same** associated member. Owners/admins
also see orders without current member associations. This is summary visibility: a multi-office
order can appear in more than one office, with its whole-order total; the endpoint exposes no
other employees' item details. Search matches order number or placer name/email; card search
matches card name/code/serial or linked member name/email.

**Who can write.** Owner/admin may assign and toggle any eligible company card. An office manager
may do so only within granted offices, and reassignment checks **both** source and destination.
They cannot claim unassigned cards. Plain members cannot write. Only owner/admin may reorder;
office grants do not grant company purchasing privileges. Every operation reloads membership
and reuses TEC-571's database scope query and company/member locks. Source/destination profile
links and card/order/route rows are locked during changes. Foreign-account and out-of-scope
targets return 404; denied write roles or missing permissions return 403.

**Lifecycle.** Immutable route identity protects the printed path, route type and metadata; it
does not prohibit changing a target. Assignment preserves path, card UID, smart code and
`is_immutable`. Card actions reject `in_production` card/order/fulfillment states, cancelled or
refunded orders/payments and reserved or retired routes (409). Deactivation changes `is_active`,
which the resolver already honors, and preserves route status and lock/passcode behavior.
Reactivation cannot claim an unactivated card or reverse permanent retirement. Successful card
writes invalidate the resolver cache. An activated company card may retain an original purchase
order in another account: lifecycle checks reach that order only through the active company's
owned card, while the private purchase order remains excluded from company order lists.

**Reordering decision.** The existing domain has no separate order draft status. Reorder therefore
creates a normal **pending/unpaid** order through `orders.Service.CreateForAccount`; it does not
copy an old price, charge a payment or provision cards. It uses the selected current product and
its optional matching variant. All employees must still have live linked company profiles;
a missing/foreign member or link returns 404, duplicates/invalid selection return 400. Header
and all items share the company transaction, including rollback on an item failure. After commit,
the ordinary pending milestone notification and search indexing run via the existing order
service. The order then follows the normal commerce/payment/design/production lifecycle.

## Analytics (TEC-574)

Company → "Analitik" (`/company/analytics`) sums the existing daily analytics aggregates of the
company's profiles, cards and smart links (no new event type), with a daily series, a top-N
breakdown and a per-employee comparison (sortable, share of total, office/group filters). Requires
`app.company_analytics.read` (migration 000055). Owners/admins see the whole company, office
managers only employees of their granted offices, plain members get 403. Events are attributed to
the profile a card/route points at **now**, so a reassigned card takes its history with it.
Endpoints, metric definitions and scope rules: [ANALYTICS.md](./ANALYTICS.md#4-company-analytics-tec-574).
