# Commerce and orders

How a physical card goes from an order to a working `/c/{code}` link. Code lives in
`backend/internal/modules/orders` (orders, items, provisioning, manifest), `modules/cards`
(activation, card edits) and `modules/routes` (the public resolver). Tenancy rules are in
[TENANCY_AND_ACCESS.md](./TENANCY_AND_ACCESS.md), path rules in
[URL_ROUTING_AND_SMART_LINKS_ARCHITECTURE.md](./URL_ROUTING_AND_SMART_LINKS_ARCHITECTURE.md).

## Who can do what

| Surface | Routes | Permission |
|---|---|---|
| Customer (app) | `GET/POST /v1/orders`, `/v1/orders/bulk` | `app.orders.*` plus an owner/admin account role for writes; always the active account |
| Customer payment | `POST /v1/orders/{uuid}/checkout`, `GET /v1/orders/checkout/{uuid}` | same as above; see [BILLING.md](./BILLING.md#card-order-payments-tec-943) |
| Platform panel | `/v1/platform/orders/*` | `platform.orders.read` / `.write`; unscoped |
| Design asset upload | `GET /v1/orders/assets/{token}`, `POST /v1/orders/assets/upload` | public, by one-time token |

## Order lifecycle

Three independent status fields on `orders`:

- `status`: `pending → paid → design_pending → design_approved → in_production → ready_to_ship → shipped → delivered`, or `cancelled`.
- `payment_status`: `unpaid`, `paid`, `partially_refunded`, `refunded`, `failed`
  (`partially_refunded` is only set by provider refunds, see [BILLING.md](./BILLING.md)).
- `fulfillment_status`: per order and per item.

Side effects in `orders/usecase/service.go`:

| Change | Effect |
|---|---|
| `status` or `payment_status` → `paid` | Cards are provisioned in the background (`ProvisionOrder`); the customer is notified once for the paid milestone. A `payment_status` → `paid` change also moves a `pending` order to `paid`. |
| Every `status` change | Customer email + in-app notification (see Notifications). |
| `status` → `cancelled` (or `refunded`), `payment_status` → `refunded` | **Card routes are retired** (see below); customer notification. The platform UI asks for confirmation first. |

## Payment (TEC-943)

The customer pays a `pending` order online from `/orders` ("Pay now") through the payment module
(iyzico/PayTR); the verified callback marks it paid and starts provisioning. A failed payment leaves
it pending and payable. The platform admin's manual payment update stays for exceptions and is
recorded in the activity log. Details: [BILLING.md](./BILLING.md#card-order-payments-tec-943).

## Status history (TEC-944)

Every `status` transition writes one `order_status_history` row (`order_id`, `account_id`,
`from_status`, `to_status`, `changed_by_user_id`, `changed_at`) in the same transaction as the
change: the repository locks the order row (`SELECT … FOR UPDATE`), updates it and records the
transition only when the status actually changed. Creation writes the first row
(`from_status` NULL → `pending`, by the placer). This covers direct status updates, shipping
marked as shipped, design requests, asset approval and payment (provider callback: system actor;
manual payment update: the admin). `changed_by_user_id` is the signed-in user
from the request context, NULL for system transitions. `payment_status` and `fulfillment_status`
changes are not recorded. Migration `000058` backfilled one row per existing order for its current
status only (`from_status` NULL, `changed_at` from the matching milestone timestamp or
`updated_at`); earlier transitions of those orders are unknown. The platform conversion funnel
reads this table ([ANALYTICS.md §3](./ANALYTICS.md#3-platform-overview-kpis)).

## Notifications (TEC-584)

Migration `000047` seeds Turkish and English email/in-app templates for `pending`, `paid`,
`design_pending`, `design_approved`, `in_production`, `ready_to_ship`, `shipped`, `delivered`,
`cancelled` and `refunded`. Creation sends the initial `pending` milestone. There is no enforced
transition graph: the existing platform status update behavior is preserved. Design requests,
asset approval and shipping marked as shipped use the same notification path as direct status
updates. Payment `paid` and `refunded` also notify, sharing the corresponding main-status milestone.
Fulfillment-only bookkeeping and tracking edits without a status change do not announce a milestone.

The recipient is the recorded order placer if still a member of the order's account; otherwise
it is that account's first owner (membership order). This prevents platform-created orders from
notifying an unrelated platform operator. Each recipient's `users.locale` selects `tr` or `en`,
with Turkish fallback; accounts currently have no locale column. Notification preferences apply.
Variables include order number, localized status, customer name, carrier/tracking number/tracking
URL when present, and an absolute link to the existing `/orders` app page. That page has no order
detail deep link. No manual/B2B payment link is generated.

An order trigger records an account-scoped milestone snapshot in `order_notification_events`
inside the status transaction. Rolled-back transitions leave no event. After the repository write
commits, the use case reads committed events and calls `notifications.Service.Enqueue` for email
and in-app. Enqueue failures never fail the transition; they log `order_notification_enqueue_failed`
with `request_id`, account id and order UUID. Failed/partial events remain pending and are retried
on the next status, payment, shipping or fulfillment mutation of that order; there is no separate
periodic retry job. Asynq handles delivery retries once scheduled.

The unique `(order_id, status)` event key and `(source_event, channel)` notification key prevent
repeat milestones and partial enqueue retries from creating duplicate channel rows. Queue retries
reuse the existing notification/task id; delivered rows are not scheduled again. Returning to a
previously visited status does not send it again. Existing orders are not backfilled. Delivery uses
the notification center's existing provider retry behavior; an external provider accepting a send
before a worker crash is not an exactly-once delivery guarantee.

## Provisioning

`ProvisionOrder` / `ProvisionSingleItem` create one card per order item, inside one transaction:

- `nfc_cards` row: `smart_code` (random, unique), `activation_code`, a system serial in `card_uid`
  (`CC-2026-XXXXXXXX`), `order_id`, `order_item_id`, and the item's profile if one was chosen.
- `routes` row `c/{smart_code}` with `route_type = card_route` and `is_immutable = true`.
- The item's `provisioning_status` becomes `provisioned` (or `failed` with the error).

Cancelled or refunded orders cannot be provisioned (409).

## Card identity is permanent

What is printed on the card or written to its chip never changes:

- The route path is immutable from creation (`protect_immutable_routes` trigger, migration 000030).
- `nfc_cards.smart_code` can never change, and `card_uid` cannot change once set
  (`protect_card_identity` trigger, migration 000043).

Everything else is editable by the owner: name, target profile or URL, lock, passcode, lead capture.

## Production hand-off (TEC-581)

From the order detail dialog, "Production files":

- **Manifest**: `POST /v1/platform/orders/{uuid}/manifest/export` queues an io-engine export
  (resource `platform.order_manifest`; XLSX/CSV/PDF/JSON). One row per card: order number, product,
  variant, SKU, recipient, card name, serial, card code, NFC URL (`{frontend}/c/{code}`), QR URL (`{frontend}/c/{code}?src=qr`),
  activation code, card status. It contains activation codes, so it needs step-up; the file lands in
  Exports and only its requester (or a platform admin) can download it.
- **QR pack**: `GET /v1/platform/orders/{uuid}/manifest/qr-codes` returns a ZIP of 1024px PNGs
  (`001-{code}.png`, …) encoding the manifest QR URL. Its `?src=qr` marker records `card.qr.scan` through
  the existing public resolver, preserving activation, lock, passcode and retirement behavior. Existing printed URLs keep working. No activation codes in it.

## Order list export (TEC-596)

The platform order list has an Export menu (`ResourceIOToolbar`, resource `platform.orders`):
`POST /v1/platform/orders/export` queues an io-engine job (PDF, XLSX, CSV or JSON) that lands in
Exports. It needs `platform.orders.export` (seeded for `super_admin`, migration 000083) and step-up.

- **Filters**: the same as `GET /v1/platform/orders`: `q` (order number, customer email, name or
  surname, tracking number), `status`, `payment_status`, `fulfillment_status`, `order_type`. The
  list has no sort or date-range parameters, so the export is newest first over all dates. Deleted
  orders are left out. At most 10,000 rows per file.
- **Columns**: order number, order type, status, payment status, fulfillment status, account and
  account type (personal/company), customer name and email, item count, grand total (two decimals)
  and currency, tracking number, created and updated time, UUID.
- Export only; orders are never imported. Adapter: `ioengine/adapters/orders.go`. The per-order
  production manifest above is a separate resource (`platform.order_manifest`).

## Product catalog import and export (TEC-596)

The platform product list has Export and Import (`ResourceIOToolbar`, resource
`platform.products`, adapter `ioengine/adapters/products.go`). Permissions
`platform.products.export` and `platform.products.import` are seeded for `super_admin`
(migration 000084); export also needs step-up.

- **Export**: `POST /v1/platform/products/export` (PDF, XLSX, CSV, JSON). Filters as on the list:
  `q` (name, slug, SKU), `status`, `product_type`; list order; at most 10,000 rows. The columns are
  the importable fields plus created/updated time and UUID, so an exported file can be edited and
  imported back.
- **Import**: `POST /v1/platform/products/import` (CSV, TSV, XLSX, JSON), sample at
  `GET .../import/sample`, then mapping, preview, confirm and rollback under
  `/v1/platform/imports/{uuid}`.
  - **Natural key: slug.** Live slugs are unique (`uq_products_slug_active`); SKU is optional and
    not unique, so it cannot identify a product. The slug is normalized as the product form does
    (`comcode Kart` → `comcode-kart`). A live product with that slug is updated; otherwise one is
    created.
  - **Columns**: `slug`, `name` (required), `sku`, `status`, `product_type`, `category` (slug or
    UUID of a live platform category), `base_price` (required for new products), `currency`,
    `tax_rate`, `stock_quantity`, `allow_custom_print`, `display_order`, `short_description`,
    `description`, `meta_title`, `meta_description`, `meta_keywords`. On update an empty cell keeps
    the current value. New products default to draft, `physical_card`, TRY, 20 % VAT, stock 0.
  - **Rules** (`productsusecase.ValidateProductInput`, the product form's rules): name 1–200
    characters; status `active`, `draft` or `archived`; price ≥ 0 and VAT 0–100, both with at most
    two decimals; stock a whole number ≥ 0; SKU ≤ 64; slug ≤ 150. The preview is a real dry run
    (`RowPreviewer`): every row shows create/update or its field errors, and nothing is written.
    Rows are written through the products use case (`Create`/`Update`), so slug handling and
    the search index behave as in the editor.
  - **Not imported**: variants, images, 3D models, SEO image and canonical URL. An update leaves
    them unchanged.
  - **Rollback** (within the import window): created products are deleted; updated products are
    rewritten field by field to their previous state, category included (also "no category").
  - The REST create/update endpoints do not run `ValidateProductInput` yet (they accept any value
    today); see the follow-up issue linked from TEC-596.

## Activation

The customer enters the activation code (`POST /v1/cards/activate`). The card moves to the
customer's active account and user, the activation code is cleared (single use), and the card route
becomes active. Card quotas are checked against the plan (`cards.max`).

## Cancel or refund (TEC-585)

All card routes of the order become `retired` in one statement (`RetireOrderCardRoutes`) and are
dropped from the resolver cache. Unused activation codes of those cards are cleared, so a card that
was never activated can no longer be claimed. A retired route resolves without a target URL; `/c/{code}` shows
"Card deactivated". Retirement is one-way, and the path is never handed out again.

An order paid online is refunded at the provider (iyzico/PayTR, TEC-935) before it becomes
`refunded`: moving it to `refunded` refunds what is left of its payment first, and a failed provider
refund leaves the order unchanged. Partial refunds keep the order and its cards. See "Card order
refunds" in [BILLING.md](./BILLING.md).

## Platform card and route support (TEC-950)

Platform staff find cards and routes across accounts at `/platform/support/cards` (by smart code,
card UID/serial or legacy serial) and `/platform/support/routes` (by path, deleted smart links and
renamed-away paths included, with `route_history`). Reads need `platform.cards.read` /
`platform.routes.read`. Actions need the matching `*.write` permission and a step-up grant:

| Action | Endpoint | Effect |
|--------|----------|--------|
| Retire a card | `POST /v1/platform/cards/{uuid}/retire` | Same as an order cancel for one card: route `retired`, unused activation code cleared. One-way. |
| Reassign a card | `POST /v1/platform/cards/{uuid}/reassign` | Activated cards only. Card and card route move to the profile, its account and owner (profile user, else account owner). Code and path unchanged. No plan quota check. |
| Release a quarantined path | `POST /v1/platform/routes/quarantine/release` | Ends the 30-day hold of deleted or renamed smart links on the path. |

Each writes `card.retired`, `card.reassigned` or `route.quarantine_released` to the activity log and
drops the affected resolver cache entries.

## Resolution and cache

`/v1/c/{code}`, `/v1/s/{slug}` and `/v1/r/{path}` go through `routes.Service.Resolve`, cached in
Redis (`app:<env>:route:path:*`, 1h). Card, profile and route writes invalidate the affected
entries. See §8.2 of the routing architecture doc.

## Search

Orders, products and company accounts appear in the platform Cmd+K palette
(`searchengine/adapters/commerce.go`); hits open `/platform/orders?open={uuid}` and
`/platform/products?open={uuid}`.
