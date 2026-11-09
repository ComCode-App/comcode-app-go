# Analytics Runtime Configuration & Administration

This document describes the runtime configuration options, database-backed settings, cache invalidation mechanisms, and RBAC permissions for the Analytics subsystem.

---

## 1. Runtime Settings Overview

The analytics subsystem is configured via the `analytics_settings` singleton table in PostgreSQL and cached in memory / Redis.

### Configuration Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `enabled` | boolean | `true` | Master switch for analytics ingestion and processing |
| `event_delivery_mode` | string | `async` | Ingestion mode: `async` (via queue), `sync` (direct write), `hybrid` (queue with sync fallback) |
| `raw_event_retention_days` | integer | `730` | Retention period in days for raw `events` partitions (2 years) |
| `aggregate_retention_days` | integer | `1095` | Retention period in days for summary aggregate tables (3 years) |
| `queue_name` | string | `analytics` | Asynq queue name for analytics tasks |
| `realtime_enabled` | boolean | `true` | Whether to maintain real-time Redis counters for live metrics |
| `batch_size` | integer | `200` | Target batch size for event ingestion worker |
| `batch_flush_interval_ms` | integer | `1000` | Maximum wait time in milliseconds before flushing pending batch |
| `ip_hash_salt_rotation_days` | integer | `7` | Rotation interval for salted IP hashing (GDPR compliance), 1–30 days (TEC-936) |
| `anonymize_ip` | boolean | `true` | Enforces IP anonymization before hashing |

### IP hash salt rotation (TEC-936)

`ip_hash_salt_rotation_days` (1–30, CHECK constraint since migration 000064; the API answers 400
outside the range) drives the collector's `privacy.RotatingSaltHasher`. A salt period is a run of
whole UTC days counted from the Unix epoch (`unix_day / days`), so a period never splits a day.

A change applies from the **next UTC day**. The update stores the period in force on the day of
the change in `ip_hash_salt_rotation_previous_days` and the day in
`ip_hash_salt_rotation_changed_on` (both read-only in the API); `RotationSchedule.DaysOn` uses the
previous period up to and including that day. Every hash of one day therefore uses the same salt,
however often the setting changes that day (a second change on the same day keeps the original
previous period). The hasher reads the cached settings (`AnalyticsService.SaltRotationSchedule`,
1 minute in memory, Redis key `analytics:settings` cleared on update). The platform config page
says when a change is pending.

---

## 2. Admin API Endpoints

### Get Configuration: `GET /v1/platform/analytics/config`
Requires `platform.analytics.read` permission.

```json
{
  "success": true,
  "data": {
    "enabled": true,
    "event_delivery_mode": "async",
    "raw_event_retention_days": 730,
    "aggregate_retention_days": 1095,
    "queue_name": "analytics",
    "realtime_enabled": true,
    "batch_size": 200,
    "batch_flush_interval_ms": 1000,
    "ip_hash_salt_rotation_days": 7,
    "anonymize_ip": true
  }
}
```

### Update Configuration: `PATCH /v1/platform/analytics/config`
Requires `platform.analytics.manage` permission.

```json
{
  "event_delivery_mode": "async",
  "raw_event_retention_days": 365,
  "ip_hash_salt_rotation_days": 7
}
```

---

## 3. RBAC Permissions Matrix

| Permission Slug | Name | Description |
|-----------------|------|-------------|
| `platform.analytics.read` | Read Analytics | Allows viewing analytics dashboards, raw events, and runtime settings |
| `platform.analytics.manage` | Manage Analytics | Allows updating analytics runtime configuration and retention rules |
| `platform.geoip.read` | Read GeoIP Settings | Allows viewing GeoIP status, database version, and configuration |
| `platform.geoip.manage` | Manage GeoIP Settings | Allows triggering GeoIP updates, cache flushes, and credential changes |
| `platform.events.manage` | Manage Event Definitions | Allows creating, updating, or disabling event definition schemas |
