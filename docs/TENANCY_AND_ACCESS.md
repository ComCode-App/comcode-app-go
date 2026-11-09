# Tenancy & access (account model)

Status: **implemented** (2026-10-11, TEC-611). Supersedes the boilerplate rule "no tenants / workspaces".

ComCode resources belong to **accounts**, not to users. Every user has a personal account; companies are accounts too. A user can belong to many accounts and switches the active one in the UI.

## Why

The previous model had user-owned rows plus a `companies` table that was never wired: handlers passed `company_id = nil`, entitlement picked an arbitrary membership, and ownership checks were spread across SQL, use cases and handlers with `owner = nil` meaning "no filter". That fail-open pattern produced a routes IDOR (TEC-610). A company could not own cards or profiles, so an employee leaving took the company's cards with them.

## Model

```
users ──< account_members >── accounts ──< profiles
                 role                   ──< nfc_cards
       owner | admin | member           ──< routes
                                        ──< bank_accounts
                                        ──< orders
                                        ──< subscriptions (one active)
                                        ──< payment_checkout_sessions / saved cards
```

| Table | Notes |
|-------|-------|
| `accounts` | `type` = `personal` \| `company`, `status` = `active` \| `suspended`, unique `slug`. Personal accounts carry `personal_user_id` (unique) |
| `account_members` | `(account_id, user_id)` unique, `role` = `owner` \| `admin` \| `member` |
| `refresh_tokens.active_account_id` | Active account for that session; drives the `acc` access-token claim |

Invariants:

- A DB trigger on `users` insert creates the personal account and its `owner` membership. Every user-creation path (register, OAuth, platform create, import, CLI, seed) gets one without code changes.
- A personal account has exactly one member, its user, as `owner`. Members cannot be added to it.
- A company account always has at least one `owner`. The last owner cannot be removed or demoted.
- Owned resources carry `account_id` (`NOT NULL`, except `routes`: system-reserved and unclaimed legacy routes have no owner). Remaining `user_id` columns mean the **acting member**, never ownership: the profile/card/bank-account creator, the member who placed an order (`orders.user_id`), activated a card (`nfc_cards.user_id`) or started a checkout. Subscriptions have no user or company column; their subject is the account.
- The legacy `companies` / `company_members` tables are gone (migration 000038); company accounts replaced them and kept their uuids.

## Authorization: two layers

1. **Global RBAC** (unchanged): roles → permissions. `platform.*` gates the admin panel; `app.*` gates product features for the subscriber role.
2. **Account role** on the active account:

| Capability | owner | admin | member |
|-----------|:-----:|:-----:|:------:|
| Read account resources | ✓ | ✓ | ✓ |
| Create / edit / delete resources | ✓ | ✓ | — |
| Manage members (non-owner) | ✓ | ✓ | — |
| Manage owners, billing, delete account | ✓ | — | — |

