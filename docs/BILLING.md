# Billing

How an account gets a paid plan: plans, subscriptions, the payment providers (iyzico, PayTR),
checkout and callbacks. Code lives in `backend/internal/modules/billing` (customer side),
`modules/plans`, `modules/subscriptions` (platform side), `modules/integrations/payment`
(provider settings) and `internal/platform/payment` (provider clients). What a plan allows is in
[ENTITLEMENTS.md](./ENTITLEMENTS.md). Billing belongs to **accounts**, not users
([TENANCY_AND_ACCESS.md](./TENANCY_AND_ACCESS.md)).

## Who can do what

| Surface | Routes | Who |
|---|---|---|
| Customer billing | `/v1/billing/*` | Any session for subscription/plan/checkout reads; writes and saved-card listing require `app.billing.write` plus the **owner/admin** role of the active account |
| Plans (platform) | `/v1/platform/plans/*`, `/v1/platform/feature-definitions` | `platform.plans.read` / `.write` |
| Subscriptions (platform) | `/v1/platform/subscriptions/*` | `platform.subscriptions.read` / `.write` |
| Provider settings | `/v1/platform/payment/settings`, `/providers/paytr`, `/providers/iyzico` | `platform.payment.read` / `.write` **and** super admin |
| Provider callbacks | `POST /v1/public/payment/paytr/callback`, `GET/POST /v1/public/payment/iyzico/callback` | public; authenticity is checked against the provider (below) |

Customer routes:

| Route | Required role | Purpose |
|---|:-:|---|
| `GET /v1/billing/plans` | member | Active, non-free plans with their limits |
| `GET /v1/billing/subscription` | member | The account's paid subscription (`active`, `trialing` or `past_due`, `source = payment`), or null |
| `DELETE /v1/billing/subscription` | owner/admin | Schedule cancellation at period end |
| `POST /v1/billing/subscription/resume` | owner/admin | Undo scheduled cancellation before period end |
| `PATCH /v1/billing/subscription/plan` `{plan_uuid}` | owner/admin | Prorated upgrade now / downgrade at period end |
| `POST /v1/billing/checkout` `{plan_uuid}` | owner/admin | Start a hosted checkout |
| `GET /v1/billing/checkout/{uuid}` | member | Read a checkout session of the active account |
| `POST /v1/billing/subscribe` `{plan_uuid, saved_card_uuid}` | owner/admin | Charge a saved card and subscribe in one request |
| `GET/POST /v1/billing/cards`, `GET /cards/add/{uuid}`, `DELETE /cards/{uuid}`, `PATCH /cards/{uuid}/default` | owner/admin (list, add, delete, default); session read member | Saved cards |

There is no step-up gate on billing routes.

## Data model

| Table | Notes |
|---|---|
| `plans` | `slug`, `billing_interval` (`free`, `month`, `year`, `lifetime`), `price_minor` (kurus: `99900` = 999.00 TRY), `currency`, `trial_days`, `is_active`. Check constraint: `free` means price 0, anything else means price > 0 |
| `plan_limits`, `feature_definitions` | Per-plan feature values; see ENTITLEMENTS.md |
| `subscriptions` | `account_id`, `plan_id`, `status` (`active`, `trialing`, `past_due`, `canceled`), `source` (`manual` or `payment`), `provider`, `provider_subscription_ref`, `checkout_session_id`, `saved_card_id`, `next_billing_at` (current period end), `period_started_at`, `starts_at`, `ends_at`, `canceled_at`, `cancel_at_period_end`, `pending_plan_id`, `grace_until`, `dunning_started_at`, `retry_at`, `renewal_attempts` |
| `subscription_payment_attempts` | Durable renewal/upgrade audit: account, subscription, period, attempt, plan, amount/currency, status, provider reference and safe failure code |
| `payment_settings` | Singleton row: `active_provider` (`none`, `paytr`, `iyzico`) and `test_mode` |
| `payment_provider_settings` | One row per provider: `enabled`, `merchant_id`, `base_url`, and encrypted key/salt/secret columns (`*_enc`) |
| `payment_checkout_sessions` | One per checkout attempt; unique `merchant_oid`; `status` = `pending`, `completed`, `failed`, `expired` |
| `payment_card_sessions` | Same, for adding a card without paying |
| `payment_saved_cards`, `payment_provider_customers` | Provider card/customer references, unique per account and provider |

