# ComCode

Digital business card platform: NFC/QR cards, public profile pages (`/{username}`), smart links, vCard, visitor analytics, plans and billing, built on a Go HTTP API, a Next.js BFF frontend and Docker infra.

Planning lives in **Linear only** (project ComCode). Reference issues as `COM-123` (formerly `TEC-123`) in commits and docs; do not keep plans or TODO lists in the repo.

| Path | Role |
|------|------|
| [`backend/`](./backend) | Go HTTP API + Asynq worker (`:8080`, `/v1/...`) |
| [`frontend/`](./frontend) | Next.js App Router BFF (`/platform`, app `/`, public `/{username}`, `/c/{code}`, `/s/{slug}`) |
| [`compose.local.yml`](./compose.local.yml) | Local infra only |
| [`compose.prod.yml`](./compose.prod.yml) | Full stack + Dokploy network |
| [`docs/`](./docs) | Architecture, local/prod setup, how to extend |
| [`AGENTS.md`](./AGENTS.md) | Agent skill index |

## Product

- **Profiles** — multiple profiles per account (tr/en), vCard import/export, media in object storage
- **Public pages** — `/{username}` per profile with visible sections, published bank accounts, Background Studio backgrounds, SEO/OG ([docs/PUBLIC_PROFILES.md](./docs/PUBLIC_PROFILES.md))
- **Cards & links** — NFC/QR cards (`/c/{code}`), smart links (`/s/{slug}`), one routes registry ([docs/URL_ROUTING_AND_SMART_LINKS_ARCHITECTURE.md](./docs/URL_ROUTING_AND_SMART_LINKS_ARCHITECTURE.md))
- **Commerce** — product catalog, orders, card provisioning, iyzico/PayTR billing, plans and subscriptions
- **Entitlement** — plan quotas per account (profiles, smart links, active cards, analytics history)
- **Analytics** — visitor events, aggregates, owner dashboard, GeoIP ([docs/ANALYTICS.md](./docs/ANALYTICS.md), [docs/EVENTS.md](./docs/EVENTS.md), [docs/GEOIP.md](./docs/GEOIP.md))
- **Webhooks** — HMAC-signed deliveries of lead, scan and order events to the account's endpoints, with retries and a delivery log ([docs/WEBHOOKS.md](./docs/WEBHOOKS.md))

## Platform foundation

- **Auth** — register, login, refresh, logout, profile, password reset, email verify, passkeys, GitHub link (optional)
- **Accounts** — personal + company accounts, multi-membership with owner/admin/member roles, active-account switcher ([docs/TENANCY_AND_ACCESS.md](./docs/TENANCY_AND_ACCESS.md))
- **Global RBAC** — roles + permissions (`user_roles`) for platform and feature access
- **Platform panel** (`/platform`) — users, roles, notifications, activity, logs, storage, imports, exports, letterhead settings, access (step-up), GitHub integration
- **App shell** (`/`) — the account's profiles, cards, smart links, analytics, orders, billing
- **Notifications** — in-app, email, realtime, optional Web Push
- **Realtime** — Centrifugo connection/subscription JWTs
- **Queue** — Asynq (in-process by default, or `make -C backend worker`)
- **Search** — Cmd+K palette + Meilisearch (browser never talks to Meili)
- **Storage** — S3 storage browser (SeaweedFS); the API owns public URLs (never assemble keys in the browser)
- **I/O engine** — async CSV/XLSX/PDF/JSON export and import with rollback
- **Bulk engine** — permission-gated batch mutations with optional undo
- **Activity + logs** — audit trail and application log viewer with purge rules
- **Step-up** — password/passkey re-auth on sensitive actions (`/platform/access`)
- **Impersonation** — platform admin can act as another user (`platform.users.impersonate`)

## Quick start

```sh
cp .env.example .env
# set NAME_PREFIX if you share a Docker host with other stacks
make local-dev
```

In another terminal (after Postgres is up):

```sh
make create-super-admin
# or: make create-super-admin SA_EMAIL=you@example.com SA_PASSWORD='Secret1'
```

Then open http://localhost:3000/platform/login and sign in as `super_admin`.

`make local-dev` starts Postgres, Redis, SeaweedFS, MailHog, Adminer, Centrifugo, and Meilisearch in Docker, migrates the database, then runs **air** (API) and **pnpm dev** (frontend) on the host. It does **not** build app images.

| Who | Lands on |
|-----|----------|
| `super_admin` or any `platform.*` permission | `/platform` |
| Other authenticated users | `/` (CMS) |

## Docs

| Doc | When to read |
|-----|----------------|
| [docs/ARCHITECTURE.md](./docs/ARCHITECTURE.md) | Repo map, shells, RBAC, request flow |
| [docs/DEVELOPMENT.md](./docs/DEVELOPMENT.md) | Env, local commands, production / Dokploy |
| [docs/EXTENDING.md](./docs/EXTENDING.md) | Adding a module, API + UI conventions |
| [CONTRIBUTING.md](./CONTRIBUTING.md) | How to work in this repo |
| [backend/README.md](./backend/README.md) | API commands and curl examples |
| [frontend/README.md](./frontend/README.md) | BFF, panels, scripts |
| [AGENTS.md](./AGENTS.md) | Which agent skill to load |

Agent playbooks live under [`.agents/skills/`](./.agents/skills).