An app write needs **both**: the `app.<resource>.write` permission and an account role ≥ `admin`. On a personal account the user is `owner`. Company structure additionally supports `office_manager` with database-backed office scopes; this grants no general account write or billing privileges. See [COMPANIES.md](./COMPANIES.md#company-hierarchy-tec-571).

Platform routes (`/v1/platform/*`) are not account-scoped; they use dedicated `*Admin` repository methods and `platform.*` permissions.

## Scope is fail-closed

- `authctx.Principal.Account` holds the resolved active account (`ID`, `UUID`, `Type`, `Role`). Every authenticated request has one.
- App use cases and repositories take a non-optional `accountID int64`. There is no "nil means everything" parameter. A missing scope is a programming error, not an admin bypass.
- Rows outside the active account return `404`, never `403` (no existence oracle).

## Session & tokens

- Access JWT claims: `sub`, `roles`, `is_super_admin`, `exp`, optional `imp`, `sid`, and **`acc`** (active account UUID).
- On every request `LoadPrincipal`:
  - reloads roles and super-admin status from the DB (a revoked role stops working immediately, not after the token TTL),
  - rejects tokens whose `sid` refresh session is revoked or expired,
  - verifies membership in `acc`. If the membership is gone it falls back to the personal account instead of failing the request.
- `POST /v1/auth/accounts/switch {account_uuid}` checks membership, stores `active_account_id` on the current refresh session and returns a fresh token pair. The BFF treats it like refresh (tokens go into HttpOnly cookies).
- Refresh keeps the session's active account. Impersonation starts on the target user's personal account.

## API surface

| Method | Path | Notes |
|--------|------|-------|
| GET | `/v1/auth/accounts` | Memberships of the current user (with role, type) |
| POST | `/v1/auth/accounts/switch` | Change active account, returns tokens |
| GET | `/v1/account` | Active account detail |
| PATCH | `/v1/account` | Rename (owner/admin, company accounts) |
| GET | `/v1/account/members` | Members of the active account |
| POST | `/v1/account/members` | Add a user with a verified email (owner/admin); other addresses get an email invite (202) |
| PATCH | `/v1/account/members/{user_uuid}` | Change role |
| DELETE | `/v1/account/members/{user_uuid}` | Remove member / leave |
| — | `/v1/platform/accounts/*` | Platform CRUD for company accounts + members; replaces `/v1/platform/companies/*` |

`/v1/auth/me` returns `account` (active) and `accounts[]`.

## Entitlement

- The plan is resolved for the **active account**: its active or `trialing` subscription (a trial gets the trial plan's limits), otherwise `free`.
- Every plan defines every feature (enforced on save). Limits are fail-closed: a feature key missing from the plan means "not available". `-1` means unlimited. Platform super admins bypass plan limits.
- Usage counts are per account. Monthly quotas (TEC-941) count per account and calendar month in Europe/Istanbul (`account_monthly_usage`).

| Feature | Type | Enforced where |
|---------|------|----------------|
| `profiles.max` | int quota | `POST /v1/profiles` |
| `smart_links.max` | int quota | `POST /v1/routes/smart-links` |
| `cards.max` | int quota (active cards) | `POST /v1/cards`, `POST /v1/cards/activate` |
| `analytics.history_days` | int | `/v1/analytics/{summary,timeseries,breakdown}`: `from` is clamped; `X-Analytics-History-Days` returns the window |
| `leads.monthly_max` | int, monthly | `POST /v1/public/leads`: over the quota the lead is stored locked (masked); 0 turns the form off |
| `exchange.monthly_max` | int, monthly | `POST /v1/public/exchanges`, `POST /v1/exchanges`: same as leads |
| `leads.export.monthly_max` | int, monthly | `POST /v1/leads/export`: 403 once the month's quota is used up |
| `design.premium` | bool | Saving or picking a premium theme / animated background: 403; public pages render the basic ones without it (TEC-947) |

- Modules take `entitlement.Quotas` (`SetQuotas`) and call `entitlement.Guard(ctx, quotas, accountID, key)` before a capped create. The guard counts usage itself; a service without a checker denies (fail-closed).
- Quota errors are `403 PLAN_LIMIT_EXCEEDED`, with `error.details[0].field` naming the feature. The frontend shows a global "upgrade" toast for them.
- `GET /v1/account/entitlements` returns the plan, every catalog feature, its value and quota usage. The app shows it on `/account` (`PlanUsageCard`) and as `QuotaNotice` on list pages.
- Count-then-insert is not serialised: two concurrent creates can overshoot a quota by one. That is accepted for plan quotas.
- New feature: migration (`feature_definitions` + a value for **every** existing plan), `entitlement/catalog.go`, the guard at the create site, locale `entitlements.features.<key>`.

Analytics reads are account-scoped: anyone without `platform.analytics.read` may only query a `route`, `card` or `profile` of the active account (404 otherwise). Company analytics (`/v1/company/analytics/*`) sums those subjects for the active company only, scoped by account role ([ANALYTICS.md](./ANALYTICS.md#4-company-analytics-tec-574)).

## Card activation

Activation moves a provisioned card into the **active account**. The activation endpoint is rate-limited per user and IP, because activation codes are bearer secrets.

## Out of scope (follow-ups)

- Invitations by email for users who are not registered yet
- Company panel shell: TEC-572
- Ownership transfer between accounts (moving cards or profiles)
