# Architecture

This repo is a two-app monorepo plus Compose files. Dependencies point inward: HTTP handlers and Next.js pages call use cases / services; they do not own SQL or SMTP.

## Layout

```
.
├── backend/                      Go API + worker
│   ├── cmd/                      server, worker, create-super-admin
│   ├── internal/modules/         product: profiles, profileverifications, cards,
│   │                             routes (usernames, smart links, QR), backgrounds,
│   │                             bankaccounts, banks, socialplatforms, products,
│   │                             categories, orders, billing, plans, subscriptions,
│   │                             analytics, events, geoip, accounts, leads, utils
│   │                             company: companystructure (sectors, offices,
│   │                             groups, assignments), companyprofiles,
│   │                             companyassets (cards, orders), companyanalytics,
│   │                             companybranding
│   │                             platform: auth, authsettings, notifications,
│   │                             exports, imports, bulk, settings, activity, logs,
│   │                             storage, access, search, integrations, overview,
│   │                             platformsupport, errortracking
│   ├── internal/platform/        jwt, rbac, authctx, entitlement, publicpath,
│   │                             clientip, ratelimit, events, analytics, geoip,
│   │                             storage, mail, outbox, payment, vcard, iban,
│   │                             ioengine, bulkengine, activity, resourcemeta,
│   │                             stepup, searchengine, crypto, legacymigration
│   ├── migrations/               sequential golang-migrate SQL
│   └── docs/openapi.yaml         OpenAPI 3.1 (Scalar at /docs/)
├── frontend/                     Next.js BFF
│   ├── src/app/layout.tsx        root: locale, theme, toasts only (PublicProviders)
│   ├── src/app/(shell)           panel providers (session, query, step-up, realtime, dialogs)
│   │   ├── (platform)            /platform admin
│   │   ├── (app)                 / App shell; company management under /company/*
│   │   ├── (guest)               login, reset, verify
│   │   └── (public), share       forbidden/unauthorized, shared files
│   ├── src/app/[username]        public profile page (SSR), outside (shell): keep it lean
│   ├── src/app/c, src/app/s      card and smart link redirects, outside (shell)
│   ├── src/app/api/v1            BFF proxy → Go
│   └── src/features/             product: profiles, profile-verifications,
│                                 public-profile, cards, smart-links, route-qr,
│                                 analytics, background-studio, bank-accounts,
│                                 banks, social-platforms, sectors, orders,
│                                 billing, plans, subscriptions, products,
│                                 categories, entitlements, accounts, leads,
│                                 account, auth, auth-settings
│                                 company: company-structure, company-bulk,
│                                 company-analytics, company-branding
│                                 platform: users, roles, companies, notifications,
│                                 platform-overview, platform-analytics,
│                                 platform-support, error-tracking, geoip, io,
│                                 bulk-engine, storage, logs, settings, nav-engine,
│                                 step-up-engine, search-engine, access, integrations
├── compose.local.yml             infra only
└── compose.prod.yml              full stack
```

## Shells

| Surface | Path | Who |
|---------|------|-----|
| Platform | `/platform` | `super_admin` or any `platform.*` permission |
| App | `/` | `subscriber` or any role without platform perms |
| Public | `/{username}`, `/c/{code}`, `/s/{slug}` | anyone (no login) |
| Guest | `/platform/login` (aliases also exist at `/login`) | signed-out users |

The app shell acts in the session's **active account** (personal or company) and shows an account switcher; there is no `/business` shell. JWT claims are `sub`, `roles`, `is_super_admin`, `exp`, `sid` (session family), `acc` (active account), optional `imp` (impersonator). See [TENANCY_AND_ACCESS.md](./TENANCY_AND_ACCESS.md).

## Request flow

```
Browser
  → same-origin /api/v1/*          Next.js BFF (HttpOnly session)
  → $API_URL/...                   Go API (Bearer JWT)
  → use case → repository (sqlc)   PostgreSQL
```

