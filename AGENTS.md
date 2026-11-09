# Agent entry

ComCode is a digital business card platform (profiles, public pages, NFC/QR cards, smart links, analytics, billing). Planning lives in **Linear only**; reference issues as `COM-123` (formerly `TEC-123`).

Read this first, then the matching skill. Human docs live under `docs/`. Resources belong to **accounts** (personal + company), see [docs/TENANCY_AND_ACCESS.md](./docs/TENANCY_AND_ACCESS.md). Do not invent workspaces or a `/business` shell.

Canonical env is the **repo-root** `.env`. Compose files are `compose.local.yml` (infra) and `compose.prod.yml` (full stack).

## Skills (`.agents/skills/`)

| Skill | When |
|-------|------|
| [boilerplate-first-setup](./.agents/skills/boilerplate-first-setup/SKILL.md) | Clone, `.env`, Docker names, production env (`make prod-env-init`), Dokploy, `make local-dev` |
| [monorepo-playbooks](./.agents/skills/monorepo-playbooks/SKILL.md) | Cross-app work; repo map |
| [add-platform-resource](./.agents/skills/add-platform-resource/SKILL.md) | New list resource (module → RBAC → OpenAPI → UI → nav) |
| [nav-engine](./.agents/skills/nav-engine/SKILL.md) | Sidebar catalog, badges, info |
| [io-engine](./.agents/skills/io-engine/SKILL.md) | Export / import adapters |
| [bulk-engine](./.agents/skills/bulk-engine/SKILL.md) | Bulk actions, jobs, rollback |
| [search-engine](./.agents/skills/search-engine/SKILL.md) | Cmd+K palette, Meilisearch adapters |
| [storage-media](./.agents/skills/storage-media/SKILL.md) | Object storage, media URLs |
| [github-integration](./.agents/skills/github-integration/SKILL.md) | GitHub App OAuth, encrypted credentials |
| [step-up-engine](./.agents/skills/step-up-engine/SKILL.md) | Re-auth gates (password/passkey) on sensitive actions |
| [admin-date-picker](./.agents/skills/admin-date-picker/SKILL.md) | Any date field in `frontend/` |
| [db-migration](./.agents/skills/db-migration/SKILL.md) | New table/column/seed/query: migration (ComCode numbering) → `account_tables.txt` → sqlc → OpenAPI |
| [verify-changes](./.agents/skills/verify-changes/SKILL.md) | Checks to run before commit / PR (mirrors CI) |

## Human docs