Seeded plans (migration 000022/000023): `free`, `pro-monthly` (999.00 TRY), `pro-yearly` (9,990.00 TRY).
Only `TRY` is accepted when creating or editing a plan. The `free` plan cannot be deleted.

## Providers

`platform/payment.Registry` builds a `Provider` (init checkout, add card, charge saved card,
list/delete saved cards, verify callback) from the database:

- The active provider is `payment_settings.active_provider`. `none`, a disabled provider, or incomplete credentials fail with "not configured"; no checkout starts.
- PayTR needs `merchant_id`, merchant key and salt. iyzico needs API key and secret key (`base_url` defaults to the iyzico sandbox).
- Secrets are encrypted with `crypto.SecretBox` before they are stored. The settings API never returns them, only `*_configured` booleans.
- `test_mode` is passed to the provider on every call.
- Callbacks always use `ProviderBySlug`, so a callback for a provider that is no longer the active one still verifies.

Provider-specific verification:

| Provider | Callback transport | Verification | Response |
|---|---|---|---|
| PayTR | Form `POST` | HMAC-SHA256 of `merchant_oid + salt + status + total_amount` with the merchant key, compared with the posted `hash` | Plain text `OK` (200) or `FAIL` (400) |
| iyzico | Form `POST` or `GET` with a `token` | The token is mapped to a session by `provider_token`, then the result is fetched from iyzico's retrieve API (the payload alone is not trusted) and its signature checked with the secret key | `302` to `/billing/success`, `/billing` (card added) or `/billing?result=fail` |

## PayTR iFrame checkout and notifications (COM-591)

