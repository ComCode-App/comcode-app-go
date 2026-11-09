# Analytics Query Layer & Metrics API

This document describes the analytics query capabilities, aggregation rollups, and API endpoints for dashboards and reporting.

---

## 1. Metrics Calculation & Storage Model

To maintain sub-10ms query times on high-volume datasets, the system never aggregates raw event rows on-the-fly for dashboard endpoints.

### Aggregation Pipeline
1. **Raw Storage**: Raw immutable event rows are written to `events` partition.
2. **Real-time Counter Increment**: Redis keys (`analytics:{subject_type}:{subject_id}:today:{event_type}`) are incremented immediately.
3. **Hourly Rollup**: Upserted atomically into `event_aggregates_hourly` with JSONB breakdown merging.
4. **Daily Rollup**: Upserted atomically into `event_aggregates_daily` with JSONB breakdown merging.
5. **Idempotency Guarantee**: `event_processing_log` records `(event_id, occurred_at)` to prevent duplicate counts during queue retries.

---

## 2. Analytics Query Endpoints

All query endpoints are authenticated and operate on polymorphic subjects (`subject_type` and `subject_id`).

### 1. Metric Summary
`GET /v1/analytics/summary`

#### Query Parameters
- `subject_type` (required): e.g. `card`
- `subject_id` (required): e.g. `c79f9024-e910-410a-8d26-7fae879058b8`
- `event_type` (optional): Filter to a single event type (e.g. `card.view`)
- `from` (optional, ISO 8601): Start date (defaults to 30 days ago)
- `to` (optional, ISO 8601): End date (defaults to now)

#### Example Response
```json
{
  "success": true,
  "data": {
    "total_events": 48200,
    "unique_sessions": 31450,
    "top_events": [
      { "event_type": "card.view", "count": 32100 },
      { "event_type": "contact.phone.click", "count": 6400 },
      { "event_type": "vcard.download", "count": 5200 },
      { "event_type": "card.qr.scan", "count": 4500 }
    ],
    "today_live_count": 1420,
    "today_live_sessions": 890
  },
  "meta": { "request_id": "req_01j6g545" }
}
```

---

### 2. Time Series Timeline
`GET /v1/analytics/timeseries`

#### Query Parameters
- `subject_type` (required): Subject entity type
- `subject_id` (required): Subject entity ID
- `event_type` (optional): Specific event type
- `interval` (optional): `day` (default) or `hour`
- `from` (optional, ISO 8601)
- `to` (optional, ISO 8601)

#### Example Response
```json
{
  "success": true,
  "data": {
    "interval": "day",
    "total": 48200,
    "points": [
      {
        "bucket": "2026-08-01T00:00:00Z",
        "total_count": 1520,
        "unique_sessions": 980
      },
      {
        "bucket": "2026-08-02T00:00:00Z",
        "total_count": 1640,
        "unique_sessions": 1050
      }
    ]
  },
  "meta": { "request_id": "req_01j6g546" }
}
```

---

### 3. Dimensional Breakdown
`GET /v1/analytics/breakdown`

#### Query Parameters
- `subject_type` (required): Subject entity type
- `subject_id` (required): Subject entity ID
- `dimension` (required): Dimension to inspect (`country`, `city`, `device`, `browser`, `os`, `referer`)
- `event_type` (optional): Specific event type
- `from` (optional, ISO 8601)
- `to` (optional, ISO 8601)

#### Example Response (`dimension=country`)
```json
{
  "success": true,
  "data": {
    "dimension": "country",
    "total": 48200,
    "items": [
      { "key": "TR", "count": 28920, "percentage": 60.0 },
      { "key": "US", "count": 9640, "percentage": 20.0 },
      { "key": "DE", "count": 4820, "percentage": 10.0 },
      { "key": "GB", "count": 2410, "percentage": 5.0 },
      { "key": "Other", "count": 2410, "percentage": 5.0 }
    ]
  },
  "meta": { "request_id": "req_01j6g547" }
}
```

---

### 4. Raw Events Audit Drilldown
`GET /v1/analytics/events` (Protected by `platform.analytics.read`)

Allows paginated inspection of raw telemetry events for debugging, security auditing, and forensic analysis.

#### Query Parameters
- `subject_type` (optional)
- `subject_id` (optional)
- `event_type` (optional)
- `country` (optional)
- `from` (optional)
- `to` (optional)
- `limit` (default: 20, max: 100)
- `offset` (default: 0)

The response is `{ items, total, total_capped, has_more, limit, offset }` (items typed as
`AnalyticsRawEvent` in OpenAPI). The count is bounded (TEC-859): `total` counts at most 10,000
matching rows and `total_capped` is true when more match; `has_more` comes from fetching one extra
row, so paging still works past the cap. Narrow the filters (date range, subject) for exact totals.