| Doc | When |
|-----|------|
| [docs/ARCHITECTURE.md](./docs/ARCHITECTURE.md) | Shells, request flow, stores, RBAC |
| [docs/TENANCY_AND_ACCESS.md](./docs/TENANCY_AND_ACCESS.md) | Accounts, account roles, active account, fail-closed scope |
| [docs/PUBLIC_PROFILES.md](./docs/PUBLIC_PROFILES.md) | `/{username}` pages, usernames, reserved paths, visitor API, backgrounds |
| [docs/COMMERCE_AND_ORDERS.md](./docs/COMMERCE_AND_ORDERS.md) | Orders, provisioning, production manifest, activation, cancel/retire |
| [docs/BILLING.md](./docs/BILLING.md) | Plans, subscriptions, iyzico/PayTR checkout and callbacks, what is not automated |
| [docs/COMPANIES.md](./docs/COMPANIES.md) | Company accounts, member management, platform account admin, suspension |
| [docs/ENTITLEMENTS.md](./docs/ENTITLEMENTS.md) | Plan limits, quota guards, adding a feature |
| [docs/PROFILES.md](./docs/PROFILES.md) | Owner-side profile editing, media, vCard, completion rate |
| [docs/URL_ROUTING_AND_SMART_LINKS_ARCHITECTURE.md](./docs/URL_ROUTING_AND_SMART_LINKS_ARCHITECTURE.md) | Routes registry, card routes, smart links |
| [docs/ANALYTICS.md](./docs/ANALYTICS.md) · [ANALYTICS_ARCHITECTURE](./docs/ANALYTICS_ARCHITECTURE.md) · [ANALYTICS_CONFIGURATION](./docs/ANALYTICS_CONFIGURATION.md) | Visitor analytics, aggregates, dashboard |
| [docs/EVENTS.md](./docs/EVENTS.md) | Event definitions, ingestion (`POST /v1/events`) |
| [docs/GEOIP.md](./docs/GEOIP.md) | GeoIP resolution |
| [docs/WEBHOOKS.md](./docs/WEBHOOKS.md) | Outbound webhooks: endpoints, events, signature, retries |
| [docs/OBSERVABILITY.md](./docs/OBSERVABILITY.md) | `X-Request-ID` correlation (BFF → API → queue), 5xx/panic and job-failure logs, Sentry/GlitchTip error tracking (DSNs managed in the admin panel, `SENTRY_DSN` fallback; lazy catcher on visitor pages) and Prometheus (`METRICS_ADDR`) |
| [docs/DEVELOPMENT.md](./docs/DEVELOPMENT.md) | Local / prod commands |
| [docs/EXTENDING.md](./docs/EXTENDING.md) | How to add a feature (narrative), incl. new env vars |
| [docs/OWNER_CHECKLIST.md](./docs/OWNER_CHECKLIST.md) | Owner-only GitHub / GHCR / Dokploy settings (agents never change them) |
| [docs/design/env-core.md](./docs/design/env-core.md) | Env schema, `prod-env.sh`, env CI checks, boot guard |
| [docs/design/account-deletion.md](./docs/design/account-deletion.md) | Self-service account deletion design (proposal, owner decisions pending) |
| [docs/BOILERPLATE_SYNC.md](./docs/BOILERPLATE_SYNC.md) | Boilerplate sync record: last reviewed SHA, per-commit PORT / ZATEN VAR / ATLA decisions, method for the next sync |
| [backend/API_CONVENTIONS.md](./backend/API_CONVENTIONS.md) | Envelope, lists, meta |
| [backend/DATABASE_RULES.md](./backend/DATABASE_RULES.md) | Migrations + sqlc |
| [backend/docs/auth.md](./backend/docs/auth.md) | JWT, roles, permissions |

## Branches and releases

- Branch `cc/<issue-id>-<short-name>` from the latest `cc-collective-branch`; open the PR into `cc-collective-branch`, never into `main`. The owner merges `cc-collective-branch` into `main` with one PR. Flow and required `ci` check: [docs/DEVELOPMENT.md](./docs/DEVELOPMENT.md) → CI → Branch flow.
- Only a push to `main` publishes GHCR images and cuts the `v0.N` tag Dokploy deploys. Never push `:main` / `:sha-*` / `:v0.*` images or `v0.*` git tags by hand, and never change repository or Dokploy settings.

## Hard rules

- Browser talks to same-origin `/api/v1/*`, never the Go host.
- Media URLs are API-owned. Never assemble S3 (SeaweedFS) keys in the browser.
- Never expose Meilisearch to the browser; search goes through `/v1/search`.
- New `/v1` routes update `backend/docs/openapi.yaml` in the same change (CI checks it: `backend/scripts/openapi_routes.py`).
- Permission slugs are added only via migration + `internal/platform/rbac` + `frontend/src/config/permissions.ts`.
- Owned app data carries `account_id`. App use cases/repositories take the active account (`authctx.Principal.Account`) as a required scope; never a nil "all rows" filter. Writes need the `app.*.write` permission **and** an owner/admin account role. Platform routes use separate unscoped methods.
- Account isolation lint (`backend/internal/database/queries_test.go`): every sqlc query on a table in `backend/internal/database/account_tables.txt` filters that table by `account_id` (a parameter, IN list, subquery or `IS NULL`; a bare join such as `a.id = p.account_id` does not count), or carries `-- account-lint: cross-account <reason>` directly above its `-- name:` line. A migration that creates an account-owned table adds it to `account_tables.txt` in the same change.
- After OpenAPI changes: `cd frontend && pnpm api:generate`.
- Production env comes from `deploy/env/schema` (`make prod-env-init`), never from a copy of `.env.example`. New env vars follow `docs/EXTENDING.md` → "Environment variables"; `.env.server.example` is generated (`scripts/prod-env.sh example`), never hand-edited. `make env-check` must pass.