Contract references: [iFrame step 1](https://dev.paytr.com/en/iframe-api/iframe-api-1-adim)
and [Bildirim URL / step 2](https://dev.paytr.com/iframe-api/iframe-api-2-adim).
Token requests sign the exact posted values, in order:

```text
base64(HMAC-SHA256(merchant_key,
  merchant_id + user_ip + merchant_oid + email + payment_amount +
  user_basket + no_installment + max_installment + currency + test_mode + merchant_salt))
```

The basket is base64-encoded JSON. The token request amount is already in kuruş
(`12345` = 123.45 TRY); it must not be multiplied by 100 again. Basket prices use decimal
major units. A non-200 token response fails the pending checkout, even with a success-shaped body.

Notifications verify `base64(HMAC-SHA256(merchant_key,
merchant_oid + merchant_salt + status + total_amount))` with a constant-time comparison.
Only `success` and `failed` statuses and nonnegative integer minor-unit totals are accepted;
a successful total must be positive. Success must match the stored provider, original
`payment_amount` and currency (`TL` aliases `TRY`); `total_amount` cannot be less than the
order amount, but may be higher for installment fees. PayTR's hash does **not** bind the
posted currency or original amount, so these fields are consistency checks; the signed
`total_amount` enforces the minimum charge. Failed notifications may have a zero total and
omit currency/order amount. Their `failed_reason_code` and `failed_reason_msg` are saved
in checkout `metadata`.

PayTR checkout completion, subscription replacement and any saved-card persistence share
one database transaction. Row locks serialize duplicate notifications and different checkouts
for the same account. Already completed **or failed** checkouts return exactly plain-text
`OK` (200) without new writes, after verifying the hash. Invalid hashes or mismatches return
plain-text `FAIL` (400) without changing state. A database failure rolls back the checkout
and subscription together, allowing a later notification retry.

Configuration:

- Required environment: `APP_ENCRYPTION_KEY` (32 bytes, or base64 of 32 bytes) to encrypt
  database-stored credentials, the standard `DB_*` database settings, and
  `PUBLIC_FRONTEND_URL` for the externally reachable frontend origin.
- There are no `PAYTR_*` credential environment variables. In platform payment settings,
  enable PayTR, set merchant ID/key/salt, select PayTR as the active provider, and set
  `payment_settings.test_mode=true` for test payments. It drives posted `test_mode=1`
  and `debug_on=1`; false sends both as `0`. Use merchant credentials supplied by the owner.
- Production token URL defaults to `https://www.paytr.com/odeme/api/get-token`.
  The provider `base_url`, when explicitly set, overrides the origin and prefixes that
  path; leave it empty for PayTR. Tests point it only at a local `httptest` server.
- Register **`{PUBLIC_FRONTEND_URL}/api/v1/public/payment/paytr/callback`** in PayTR
  Mağaza Paneli → Destek & Kurulum → Ayarlar → Bildirim URL Ayarları (HTTPS for an HTTPS
  site). The BFF forwards to Go's `POST /v1/public/payment/paytr/callback`. Both paths
  are public; no login, browser session or CSRF token is required. Success/failure browser
  redirect URLs do not finalize payment.

Offline verification: `TEST_DATABASE_URL=postgres://… go test ./internal/modules/billing -v`
uses the real routes, authentication middleware, handler, usecase, encrypted DB settings,
Postgres transactions and a local fake PayTR token endpoint. It covers configured test/live
flags, signatures, pending creation, concurrent retries, success/failure, failure metadata,
tampering, amount/currency mismatches, installment fees, HTTP errors and rollback/retry.
It never contacts PayTR. Panel delivery, actual iframe/card entry and merchant activation
still require an owner-run PayTR test payment after deployment; no live transaction was run.

## Checkout flow

1. `POST /v1/billing/checkout`: owner/admin with `app.billing.write`. The plan must exist, be active, not `free`, and cost more than 0. The active provider is resolved.
2. A `payment_checkout_sessions` row is created: `pending`, 30 minute `expires_at`, `merchant_oid` = `chk…` (max 64 chars), amount and currency copied from the plan.
3. The provider is initialised with `CallbackURL = {API base}/v1/public/payment/{provider}/callback` and success/fail URLs on `/billing/checkout/{session}`. The response carries `iframe_token` (PayTR) or `checkout_url` (iyzico). If provider init fails, the session is marked `failed`.
4. The frontend page `/billing/checkout/[sessionUuid]` redirects the browser to `checkout_url` (iyzico) or embeds the PayTR iframe, and keeps refetching `GET /v1/billing/checkout/{uuid}` while the session is pending.
   PayTR returns to the success/fail URL **inside its iFrame** (app > www.paytr.com > app). The result pages `/billing/checkout/{uuid}`, `/billing/cards/add/{uuid}` and `/orders/checkout/{uuid}` therefore send `frame-ancestors 'self' https://www.paytr.com` and no `X-Frame-Options` (TEC-1125, `frontend/src/config/security-headers.ts`); every other page stays `DENY`. `PaymentFrameExit` in the app layout moves the result page to the top window before the route guard runs (the session cookie is not sent inside the cross-site frame). iyzico needs neither: it uses a top-level redirect and a form POST to the API callback.
5. The provider calls the callback. `finalizeCheckout` (idempotent: a `completed` session is a no-op):
   - failure: session `failed`;
   - success: session `completed`, the account's `active` subscriptions are set to `canceled`, and a new `source = payment` subscription is created: `trialing` if the plan has `trial_days`, else `active`, with `next_billing_at` set (trial end, or +1 month / +1 year; none for `lifetime`).
6. If the provider returned card tokens, the card is saved as the account's default.

`GET /v1/billing/checkout/{uuid}` reports `expired` for a pending session past its `expires_at`, but the stored status is not updated (no cleanup job).

Adding a card (`POST /v1/billing/cards`) is the same shape with `payment_card_sessions` (`card…` merchant oid) and no subscription.

## Subscribe with a saved card

New checkout/subscribe requests reject accounts with an existing payment subscription; use the plan-change endpoint instead.

`POST /v1/billing/subscribe` charges the saved card synchronously through the card's own provider (`ChargeSavedCard`). A declined charge returns 400 `payment declined`; on success the account's active subscriptions are canceled and a new payment subscription is created with the card attached.

## Subscription lifecycle (COM-589)

The Asynq scheduler queues `app:billing:renewal_sweep` every five minutes on the maintenance
queue. It discovers up to 100 due subscriptions per sweep, exclusively `source = payment`
and `active` / `trialing` / `past_due`, with `next_billing_at <= now`. Manual grants and
migrated legacy users without payment subscriptions are never selected or notified.
Lifetime subscriptions have no renewal boundary and cannot schedule period-end cancellation.

Provider support:

| Provider | Automatic renewal / prorated upgrade |
|---|---|
| iyzico | Supported through the existing synchronous stored-card NON3D payment API, using the subscription's provider/card (or that provider's default card when no card is attached). Merchant stored-card permissions are required. |
| PayTR | Unsupported **in this integration**. Its recurring API can return `wait_callback`, which the existing saved-card abstraction does not preserve. No automatic request is sent; a failed attempt with `recurring_unsupported` enters dunning. A verified asynchronous settlement adapter is required before enabling it. |

Contract references: [iyzico stored-card payments](https://docs.iyzico.com/en/getting-started/preliminaries/api-reference-beta/card-storage),
[PayTR recurring outcomes](https://dev.paytr.com/en/direkt-api/kart-saklama-api/kayitli-kart-tekrarlayan-odeme).
An unsupported provider or missing saved card never counts as payment success.

The worker sweep runs every 5 minutes but does nothing unless `BILLING_RENEWAL_ENABLED=true`
(default `false`), so deploying this code never starts charging saved cards on its own; an
operator turns it on after verifying the provider's recurring-charge setup.

Successful renewal records a captured payment in `subscription_payment_attempts`, sets `active`,
clears dunning, starts a new period at the processing time and sets `next_billing_at` one calendar
month/year later (clamped to the last day of a shorter target month). Delayed processing does not charge for missed historical periods.
It emits `payment.succeeded` and `subscription.renewed` through the transactional outbox.
`next_billing_at` is the current period end; `ends_at` records the actual termination boundary.

A definitive decline sets `past_due` and emits `payment.failed`; the first decline also emits
`subscription.past_due`. Defaults: `BILLING_RETRY_DAYS=1,3,7` (offsets from the first failure,
three retries after the initial attempt) and `BILLING_GRACE_DAYS=14`. Retry offsets must be
positive, strictly increasing and less than the grace length (1–365 days). Environment values
belong in the root `.env`; production compose forwards them to API/worker. The grace deadline
is persisted; later policy changes do not extend an existing deadline. Trials and overdue
subscriptions retain paid entitlements during grace. At the deadline, entitlements resolve to
free even before the next worker sweep. The worker stores `status=canceled`, `ends_at=grace_until`
and emits `subscription.expired`. No account resources are deleted.

### Cancellation and changes

`DELETE /v1/billing/subscription` schedules cancellation; repeated calls are idempotent.
The subscription retains its status and paid access until `next_billing_at`. The worker ends
it at that boundary without charging and emits `subscription.canceled`. `POST .../resume`
removes the flag before the boundary; it cannot restart an ended subscription.
Scheduling cancellation clears a pending plan change. An unresolved payment must be reconciled
before changing cancellation or plan settings.

`PATCH /v1/billing/subscription/plan` accepts `plan_uuid`. Within the same billing interval
and currency, a higher-priced plan charges **ceil((new price − old price) × remaining period
seconds / total period seconds)** in minor units, calculated with exact integer arithmetic.
The plan changes immediately only after confirmed capture; the billing date stays unchanged.
No second charge occurs when retrying the same already-applied upgrade. A declined upgrade
leaves the current plan unchanged. Lower/equal-priced changes apply on the next renewal at
the new price, without refunds. Selecting the current plan clears a pending change; selecting
free schedules the end of the payment subscription. An accepted paid plan change clears a
previous cancellation. Cross-interval changes are rejected: end the subscription first and
start a new one. Overdue accounts must settle renewal before changing plans.
All billing writes require `app.billing.write` plus the active account's owner/admin role;
there is still no billing step-up gate.

### Charge safety and reconciliation

Row locks serialize claims and user actions. A committed attempt has a unique
`(subscription_id, period_at, kind, attempt)` key before any network request, and the worker
will not charge while an attempt is `processing` or `unknown`. After a network error or crash,
**do not automatically retry**: the bank may already have captured the payment. Such attempts
require operator reconciliation using the attempt UUID (`ren<uuid>` / `upg<uuid>` merchant
reference), provider panel and audit history. No reconciliation HTTP endpoint is provided.
An operator must verify capture/decline before making any corrective database change or
allowing another charge; never delete a pending claim merely to unblock the worker.
A crash after capture but before settlement deliberately favors preventing double charges over
automatic recovery. Grace still bounds access; unresolved upgrades enter dunning at renewal.

Lifecycle writes and outbox events commit together. Notifications use existing tr/en templates,
preferences and notification delivery exclusively, targeted to the account's owners/admins.
Event/user/channel deduplication protects outbox redelivery. SMTP configuration is unchanged;
no alternate mail sender or migration-wide mailing is introduced.

## Automated vs manual

| Step | Status |
|---|---|
| Hosted checkout, callback verification, subscription creation | Automated |
| Saved cards, immediate charge on subscribe | Automated |
| Renewal, failed-payment retries, grace expiry | Automated (provider support above) |
| Period-end cancellation/undo, prorated upgrade, scheduled downgrade | Implemented |
| Plan limits including trials and grace | Resolved per request, no cache |
| Card order checkout, callback, paid → provisioning | Automated (TEC-943) |
| Card order refunds through iyzico/PayTR | Automated, platform action (TEC-935) |
| Payment history and PDF receipts | Implemented (TEC-590, below) |
| Subscription refunds, invoices (e-arşiv) | Not implemented; subscription refunds remain in the provider panel, the e-arşiv integrator is an open decision |
| Manual subscriptions | Platform-admin managed; excluded from automatic renewal/dunning |

## Card order payments (TEC-943)

Card orders ([COMMERCE_AND_ORDERS.md](./COMMERCE_AND_ORDERS.md)) are paid online through the same
registry, active provider and callback routes as subscriptions.

| Route | Who | Purpose |
|---|---|---|
| `POST /v1/orders/{uuid}/checkout` | `app.orders.write` + owner/admin of the order's account | Start a checkout for a payable order |
| `GET /v1/orders/checkout/{uuid}` | `app.orders.read`, same account | Read the session (result screen) |

- An order is payable while `status = pending` and `payment_status` is `unpaid` or `failed`
  (409 otherwise; 503 when no provider is configured). Only `TRY` orders can be paid.
- Each start creates an `order_payment_sessions` row (migration `000062`): `pending`, 30 minute
  `expires_at`, `merchant_oid` = `ord…` (the prefix routes the callback), amount = the order's
  `grand_total` in kuruş, provider = the active one. The card is not stored. iyzico gets a
  `PRODUCT` payment group with a physical basket item.
- Success/fail URLs and the iyzico redirect go to `/orders/checkout/{session}?result=…`. That page
  embeds the PayTR iFrame or redirects to iyzico, polls the session and shows the result; it never
  decides the outcome. A failed or expired attempt offers "pay again", which starts a new session.
- Callbacks are verified exactly like subscription checkouts (PayTR hash plus amount/currency
  consistency checks, iyzico retrieve API + signature). The session row is locked; a session that
  already left `pending` returns `OK` without writes, so duplicate notifications are no-ops.
- **Success**: in one transaction the session becomes `completed` and the orders module
  (`ApplyProviderPayment`) locks the order, sets `payment_status = paid` with the provider and
  reference, and moves a `pending` order to `paid` with a system `order_status_history` row. After
  commit the paid milestone notification is sent and cards are provisioned in the background.
- **Failure**: only the session becomes `failed` (PayTR reason saved in `metadata`); the order stays
  `pending`/`unpaid` and can be paid again.
- If the money arrives for an order that was cancelled/refunded or already paid (two sessions paid,
  or an admin marked it by hand), the session is completed with `metadata.order_result`
  (`order_closed` / `already_paid`), the order is not changed and `order_payment_needs_refund` is
  logged. The platform order screen lists such a payment as an extra payment to refund (below).
- A pending session past `expires_at` reads as `expired`, but a late successful callback still pays
  the order.

The platform admin can still record payment by hand with `PUT /v1/platform/orders/{uuid}/payment`
(`payment_status`, `payment_provider`, `payment_reference` as free text) for bank transfers and
other exceptions. Marking it `paid` also moves a pending order to `paid` (history row with the admin
as actor). Manual payment changes and manual `status = paid` are written to the activity log
(`order.payment_status_changed`, `order.status_marked_paid`).

### Card order refunds (TEC-935)

Online order payments are refunded through the provider's refund API. Orders paid by hand (bank
transfer) keep the manual `payment_status = refunded`, which refunds nothing.

| Route | Who | Purpose |
|---|---|---|
| `GET /v1/platform/orders/{uuid}/refunds` | `platform.orders.read` | Completed online payments with paid/refunded/pending/refundable amounts, and the refunds |
| `POST /v1/platform/orders/{uuid}/refunds` `{idempotency_key, amount_minor?, payment_uuid?, reason?}` | `platform.orders.write` + step-up | Full (no amount) or partial refund |
| `POST /v1/platform/orders/{uuid}/refunds/{refundUuid}/retry` | `platform.orders.write` + step-up | Send a failed refund again |

- Each refund is an `order_payment_refunds` row (migration `000070`): `pending`, `succeeded` or
  `failed`, amount in kuruş, a unique `reference_no` (`rfd…`) sent to the provider, the caller's
  unique `idempotency_key`, the provider reference, the error code/message and the attempt count.
- **Order of steps.** The row is committed as `pending` under a lock on the payment, then the
  payment's own provider is called (`ProviderBySlug`, so it works after switching the active
  provider), then the result is stored. Only after the provider confirmed does the orders module
  (`ApplyProviderRefund`) change the order, in the same transaction as the refund row:
  `payment_status = partially_refunded`, or `refunded` once the whole payment is returned, and a
  fully refunded order that was not cancelled moves to `refunded` with an `order_status_history`
  row (the admin as actor). After the commit its card routes are retired (TEC-585) and the refunded
  milestone is sent. Refunding an extra payment (`already_paid` / `order_closed`, above) never
  changes the order.
- **Provider failure.** The order is untouched; the refund becomes `failed` with
  `provider_rejected` (the provider answered, nothing was refunded, retry is safe),
  `provider_error` (no answer: check the provider panel before retrying) or
  `provider_not_configured`. The route answers `502 REFUND_FAILED`; the platform order screen
  (Customer & finance tab) shows the error and a retry button. Failed refunds do not count against
  the refundable amount; a retry re-checks it and reuses the same `reference_no`.
- **Idempotency and concurrency.** The same `idempotency_key` returns the first refund (`200`)
  without calling the provider, even for concurrent requests. At most one refund per payment can be
  pending (`409 REFUND_IN_PROGRESS`); nothing left is `409 NOTHING_TO_REFUND`; an amount above the
  refundable one is `400`. A refund that stays `pending` means its result could not be stored
  (`order_refund_finalize_failed` is logged): check the provider panel.
- **Status change to refunded.** `PUT /v1/platform/orders/{uuid}/status` with `refunded` or
  `PUT …/payment` with `refunded` on an order paid online first refunds what is left of the paying
  payment (same flow, a generated key) and needs a step-up grant; a failed refund returns `502` and
  the status does not change.
- **Providers.** PayTR: `POST https://www.paytr.com/odeme/iade` with `merchant_id`, `merchant_oid`,
  `return_amount` (decimal TRY), `reference_no` and `paytr_token` =
  `base64(HMAC-SHA256(merchant_key, merchant_id + merchant_oid + return_amount + merchant_salt))`
  (a configured `base_url` replaces the origin, for tests). iyzico: the payment
  (`provider_ref` = paymentId) is retrieved and its single basket transaction refunded
  (`/payment/refund`, `conversationId` = `reference_no`).
- Activity log: `order.refund_succeeded` / `order.refund_failed` with amount, provider, payment,
  attempt and error.

Offline verification: `TEST_DATABASE_URL=… go test ./internal/modules/billing -run 'TestOrderRefund|TestOrderStatusRefunded'`
(fake PayTR refund endpoint; full, partial, concurrent idempotent requests, declined and unanswered
calls with retry, status change to refunded, step-up, card retirement) and
`go test ./internal/platform/payment -run Refund` (PayTR signature, iyzico retrieve + refund
against fake servers). No live refund was run: an owner-run test refund in the PayTR/iyzico
test mode is still needed after deployment.

Offline verification: `TEST_DATABASE_URL=… go test ./internal/modules/billing -run TestOrderCheckout`
(fake PayTR token endpoint; covers signatures, duplicates, paid → provisioning, failure/retry,
closed and double-paid orders, account scoping).

## Payment history and receipts (TEC-590)

| Route | Who | Purpose |
|---|---|---|
| `GET /v1/billing/payments` | any member of the active account | Every successful charge, newest first (`limit`/`offset`) |
| `GET /v1/billing/payments/{uuid}/receipt?locale=tr\|en` | same | Letterheaded PDF receipt of one payment |

Nothing new is stored: the history is a union over the tables that already record payments
(`internal/database/queries/billing_payments.sql`):

| `kind` | Source |
|---|---|
| `subscription_checkout` | The completed `payment_checkout_sessions` row a subscription started from |
| `subscription_renewal`, `subscription_upgrade` | `subscription_payment_attempts` with `status = succeeded` |
| `order` | Completed `order_payment_sessions` whose `metadata.order_result` is `applied` (money that arrived for a closed or already paid order is a refund case, not a receipt) |
| `order_manual` | Orders with `paid_at` set and no online payment: bank transfers recorded by the platform (`PUT /v1/platform/orders/{uuid}/payment`); the provider column is the free text the admin typed |

Each payment has a stable `receipt_number` (`RCP-` + the first 12 hex digits of its uuid). The
receipt (`ioengine.EncodeReceiptPDF`) carries the platform letterhead (`/platform/settings`
letterhead, logo included), the payment date, the account name, a description (plan and interval or
order number), the payment method and provider reference, the amount, and a note that it is a
receipt, not an invoice. Refunded orders keep their receipt; the refund itself is a platform record
(TEC-935). The app shows the list as "Ödeme geçmişi" on `/billing` with a download button per row.

Not implemented: invoices. Issuing e-fatura / e-arşiv documents needs an integrator decision
(the receipt note tells the customer the invoice is issued separately).

## Logging

Billing and provider code logs structured events (`checkout_*`, `paytr_callback_*`,
`iyzico_callback_*`, `finalize_checkout_*`, `order_checkout_*`, `order_payment_*`, `order_refund_*`,
`paytr_refund_*`, `iyzico_refund_*`). Payloads go through `SafePayload` / `MaskSecret`
(`platform/payment/log.go`), which hides hashes, tokens and secrets.

## Extending

- New provider: implement `payment.Provider`, add it to `Registry.providerFromConfig`, the `payment_provider` / `active_payment_provider` enums (migration), the settings usecase, and a public callback route (plus an entry in `backend/docs/openapi.yaml`).
- New plan: create it in the platform panel (`/platform/plans`) or `POST /v1/platform/plans`; give it a value for every feature (ENTITLEMENTS.md).
