# Webhooks

Outbound webhooks (TEC-603): an account registers HTTPS endpoints, subscribes them to events,
and the platform POSTs a signed JSON body for every matching event, retrying with backoff until
the endpoint answers 2xx. Code: `backend/internal/modules/webhooks` (API, fan-out, delivery),
`internal/queue` (`app:webhook:deliver` task on the `webhooks` queue), frontend
`src/features/webhooks` (`/webhooks` in the app shell).

## Who can do what

| Route | Who | Purpose |
|---|---|---|
| `GET /v1/webhooks/events` | `app.webhooks.read` | Subscribable events (registry) |
| `GET /v1/webhooks`, `GET /v1/webhooks/{uuid}` | `app.webhooks.read` (any member) | Endpoints of the active account |
| `POST /v1/webhooks`, `PUT`, `DELETE /v1/webhooks/{uuid}` | `app.webhooks.write` + owner/admin | Register, change, delete (at most 10 endpoints per account) |
| `POST /v1/webhooks/{uuid}/rotate-secret` | same | New signing secret, returned once |
| `POST /v1/webhooks/{uuid}/test` | same | Queue a `webhook.test` delivery |
| `GET /v1/webhooks/{uuid}/deliveries?status=` | `app.webhooks.read` | Delivery log, newest first |
| `POST /v1/webhooks/{uuid}/deliveries/{deliveryUuid}/resend` | `app.webhooks.write` + owner/admin | Send again from attempt zero |

Both slugs are granted to `subscriber` and `super_admin` (migration `000071`). The secret is
stored encrypted (`crypto.SecretBox`) and never returned after creation or rotation; the API
shows its last six characters (`secret_hint`).

## Events

| Event | Sent when | `data` |
|---|---|---|
| `lead.created` | A contact form lead or a contact exchange is stored (`modules/leads`) | `lead_uuid`, `kind` (`form` / `exchange`), `source_type`, `route_path`, `locale`, `locked`, `created_at`; when not locked also `name`, `email`, `phone`, `message`, `company`, `job_title`. A lead over the monthly quota (TEC-941) is `locked` and carries no contact details, like the panel. |
| `card.scanned` | A card, smart link or legacy hash route is resolved by a visitor, or a profile's own QR is scanned (`modules/routes`) | `route_uuid`, `path`, `route_type`, `target_type`, `source` (`qr`, `nfc`, `link`…), `scanned_at`, optional `card_uuid`, `card_name`, `profile_uuid`, `profile_username`. Visitor device and location stay in analytics. |
| `order.created` | The first milestone (`pending`) of a card order | Order snapshot (below) with `milestone: pending` |
| `order.status_changed` | Every later milestone: `paid`, `design_pending`, `in_production`, `shipped`, `delivered`, `cancelled`, `refunded`… (`modules/orders`, same trigger as the customer notifications) | `order_uuid`, `order_number`, `order_type`, `milestone`, `status`, `payment_status`, `grand_total`, `currency`, `shipping_carrier`, `tracking_number`, `tracking_url`, `items[]` (`uuid`, `product_name`, `quantity`, `recipient_name`), `created_at` |
| `webhook.test` | "Send test" in the panel | `message`, `endpoint_uuid`, `sent_at` |

Adding an event: a constant and a `Registry` entry in `modules/webhooks/model`, a `Dispatch`
call where the domain write is committed, a row here and in the frontend `webhooks.events.*`
strings.

## Request

```http
POST /hooks/comcode HTTP/1.1
Content-Type: application/json
User-Agent: ComCode-Webhooks/1.0
X-ComCode-Event: lead.created
X-ComCode-Delivery: 7c9e6679-7425-40de-944b-e07fc1f90ae7
X-ComCode-Event-Id: 2b0d1f4e-…
X-ComCode-Signature: t=1761500000,v1=5f1a…

{"id":"7c9e…","event":"lead.created","event_id":"2b0d…","occurred_at":"2026-10-26T17:00:00Z","attempt":1,"data":{…}}
```

- `id` is the delivery; `event_id` is shared by every endpoint's delivery of the same event, so
  a receiver subscribed twice can deduplicate on it. `attempt` counts retries of this delivery.
- **Signature.** `v1` is `hex(HMAC-SHA256(secret, "<t>.<raw body>"))` where `t` is the Unix
  time the request was signed. Verify against the raw bytes before parsing JSON, compare with a
  constant-time function, and reject `t` older than a few minutes to stop replays.
  `usecase.Verify` in `modules/webhooks/usecase/signature.go` is a reference implementation.
- Answer any 2xx within 10 seconds (the body is ignored, at most 4 KB is read). Anything else,
  including 3xx (redirects are not followed), a timeout or a TLS error, counts as a failure.

## Delivery and retries

`Dispatch` is fail-soft: the caller's own write is already committed, so a webhook problem is
logged and never fails a lead, scan or order. It runs one indexed query per event; an account
with no active endpoint subscribed to the event costs nothing else. Every subscribed endpoint
gets its own `webhook_deliveries` row (the audit log) and an Asynq task on the `webhooks` queue.

The worker (`Service.Deliver`, in-process or `cmd/worker`) POSTs the body and records the attempt:
`succeeded`, or `failed` with the next attempt scheduled by the service itself (Asynq retries are
off): 30 s, 1 m, 2 m, 4 m … capped at 6 h, 8 attempts in all (about 9 hours), then `exhausted`.
Each attempt first claims the row (pending or failed, retry time reached), so a task that fires
early or twice posts nothing; the Asynq task id is `webhook:<delivery>:<attempt>`, so a lost
task is re-queued by the sweep without duplicates. Resend applies to `succeeded` and `exhausted`
deliveries only (a pending or failed one still has its task; the API answers 409
`DELIVERY_PENDING`) and resets it to attempt zero. A deactivated or deleted endpoint turns pending
deliveries `exhausted` when they are picked up; a secret that can no longer be decrypted does
the same with an error asking for a rotation. An endpoint with 500 undelivered events drops new
ones (logged) until the queue drains, and `card.scanned` is emitted at most once per route every
10 seconds per API process, so a bot or link preview cannot flood a receiver. `consecutive_failures` on the endpoint counts
failed attempts since the last success and is reset by a success or by re-activating the
endpoint; the panel shows it so a dead endpoint is visible.

The log purge sweep also prunes delivery rows older than 30 days (`Service.Prune`) and re-queues
deliveries that have been due for more than 10 minutes without an attempt (`Service.Sweep`).

## URL safety

Endpoint URLs must be public `https://` addresses: no credentials, no fragment, no loopback,
private (RFC 1918, CGNAT, ULA), link-local or metadata hosts. The check runs on the saved URL
and again at dial time on the resolved IP (`NewHTTPClient`), so DNS cannot point a hook back at
the API's own network. `development` and `test` environments allow `http://` and local hosts so
a local receiver can be used.

## Panel

`/webhooks` (app shell, nav item under the account section): endpoints with their events,
status and last delivery; create/edit dialog (URL, description, events, active); the secret shown
once after create or rotate; a deliveries drawer per endpoint with status, attempts, HTTP code,
error and a resend button; "send test".