- Access and refresh tokens never enter JavaScript. The BFF stores them in HttpOnly cookies and strips them from JSON responses.
- NextAuth is the frontend session layer; the Go adapter talks to `/v1/internal/auth/*`.
- OpenAPI types are generated into `frontend/src/generated/api.d.ts` via `pnpm api:generate`.
- Security headers for every Next route live in `frontend/src/config/security-headers.ts` (TEC-1102). No page can be framed (`X-Frame-Options: DENY`, `frame-ancestors 'none'`), public profiles included (TEC-1119); only the storage previews the app frames itself allow `'self'`. Invite-token pages add `Referrer-Policy: no-referrer` in `next.config.ts` (TEC-1104).

## Backend layers

```
handler → usecase → repository interface → sqlc/pgx → PostgreSQL
```

Cross-module signals go through `internal/platform/events` (in-process, fail-soft). Handlers never call SMTP or SMS; they enqueue via the notification service (`app:notification:deliver` on the `notifications` queue).

Audit: `activity.Recorder.Record` (fail-soft). Application logs: `internal/modules/logs`.

## Platform engines

| Engine | Package | Job |
|--------|---------|-----|
| Resource meta | `internal/platform/resourcemeta` | `GET /v1/.../meta` — columns, filters, capabilities |
| I/O | `internal/platform/ioengine` | Export / import adapters; jobs on Asynq `exports` / `imports` |
| Bulk | `internal/platform/bulkengine` | Batch mutations; jobs on Asynq `bulk` |
| Search | `internal/platform/searchengine` | Meilisearch indexing + `GET /v1/search` for Cmd+K |
| Storage | `internal/platform/storage` + `modules/storage` | Object bytes in S3 (SeaweedFS); metadata in Postgres |
| Activity | `internal/platform/activity` | Audit events for mutations and I/O |
| Step-up | `internal/platform/stepup` | Re-auth grant (password/passkey) before sensitive routes |
| Secrets | `internal/platform/crypto` | AES-256-GCM box for GitHub App credentials |

Adapters register in `internal/httpserver/server.go` and `cmd/worker/main.go`.

## Data stores

| Store | Role |
|-------|------|
| PostgreSQL 18 | Source of truth |
| Redis | Cache, Asynq, ephemeral state |
| S3 (SeaweedFS) | Object bytes; metadata stays in Postgres |
| Meilisearch | Cmd+K record search; HMAC/master key stays on the API |
| Centrifugo | Websocket fan-out; HMAC stays on the API |

Media: the browser must only use API-returned `public_url` / stream paths. See `frontend/src/lib/media/urls.ts`.

## Platform home

`/platform` renders `GET /v1/platform/overview` (`modules/overview`, `platform.overview.read`):
platform-wide KPIs for users, profiles, cards, orders by status, subscriptions/MRR and at-risk MRR,
the most viewed profiles and the last 24 hours of events, plus the order conversion funnel from
`GET /v1/platform/overview/funnel`. It reads the analytics aggregate tables only and is cached in
Redis for 60 s (`app:<env>:platform:overview:v2`, `…:funnel:v1:<days>`); with Redis down every
request computes it. Staff without the permission get links to their own work instead. Details:
[ANALYTICS.md §3](./ANALYTICS.md#3-platform-overview-kpis).

## RBAC

Users are global. Permissions are the union of assigned roles. App data is additionally scoped to the active account and gated by the account role (owner/admin/member) — see [TENANCY_AND_ACCESS.md](./TENANCY_AND_ACCESS.md).

| Seed role | Notes |
|-----------|--------|
| `super_admin` | System role (`is_system=true`). Created by `make create-super-admin`, not a user column. |
| `subscriber` | `auth.session` + `notifications.read` + `app.*` feature permissions |

Custom roles and permission assignment live under `/v1/platform/roles`. New permission slugs are added only via migrations. Details: [backend/docs/auth.md](../backend/docs/auth.md).
