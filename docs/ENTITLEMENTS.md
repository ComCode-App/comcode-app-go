# Entitlements (plan limits)

What an account's plan allows and where the API enforces it. Code: `backend/internal/platform/entitlement`
(catalog, service, guard), plan data in `plans` / `plan_limits` / `feature_definitions`, and the
guards in the modules that create capped resources. Who pays for a plan is in
[BILLING.md](./BILLING.md); the account scope is in [TENANCY_AND_ACCESS.md](./TENANCY_AND_ACCESS.md).

## How a plan is resolved

`entitlement.Service.ResolvePlan(accountID)` runs on every check (no caching):

1. The account's `active` or `trialing` subscription, or `past_due` subscription with a future `grace_until`.
2. Reject a subscription whose `ends_at` has passed, or whose scheduled cancellation boundary has passed.
3. Otherwise use the plan with slug `free`.

Both manual and payment subscriptions participate in entitlement resolution. Only payment
subscriptions participate in the billing worker. Paid features remain available through a trial,
the paid-through cancellation boundary, and the configured payment grace period. Once access
ends, existing resources remain; an account over its new limits cannot create more.
Platform super admins bypass entitlement checks, including history clamping.
See [BILLING.md](./BILLING.md#subscription-lifecycle-com-589) for retry, expiry and plan-change rules.

## Semantics of a limit

| Rule | Meaning |
|---|---|
| Every feature defined | Saving a plan requires a value for every feature (400 `required` on `limits.<key>` otherwise). A migration that adds a feature writes a value for every existing plan (`TestEveryPlanDefinesEveryFeature` guards it) |
| Fail-closed | Should a row still be missing, the feature is **not available**: a bool is false, a quota allows nothing, the analytics window is 0 days |
| `-1` | Unlimited (`entitlement.Unlimited`) |
| Quota check | Allowed while `used < limit`; the guard counts current usage itself |
| Type | `feature_definitions.value_type` is `int` or `bool`; a `plan_limits` row stores `value_int` or `value_bool` (check constraint: exactly one) |

## Feature catalog

Defined in code (`entitlement/catalog.go`, `Catalog()`) and in the DB (`feature_definitions`); the two must be kept in sync by migrations.

| Key | Type | Counts | Enforced in | Free / Pro (seed) |
|---|---|---|---|---|
| `profiles.max` | int quota | Non-deleted profiles of the account | `profiles` use case, create (`POST /v1/profiles`) | 1 / 10 |
| `smart_links.max` | int quota | `routes` with `route_type = 'custom_smart_link'`, not deleted | `routes` use case, create (`POST /v1/routes/smart-links`) | 3 / -1 |
| `cards.max` | int quota | `nfc_cards` with `is_active = true`, not deleted | `cards` use case: create (`POST /v1/cards`) and activation (`POST /v1/cards/activate`) | 1 / 10 |
| `analytics.history_days` | int | not a quota | `analytics/handler/access.go`: `from` of `/v1/analytics/*` is clamped to `today - days`; the window is returned in header `X-Analytics-History-Days`; `-1` = no clamp | 30 / 365 |
| `leads.monthly_max` | int, monthly | Lead form submissions this month | `leads` use case, `Submit`: over the quota the lead is stored **locked**; 0 = form off | 20 / 1000 |
| `exchange.monthly_max` | int, monthly | Contact exchanges this month | `leads` use case, `Exchange` / `ShareProfile`: same as leads | 20 / 1000 |
| `leads.export.monthly_max` | int, monthly | Lead exports this month | `exports` handler via `leads` `PrepareExport`: 403 past the quota | 2 / 100 |
| `design.premium` | bool | not a quota | Premium themes and animated backgrounds (TEC-947), see [below](#premium-design-tec-947) | false / true |

Migrations: 000022 (`cards.max`, plans), 000039 (the other three, and a value for every existing plan), 000057
(TEC-942: removes `nfc.enabled`, since NFC cards work on every plan, and backfills 0/false for any plan missing a feature),
000061 (TEC-941: the three monthly features; free gets the free value, every other plan the paid one),
000068 (TEC-947: `design.premium`, false for `free` and true for every other existing plan).
Pro values apply to `pro-monthly` and `pro-yearly`. Physical cards bought through an order are provisioned
without a quota check; the quota applies when the customer activates the card.

## How quotas are enforced

```go
// in a use case that creates a capped resource
if err := entitlement.Guard(ctx, s.quotas, account.ID, entitlement.FeatureProfilesMax); err != nil {
    return nil, err
}
```

- `s.quotas` is an `entitlement.Quotas` set with `SetQuotas(entitlementSvc)` in `internal/httpserver/server.go` (profiles, routes, cards). A service without a checker **denies** (fail-closed).
- `Guard` takes the principal's `IsSuperAdmin` from the context, then `Service.CheckQuota`, which counts usage by key (`Service.usage`) and compares it with the plan.
- An exceeded quota returns `*QuotaError` (`errors.Is(err, ErrPlanLimitExceeded)`). The handler calls `entitlement.WriteError(w, r, err)`, which answers **403 `PLAN_LIMIT_EXCEEDED`** with `error.details[0].field` = feature key and message `limit N, used M`.
- Count-then-insert is not serialised, so two concurrent creates can overshoot by one. Accepted for plan quotas; it is not a security boundary.
- Existing resources are never removed when a plan shrinks.

## Premium design (TEC-947)

`design.premium` (bool) unlocks premium visuals. Basic themes and static or color backgrounds stay free.

| Premium item | Defined in | Basic stand-in without the feature |
|---|---|---|
| Themes `bold_gradient`, `elegant_serif`, `glass` | `profiles/model.PremiumThemes` ↔ `premium` / `fallback` in `themes.json` | `classic_light`, `classic_light`, `dark_minimal` |
| Animated shaders (every shader except `backgrounds/model.StaticShaderTypes`) and videos | `backgrounds/model.IsPremium` ↔ `background-studio/lib/premium.ts`, `data/shader-tiers.json` | The theme's own backdrop |

- Saving checks it with `entitlement.RequireFeature` (super admins bypass; a nil checker denies): `POST`/`PUT /v1/backgrounds`
  for a premium design; `PUT /v1/profiles/{uuid}/public-page` and the company branding (`PUT` of `updateCompanyBranding`,
  TEC-946) for a premium theme or background that differs from the stored one; and the company bulk `set_theme`
  action (400 at job start). The answer is **403 `PLAN_LIMIT_EXCEEDED`**
  with `error.details[0].field = design.premium` (`*FeatureError`).
- A choice saved on a paid plan is never removed. After a downgrade the visitor API swaps in the basic stand-in
  (`premiumAllowed` in `profiles/usecase/public_page.go`), and the owner form may re-send the kept choice unchanged.
  An upgrade brings it back at once. An inherited company theme follows the same rule.
- Frontend: `useDesignPremium()`, `PremiumLockIcon`, `UpgradeLink` and `PremiumNotice` in `features/entitlements`.
  The theme picker, the company branding and bulk theme selects, the public page background select and the Background Studio (shader list, video kind, save
  button) show locks and the "Yükselt" link. Platform presets are not gated when created, only when picked.
- Sync tests: `themes.test.mjs` (premium themes and fallbacks) and `background-studio/lib/premium.test.mjs` (static shaders).

## Monthly quotas (TEC-941)

Catalog entries with `Monthly: true` count per account and calendar month in **Europe/Istanbul**
(`entitlement/monthly.go`, table `account_monthly_usage`, one row per account, feature and month). A new month
starts a new row, which is the reset; nothing has to run at midnight. `-1` is unlimited, `0` turns the feature
off, and a missing value is off (fail-closed). The admin plan form sets them like any int feature.

| Method | Used for | Behaviour |
|---|---|---|
| `RecordMonthly` | a lead / an exchange arrives | Counts it (`used`) and grants an unlocked slot while `unlocked < limit`; returns whether it was granted. An Off feature counts nothing |
| `ReserveMonthly` | an export is requested | `*QuotaError` when off or used up; otherwise counts it (an increment past the limit is undone, so concurrent requests cannot overshoot). Super admins bypass |
| `MonthlyUsage` / `AddMonthlyUnlocked` | unlocking after an upgrade | Allowance = `limit - unlocked` this month |

- `used` counts every event, locked leads included (the meter can read 25/20); `unlocked` counts the month's
  leads the owner may see, so deleting a visible lead does not unlock another.
- Locking, masking and unlocking of leads are described in [PUBLIC_PROFILES.md](./PUBLIC_PROFILES.md#locked-leads-tec-941).
- Alerts: when `used` reaches 80% of the limit (rounded up; skipped when that equals the limit) and when it reaches
  the limit, `SetUsageAlerter` (`notifications.QuotaAlerter`) notifies the account's owners and admins in the panel,
  realtime and by email (`entitlements.quota_warning` / `entitlements.quota_reached`). Counters move by one, so
  each threshold fires once a month.
- `GET /v1/account/entitlements` returns monthly features with `monthly: true`, `used` (this month) and `resets_at`.

## Reading limits

| Surface | Route | Shape |
|---|---|---|
| Active account overview | `GET /v1/account/entitlements` (any member) | Plan, and for every catalog feature: `included`, value, `unlimited`, `used` and `reached` (quota features) |
| Session snapshot | `GET /v1/auth/me` | `subscription` summary with the plan's limits (`Summary`) |
| Public plan list | `GET /v1/billing/plans` | Active paid plans with limits |

Frontend: `features/entitlements` (`PlanUsageCard` on `/account`, `QuotaNotice` and `useQuotaReached`
on the profiles, cards and smart-link list pages; the create button takes a disabled state from `useQuotaReached`;
`LeadsQuotaMeter` and the export button on `/leads` read the monthly quotas). Labels come from
locale keys `entitlements.features.<key>` in `locales/{en,tr}/entitlements.json`. A global toast handles any 403 `PLAN_LIMIT_EXCEEDED`.

## Managing plans

Platform panel `/platform/plans`, API `/v1/platform/plans` (`platform.plans.read` / `.write`):

- `GET /v1/platform/feature-definitions` lists the features a plan can set.
- `PATCH` with `limits` **replaces the whole limit set** (`replaceLimits` deletes then inserts); it is never merged. The set must hold every feature; omit `limits` to keep the current set. The admin form always sends the full set.
- Create and update write the plan row and its limits in one transaction: a failed limit write rolls the plan back too.
- Limits are validated on create and patch before anything is written (`plans/usecase/limits.go`, `ValidateLimits`, against `feature_definitions`):

  | Rule | Detail `code` |
  |---|---|
  | `feature_key` set and defined in `feature_definitions` | `required` / `unknown_feature` (field `limits[<i>].feature_key`) |
  | A key appears once | `duplicate` |
  | Exactly one value, matching `value_type`: `int_value` for `int`, `bool_value` for `bool` | `type_mismatch` / `required` |
  | `int_value` is `-1` (unlimited) or `0..2147483647` | `out_of_range` |

  Failures answer **400 `VALIDATION_ERROR`** with one `error.details[]` entry per problem, field `limits.<feature_key>`; the admin form shows them under the matching input. `plans/usecase/limits_integration_test.go` checks that every seeded plan passes these rules and that `feature_definitions` matches `Catalog()`.
- Pricing rules: `free` has price 0, other intervals need price > 0, currency `TRY`, `trial_days >= 0`.

## Adding a feature

1. Migration: insert into `feature_definitions` (`key`, `value_type`, label, description, `sort_order`) and insert a `plan_limits` value for **every existing plan** (follow 000039). Any plan without one silently loses the feature.
2. `entitlement/catalog.go`: add the `Feature…` constant and a `Catalog()` entry. Set `Quota: true` for a counted resource.
3. Quota only: add a `Count…Usage` query to `backend/internal/database/queries/entitlements.sql` (`sqlc generate`) and a case in `Service.usage`. Without it `CheckQuota` returns an error for the key.
4. Call `entitlement.Guard(ctx, quotas, accountID, key)` at the create site of the owning module (and `SetQuotas` in `server.go` if the module has none), and map the error with `entitlement.WriteError` in its handler.
   - Bool feature: call `Service.CheckBool` from the use case instead.
   - Non-counted int (like the analytics window): add a purpose-built method as `HistoryWindow` does.
5. Frontend: `entitlements.features.<key>` in both locales, and `QuotaNotice` / `useQuotaReached` on the list page.
6. Update the table above and the one in TENANCY_AND_ACCESS.md. If a route's behaviour changes, update `backend/docs/openapi.yaml`.
7. Test with `entitlement.checkIntLimit`-style unit tests (`service_test.go`) and the module's quota test (see `profiles/usecase/quotas_test.go`).