QR attribution (TEC-569): generated QR payloads use the canonical public identity plus `?src=qr`,
never an API/versioned URL. Public pages forward this marker through the existing resolver or
visitor profile API via the BFF. The backend records `card.qr.scan` (card subject when present,
route subject otherwise) without changing activation, lock, passcode or retirement checks;
analytics failure never blocks the visit. Normal `route.scan`/page-view telemetry remains separate;
do not add these event types together to estimate unique scans. Unmarked QR/NFC visits are not
counted as QR scans. See the Smart QR section in `URL_ROUTING_AND_SMART_LINKS_ARCHITECTURE.md`.

### 5. Entry sources (TEC-953)
`GET /v1/analytics/sources` (same parameters, scoping and history clamp as the summary)

The owner dashboard (`/analytics`) reads its "Görüntülenme", "QR okutmaları" KPIs and the source
breakdown from here. It counts the subject **and the subjects that lead to it**, from the subject's
own account only: a profile brings its card routes, canonical route, smart links, legacy hash
routes and cards; a route its card; a card its routes.

- `views`: page openings only. For a profile `card.view` plus imported legacy page hits
  (`legacy.profile.view`); for a card or link the route scans that were not QR-opened. QR scans
  are never added to views.
- `qr_scans`: `card.qr.scan` of the subject and its related subjects.
- `sources`: always `qr`, `nfc`, `link`, `legacy` ("eski kart"), with count and percentage;
  `total_entries` is their sum.

Aggregates carry the source (`event_aggregates_hourly/daily.source`, migration 000064, part of the
rollup key), set by `domain.EntrySource` at aggregation time: `card.qr.scan` is `qr`;
`route.scan` follows its route type (`card_route` = `nfc`, `legacy_route` = `legacy`,
`custom_smart_link` = `link`) unless it carries `entry_source = qr` (a QR-opened route, already
counted by its `card.qr.scan`); imported `legacy.profile.view` rows are `legacy`. Other events have
`''`. The migration backfilled existing rows the same way; route scans from before it carry no QR
marker, so QR-opened card routes of that time stay counted as NFC.

---

## 3. Platform overview KPIs

`GET /v1/platform/overview` (TEC-594, `platform.overview.read`, granted to `super_admin` by
migration 000049) feeds the `/platform` home. It is platform-wide by design (no account scope) and
cached in Redis for 60 seconds; `generated_at` says when the numbers were computed.

| Field | Source | Definition |
|---|---|---|
| `users` | `users` | non-deleted; `active_last_30d` = `last_login_at` in the last 30 days |
| `profiles` | `profiles`, `routes` | non-deleted; `published` = has an `active` `canonical_profile` route |
| `cards` | `nfc_cards` | non-deleted; `activated` = `status = 'active'`; `activation_rate` = activated / total |
| `orders` | `orders` | created in the last 30 days, grouped by **current** status, every status listed in lifecycle order |
| `subscriptions` | `subscriptions`, `plans` | counts per status; `mrr` per currency = active subscriptions, online (`source = payment`) and manual alike, on `month` plans + `year` plans / 12 (free and lifetime excluded, `trialing` and `past_due` excluded); `manual_amount_minor` is the share of manual subscriptions; `at_risk_mrr` is the same sum for `past_due` subscriptions, shown as its own "at-risk MRR" card and never added to `mrr` (TEC-944) |
| `top_profiles` | `event_aggregates_daily` | top 10 non-deleted profiles by `card.view` on `subject_type = 'profile'` over the last 30 UTC days |
| `events_last_24h` | `event_aggregates_hourly` | the current hour and the 23 before it, zero-filled, plus the top 8 event types |

The analytics figures come from the aggregate tables only; raw event partitions are never scanned.
Aggregates are filled by the worker, so events still in the queue are not counted yet.

