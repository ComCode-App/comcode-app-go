# Polymorphic Event Specification & Registry

This document outlines the polymorphic event envelope, event definition registry, schema validation rules, and context extraction pipeline.

---

## 1. Event Envelope Specification

All events in the system conform to the canonical `Event` envelope:

```json
{
  "uuid": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "type": "card.view",
  "schema_version": 1,
  "subject": {
    "type": "card",
    "id": "c79f9024-e910-410a-8d26-7fae879058b8"
  },
  "actor": {
    "type": "user",
    "id": "usr_99812"
  },
  "session_id": "sess_88b1f8e21a",
  "request_id": "req_01j6g530h8",
  "occurred_at": "2026-08-30T12:30:00Z",
  "created_at": "2026-08-30T12:30:00.125Z",
  "properties": {
    "source": "qr",
    "campaign": "tech-expo-2026",
    "utm_medium": "nfc"
  },
  "context": {
    "ip_hash": "a1f94d93e18a0029384bcda839201948",
    "country_code": "TR",
    "country_name": "Turkey",
    "city": "Istanbul",
    "timezone": "Europe/Istanbul",
    "latitude": 41.0082,
    "longitude": 28.9784,
    "device_type": "mobile",
    "device_brand": "Apple",
    "device_model": "iPhone",
    "browser": "Safari",
    "browser_version": "17.5",
    "os": "iOS",
    "os_version": "17.5",
    "referer": "https://instagram.com/p/123",
    "user_agent": "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X)..."
  },
  "metadata": {
    "source": "http_api",
    "delivery_mode": "async",
    "ingested_at": "2026-08-30T12:30:00.002Z"
  }
}
```

---

## 2. Event Ingestion API

### Endpoint: `POST /v1/events`
Publicly available ingestion endpoint for web applications, mobile SDKs, and physical touchpoints (NFC/QR).

#### Request Body
```json
{
  "type": "card.view",
  "subject": {
    "type": "card",
    "id": "c79f9024-e910-410a-8d26-7fae879058b8"
  },
  "session_id": "sess_88b1f8e21a",
  "occurred_at": "2026-08-30T12:30:00Z",
  "properties": {
    "source": "qr"
  }
}
```

#### Response (`202 Accepted`)
```json
{
  "success": true,
  "data": {
    "uuid": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
    "type": "card.view",
    "status": "accepted",
    "created_at": "2026-08-30T12:30:00.002Z"
  },
  "meta": {
    "request_id": "req_01j6g530h8"
  }
}
```

---

## 3. Event Definition Registry & Validation

Event definitions govern authorization, schema validation, aggregation eligibility, and data retention.

### Registry Fields
- `key`: Unique dot-separated event identifier (`[a-z0-9._-]+`).
- `category`: Grouping for catalog visualization (`card`, `contact`, `auth`, `billing`, `social`).
- `is_public`: `true` allows ingestion via public `POST /v1/events`. If `false`, only trusted internal backend services can emit the event.
- `enabled`: Global kill-switch for an event type.
- `aggregate_enabled`: Whether background workers include this event in hourly/daily rollups.
- `retention_days`: Custom retention window for raw event pruning.
- `required_properties`: List of mandatory property keys in `event.properties`.

### Built-in Seeded Event Definitions

| Key | Category | Public | Aggregate | Required Properties | Description |
|-----|----------|--------|-----------|---------------------|-------------|
| `card.view` | `card` | Yes | Yes | — | Visitor viewed digital card |
| `card.qr.scan` | `card` | Yes | Yes | — | Visitor scanned QR code |
| `contact.phone.click` | `contact` | Yes | Yes | — | Visitor clicked phone call button |
| `contact.email.click` | `contact` | Yes | Yes | — | Visitor clicked email link |
| `contact.website.click` | `contact` | Yes | Yes | — | Visitor clicked external website |
| `social.instagram.click` | `social` | Yes | Yes | — | Visitor clicked Instagram link |
| `vcard.download` | `contact` | Yes | Yes | — | Visitor downloaded contact vCard |
| `user.login` | `auth` | No | Yes | — | User authenticated |
| `user.logout` | `auth` | No | Yes | — | User logged out |
| `subscription.started` | `billing` | No | Yes | — | Subscription created/activated |
| `subscription.cancelled` | `billing` | No | Yes | — | Subscription cancelled |
| `payment.completed` | `billing` | No | Yes | `amount`, `currency` | Payment transaction succeeded |
| `payment.failed` | `billing` | No | Yes | — | Payment transaction failed |

---

## 4. Administrative Event Definition Endpoints

Authorized platform administrators (`platform.events.manage`) can manage event schemas via:
- `GET /v1/platform/events/definitions` (List with optional `category`, `enabled`, `is_public` filters)
- `POST /v1/platform/events/definitions` (Register new event type)
- `GET /v1/platform/events/definitions/{id}` (Get definition by ID)
- `PATCH /v1/platform/events/definitions/{id}` (Update metadata, status, or validation rules)
- `DELETE /v1/platform/events/definitions/{id}` (Remove definition)