**Conversion funnel (TEC-944).** `GET /v1/platform/overview/funnel?days=7|30|90|365` (default 30,
same permission) reads the orders created in the period and their `order_status_history` (see
[COMMERCE_AND_ORDERS.md](./COMMERCE_AND_ORDERS.md#status-history-tec-944)). Stages: `created`,
`paid`, `in_production`, `shipped`, `delivered`. An order reaches a stage when it ever entered
that status or a later lifecycle status, so an order moved straight from `pending` to
`in_production` (payment tracked on `payment_status`) still counts as paid. Per stage: orders,
`conversion_from_previous`, `conversion_from_created`, and `avg_hours_from_created` (creation to
the first recorded entry; null without recorded entries). Backfilled rows count as reached but
are left out of the averages. `cancelled` counts the period's orders now cancelled or refunded.
The fixed periods keep the query bounded (`idx_orders_created_at`, then the history index per
order); each period is cached in Redis for 60 s.

Platform staff without `platform.overview.read` (it stays `super_admin` only) see an empty state
on `/platform` with links to the areas their permissions open (order queue, orders, profile
verifications, products, subscriptions, analytics) instead of a permission error.

### Platform analytics overview (TEC-859)

`GET /v1/platform/analytics/overview?tz=Europe/Istanbul` (`platform.analytics.read`) feeds
`/platform/analytics`. It sums `event_aggregates_hourly` over every subject into local calendar
days of `tz` (IANA name, default UTC) and returns `today`, `last_7_days`, `last_30_days` and a
30-point zero-filled `daily` series. It replaced eight raw-event `COUNT(*)` calls from the browser.
TEC-953 adds `unique_visitors` (`today`, `last_7_days`, `last_30_days` over the same local-day
windows) and `top_event_types` (the 10 most frequent event types of the 30-day window, from the
hourly rollups). Unique visitors come from `analytics_visitors_hourly`: the aggregator writes one
row per visitor and UTC hour (visitor key = SHA-256 of the session id, else of the rotating IP
hash), so distinct counts never read raw events. Rows older than 35 days are pruned by the
retention sweep. A visitor without a session counts again after an IP salt rotation.
Differences from the old raw counts: events still in the queue are not included until the worker
rolls them up, and zones with a non-whole-hour offset (e.g. UTC+5:30) are approximated by hourly
buckets. The "latest events" card still reads 8 raw rows.


---

## 4. Company analytics (TEC-574)

Company → Analytics (`/company/analytics`, `frontend/src/features/company-analytics`) shows a
company account's totals, a daily series, a dimension breakdown and a per-employee comparison.
Code: `backend/internal/modules/companyanalytics`, queries in
`internal/database/queries/company_analytics.sql`.

**No account-level events or rollups.** The pipeline is polymorphic, so nothing new is written:
the company's numbers are the sum of the existing `event_aggregates_daily` rows of the account's
own live subjects, resolved at query time:

| Subject | Attributed to the employee profile |
|---|---|
| `profile` (public page events) | the profile itself |
| `route` (`route.scan`: card routes, canonical routes, smart links) | `routes.profile_id` |
| `card` (`card.qr.scan`) | `nfc_cards.profile_id` |
| leads (`leads` table, not an event) | `leads.profile_id`, else the card's, else the route's profile |

Writing `subject_type=account` events would have duplicated every event (and the per-subject
dashboards already read the subject rows), could not answer office/group filters or per-employee
questions, and would freeze attribution at event time. The trade-off of query-time attribution:
history follows the **current** target, so reassigning a card moves its past taps to the new
employee, and deleted profiles/cards/routes drop out. Raw `events` are never scanned, so there is
no unique-visitor count; today's numbers are whatever the worker has aggregated so far.

**Metrics** (`total_events` counts every aggregated event; the others are subsets of it):
`profile_views` = `card.view`; `card_taps` = `route.scan` on `card_route`/`legacy_route` routes;
`qr_scans` = `card.qr.scan`; `contact_saves` = `vcard.download`; `link_clicks` = `route.scan` on
`custom_smart_link` routes; `contact_clicks` = `contact.*`, `social.*`, `bank.*`; `leads` = lead
submissions created in the range (UTC days).

**Endpoints** (permission `app.company_analytics.read`, migration 000055, granted to `subscriber`):

| Route | Returns |
|---|---|
| `GET /v1/company/analytics/summary` | `totals`, `top_events` (10), `profiles`, `active_profiles`, effective `from`/`to`, `history_days` |
| `GET /v1/company/analytics/timeseries` | one zero-filled point per UTC day with every metric |
| `GET /v1/company/analytics/breakdown` | `dimension` (`country`, `city`, `device`, `browser`, `os`, `referer`), optional `event_type`, `limit` 1–50 (default 10); `total` is over every value |
| `GET /v1/company/analytics/employees` | list envelope, one row per live profile in scope (with linked member, office, group), `share_percent` of the scope's total events; `q`, `sort` (`name` or a metric, `-` for descending, default `-total_events`), `limit`/`offset` |

All take `from`/`to` (inclusive `YYYY-MM-DD`, default the last 30 days, at most 366 days) and
`office_uuid`/`group_uuid`. `from` is clamped to the plan's `analytics.history_days` exactly like
the per-subject endpoints (`X-Analytics-History-Days` on summary/timeseries).

**Scope.** Only company accounts; the role is reloaded from the database on every request and all
queries of a request run in one read-only repeatable-read transaction. Owner/admin: every live
subject of the company; without an office/group filter this includes subjects that point at no
live profile (e.g. a smart link to a URL), so totals can exceed the sum of the employee rows.
Office manager: only profiles linked to members assigned to a currently granted office (and the
cards/routes/leads attributed to them); a filter can narrow but never widen that. Plain members and
personal accounts get 403. Another account's subjects are never joined (every subject query is
`account_id`-scoped, also a foreign route that targets one of our profiles).
