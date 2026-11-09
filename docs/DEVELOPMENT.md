# Development

## Prerequisites

- Go 1.22+ (match `backend/go.mod`)
- Node 20.9+ and pnpm
- Docker + Compose
- golang-migrate, sqlc, air, golangci-lint, gofumpt — see [backend/TOOLCHAIN.md](../backend/TOOLCHAIN.md)

## First clone

```sh
cp .env.example .env
```

Set these **before** sharing a Docker host with another stack:

| Variable | Purpose |
|----------|---------|
| `NAME_PREFIX` | Container names (`${NAME_PREFIX}-postgres`, `-seaweedfs`, …) |
| `COMPOSE_PROJECT_NAME` | Compose project name |
| `LOCAL_NETWORK_NAME` | Local bridge |
| `APP_NETWORK_NAME` | Prod private bridge (default `${NAME_PREFIX}-app`) |
| `*_VOLUME_NAME` | Named volumes so SeaweedFS/Postgres do not collide |

Never reference short Docker DNS names (`seaweedfs`, `centrifugo`, `backend`) on the shared Dokploy overlay: other stacks use the same names. `compose.prod.yml` reaches services by prefixed container name (`${NAME_PREFIX}-seaweedfs`) or `<service>.${APP_NETWORK_NAME}`, and the object store's service key itself is `comcode-seaweedfs`.

Root `.env` is canonical. `backend/.env` is a fallback only. Do not keep a long-lived `frontend/.env.local` for API URLs; `API_URL` comes from the root env.

`API_URL` **must** include `/v1` (Go routes are `/v1/...`). Default: `http://127.0.0.1:8080/v1`.

## Local

```sh
make local-dev
make create-super-admin
```

Helpers:

| Target | What it does |
|--------|----------------|
| `make infra` / `make infra-down` | Compose infra only |
| `make backend-dev` | air on `:8080` |
| `make frontend-dev` | `pnpm dev` on `:3000` |
| `make migrate-up` | Apply SQL migrations |
| `make create-super-admin` | Upsert platform admin + `super_admin` role |
| `make search-reindex` | Rebuild Meilisearch indexes (users, roles, …) |
| `make check-i18n` | Find missing / unwired translation keys (en+tr, activity, RBAC, exports) |

### Local URLs

| Service | URL |
|---------|-----|
| Frontend | http://localhost:3000 |
| API | http://127.0.0.1:8080 |
| Scalar docs | http://127.0.0.1:8080/docs/ |
| Health / ready | http://127.0.0.1:8080/healthz · `/readyz` |
| Adminer | http://127.0.0.1:8081 |
| SeaweedFS S3 | http://127.0.0.1:8333 (bucket `S3_BUCKET`, keys `S3_ACCESS_KEY` / `S3_SECRET_KEY`) |
| MailHog | http://127.0.0.1:8025 |
| Meilisearch | http://127.0.0.1:7700 |
| Centrifugo WS | ws://127.0.0.1:8000/connection/websocket |

Login: http://localhost:3000/platform/login

## Branding

Swap name and colors in `frontend/src/config/brand.ts` and the CSS tokens in the frontend theme. Default product name is **App**.

## VAPID (optional Web Push)

```sh
npx web-push generate-vapid-keys
```

Set `VAPID_PUBLIC_KEY`, `VAPID_PRIVATE_KEY`, and `VAPID_SUBJECT` in `.env`. The frontend registers via `/app-push-sw.js` → BFF → `/v1/notifications/vapid-public-key` + push-subscriptions.

Closed-browser push does not work until these three are set.

`TRUSTED_PROXIES` (CIDRs, default loopback + private ranges) lists the peers whose `X-Forwarded-For` the API honours; see `backend/docs/auth.md` → Rate limits.

Optional frontend public vars (root `.env`): `NEXT_PUBLIC_BFF_BASE_URL`, `NEXT_PUBLIC_REALTIME_ENABLED`, `NEXT_PUBLIC_CENTRIFUGO_URL`, `NEXT_PUBLIC_DEFAULT_LOCALE`, `NEXT_PUBLIC_FALLBACK_LOCALE`.

## Observability (optional)

Error tracking (`SENTRY_DSN`, which works with Sentry or GlitchTip) and Prometheus metrics (`METRICS_ADDR`, a separate internal listener) are off when unset. `X-Request-ID` correlation is always on. Look up `meta.request_id` from an error response in the logs and the platform log viewer. See [OBSERVABILITY.md](./OBSERVABILITY.md).

## Production / Dokploy

1. Create the external network once (plain Docker host only; Dokploy creates it): `docker network create dokploy-network`
2. `make prod-env-init` generates `.env.server` from `deploy/env/schema` (see Environment file below). Never start from a copy of `.env.example`.
3. `make prod-config` then `make prod-up`

### Environment file

The production env is generated from `deploy/env/schema`; `.env.server.example` is that schema rendered (`scripts/prod-env.sh example`), never hand-edited. Design: [design/env-core.md](./design/env-core.md). Needs bash 4.3+ and openssl (macOS: `brew install bash`).

1. `make prod-env-init` asks for `APP_URL` (the frontend origin, e.g. `https://comcode.technowide.software`) and offers defaults for `NAME_PREFIX` (first host label), `IMAGE_REPO` (`ghcr.io/comcode-app/comcode-app-go`), the realtime URL (`wss://realtime.<host>/connection/websocket`) and the media URL (`https://media.<host>/<S3_BUCKET>`). It derives every other URL (`AUTH_URL`, `PUBLIC_FRONTEND_URL`, `CORS_ALLOWED_ORIGINS`, `AUTH_WEBAUTHN_RP_ID`, …), generates every secret and the VAPID pair, and writes `.env.server` (mode 600, git-ignored). Non-interactive: `make prod-env-init ARGS="--set APP_URL=https://app.example.com --yes"`.
2. Fill `SMTP_*` in `.env.server`. Everything under "Optional tuning" may stay blank: a blank value keeps the default from `compose.prod.yml` / `backend/internal/config`. iyzico, PayTR, GeoIP (MaxMind), analytics and the error-tracking DSNs are configured in the admin panel, not in env.
3. `make prod-config` then `make prod-up`. On Dokploy, paste the file into the compose app's environment and add the domains below.

Images: `IMAGE_REPO` + `IMAGE_TAG` (`main`, `v0.N` or `sha-<short>`) name the backend and frontend images. `compose.prod.yml` still reads `BACKEND_IMAGE` / `FRONTEND_IMAGE`, which `init` derives from them; change all of them together (`make prod-env-check` reports a mismatch).

Keep the generated file as the master copy (password manager or the server) and run the commands below on it; values edited only in the Dokploy UI are invisible to `check`.

| Task | Command |
|---|---|
| After pulling a schema change | `make prod-env-upgrade`: keeps every value, applies renames/removals, fills only new keys, re-stamps `ENV_SCHEMA_VERSION`, backup in `.env.server.bak` |
| Verify a file | `make prod-env-check`: exits 1 on a schema-version gap, missing or blank keys, removed keys still present (schema v2 dropped `BACKEND_IMAGE` / `FRONTEND_IMAGE`), or secrets still holding `.env.example` values |
| Rotate a secret on purpose | `scripts/prod-env.sh upgrade --rotate JWT_REFRESH_SECRET` (confirmation; restart the stack). Refuses `APP_ENCRYPTION_KEY` (encrypted data becomes unreadable) and `DB_PASSWORD` (Postgres keeps it in its volume: `ALTER USER` first, then edit the file) |
| Older hand-made `.env.server` | `make prod-env-upgrade` (`APP_URL` defaults to the file's `AUTH_URL`), then `make prod-env-check`, then rotate every secret it reports as insecure. Rotation is never automatic |

### Compose images: pulled, never built on the server

`compose.prod.yml` has no `build:` sections. It runs `${IMAGE_REPO}-backend:${IMAGE_TAG}` for `migrate`, `backend` and `worker`, and `${IMAGE_REPO}-frontend:${IMAGE_TAG}` for `frontend`, and fails at `config` time when `IMAGE_REPO` or `IMAGE_TAG` is missing. CI publishes the images to GHCR.

- `pull_policy` is `${IMAGE_PULL_POLICY:-always}`: every `up` / redeploy fetches the tag again, so a moving tag such as `:main` picks up the newest build. Set `IMAGE_PULL_POLICY=missing` to reuse a cached image.
- Pin or roll back by setting `IMAGE_TAG` to `v0.N` or `sha-<short>` and redeploying. Volumes, container names (`${NAME_PREFIX}-*`), the app network and `dokploy-network` do not depend on the tag.
- `make prod-up` runs `up -d` without `--build`.
- Build on this machine instead (smoke test, no registry): `make prod-build-local` / `make prod-up-local`. They add [`compose.build.yml`](../compose.build.yml), which puts the `build:` contexts back (the same ones as `deploy/images.json`) and tags the images `${NAME_PREFIX}-backend:local` / `${NAME_PREFIX}-frontend:local`, so a local build never shadows a GHCR tag. Dokploy never uses this file.
- `stop_grace_period`: `backend` 30s (the server drains requests for up to 10s on SIGTERM), `worker` 35s (asynq waits for running jobs). Docker's 10s default could SIGKILL either mid-shutdown.
- Upgrading an existing `.env.server` from schema v1: `make prod-env-upgrade` comments out `BACKEND_IMAGE` / `FRONTEND_IMAGE` (`IMAGE_REPO` / `IMAGE_TAG` are already there); on Dokploy remove the two lines from the compose app's environment too.

### Required secrets (boot guard)

`APP_ENV` must be `development`, `test`, `staging` or `production`; any other value stops the backend at boot (`compose.prod.yml` sets `production`). In `staging` and `production` the backend and worker also refuse to start while a secret below is empty, shorter than its minimum, or still a public default (the `.env.example` / `compose.local.yml` values, the fallbacks in `config.go`, or a placeholder such as `change_me`). The error names every offending variable, never its value. Development and test keep the defaults, so `make local-dev` and `scripts/e2e.sh` are unchanged.

| Variable | Minimum length | Checked when |
|----------|----------------|--------------|
| `JWT_ACCESS_SECRET`, `JWT_REFRESH_SECRET` | 32 | always |
| `AUTH_ADAPTER_SECRET` | 32 | always |
| `APP_ENCRYPTION_KEY` | 32 (32 raw bytes or base64 of 32 bytes) | always |
| `DB_PASSWORD`, `REDIS_PASSWORD` | 12 | always |
| `S3_ACCESS_KEY` / `S3_SECRET_KEY` | 8 / 16 | always |
| `CENTRIFUGO_API_KEY` / `CENTRIFUGO_TOKEN_HMAC_SECRET` | 16 / 32 | `CENTRIFUGO_ENABLED=true` |
| `MEILI_MASTER_KEY` | 16 | `SEARCH_ENABLED=true` |

`make prod-env-init` generates all of them (long enough for every minimum). The list lives in one table, `guardedSecrets` in `backend/internal/config/guard.go`; `schema_test.go` fails when a `secret:` key of `deploy/env/schema` has no row there (or a row has no schema key, or the generated length is below the minimum). iyzico and PayTR credentials are not env vars: they are stored in the database, encrypted with `APP_ENCRYPTION_KEY`.

`compose.prod.yml` now requires `APP_ENCRYPTION_KEY` (`${APP_ENCRYPTION_KEY:?…}`). Before, an empty value made the backend fall back to the public example key.

> **Warning: existing deployments without `APP_ENCRYPTION_KEY`.** If production ran without `APP_ENCRYPTION_KEY` (or with the example value), every value encrypted at rest was encrypted with the public key `app-dev-encryption-key-32bytes!!`: OAuth provider and GitHub App secrets, TOTP secrets, payment provider (iyzico / PayTR) keys, webhook and error-tracking secrets. Setting a new key alone makes those values unreadable. Before deploying this guard, either re-encrypt them with the new key or re-enter them through the admin UI after the deploy (TOTP users would have to enroll again), and treat the old values as exposed: rotate the provider-side credentials. There is no key-rotation tool in the repo yet. The same key also seeds the analytics visitor hash, so unique-visitor counts restart from the new key.

Dokploy-attached services: **comcode-seaweedfs, centrifugo, backend, frontend**. Postgres, Redis, Meilisearch, and the worker stay on the private app network.

### Dokploy domains

Dokploy → the compose app → **Domains**: pick the service and the **container port**, enable HTTPS (Let's Encrypt), path `/`. Each domain needs a DNS `A` record to the server.

| Domain (example) | Service | Container port | Env vars that must match (`.env.server`) |
|------------------|---------|----------------|-------------------------------------------|
| `comcode.technowide.software` | `frontend` | `3000` | `AUTH_URL=https://comcode.technowide.software`, `PUBLIC_FRONTEND_URL=https://comcode.technowide.software`, `CORS_ALLOWED_ORIGINS=https://comcode.technowide.software`, `AUTH_WEBAUTHN_RP_ID=comcode.technowide.software` |
| `media.comcode.technowide.software` | `comcode-seaweedfs` | `8333` | `STORAGE_PUBLIC_BASE_URL=https://media.comcode.technowide.software/<S3_BUCKET>` (path-style: the bucket is the first path segment) |
| `realtime.comcode.technowide.software` | `centrifugo` | `8000` | `CENTRIFUGO_WS_URL` and `NEXT_PUBLIC_CENTRIFUGO_URL` = `wss://realtime.comcode.technowide.software/connection/websocket`, `CENTRIFUGO_ALLOWED_ORIGINS=https://comcode.technowide.software` |
| — (no domain) | `backend` | `8080` | The browser only talks to the BFF; the frontend reaches Go at `http://${NAME_PREFIX}-backend:8080/v1` |

- The media domain serves anonymous GETs only (public media). Do not route it to the filer or master ports; they are not listening outside the container.
- `NEXT_PUBLIC_*` values are inlined when CI builds the frontend image, so setting them in `.env.server` does not change a pulled image (the code defaults apply: `/api`, realtime on, `tr` / `en`). The realtime URL the browser uses comes from the backend at runtime (`CENTRIFUGO_WS_URL`, returned as `ws_url` with the Centrifugo token).

### Object storage (SeaweedFS)

`comcode-seaweedfs` (`seaweedfs` in `compose.local.yml`) runs master, volume, filer and the S3 gateway in one container (`chrislusf/seaweedfs`, pinned). `comcode-seaweedfs-init` creates `S3_BUCKET` through the S3 API.

- Only the S3 port (8333) listens outside the container. Master, volume and filer have no auth and are bound to the container loopback.
- `S3_ACCESS_KEY` / `S3_SECRET_KEY` are the admin identity. Anonymous users may only `GET` objects in `S3_BUCKET` (public media); listing, writes and deletes need the key.
- In Dokploy, route the media domain to service `comcode-seaweedfs`, port `8333` (was `minio:9000`); see Dokploy domains.
- Any other S3-compatible store works with `STORAGE_DRIVER=s3` and `S3_ENDPOINT`. `minio` is accepted as a legacy alias.
- Driver check against a live store: `STORAGE_IT_ENDPOINT=http://127.0.0.1:8333 S3_ACCESS_KEY=… S3_SECRET_KEY=… S3_BUCKET=… go test ./internal/platform/storage -run TestS3CompatibleBackend`.
- Moving from MinIO: there is no prod data. Locally, copy objects with any S3 client (`rclone sync minio:bucket seaweed:bucket`) or start fresh; the old `*-minio` volume can be removed.

`APP_ENCRYPTION_KEY` (32-byte, raw or base64) encrypts OAuth, GitHub App, TOTP, payment and webhook secrets at rest. `make prod-env-init` generates it; never rotate it once data is stored. The backend refuses to start outside development with the example key (see Required secrets).

## CI

`.github/workflows/ci.yml` runs on **GitHub-hosted runners** (`ubuntu-24.04`) on pushes to `main` and `cc-collective-branch`, on every pull request (any base) and on manual runs (`workflow_dispatch`). Feature branches are tested through their pull request. A newer push to the same branch or PR cancels the running one (never on `main`).

Job graph:

```
changes ─┬─ backend ─────┐
         ├─ frontend ────┤
         ├─ e2e ─────────┤
         ├─ env ─────────┤
         ├─ image-build ─┼─ ci (single required check)
         ├─ mobile ──────┤
         └─ mobile-release
auto-pr (disabled)
```

| Job | Runs when | What |
|-----|-----------|------|
| `changes` | always | Path filter (`git diff` against the PR base, the previous push on `cc-collective-branch`, or the merge-base with `main`). Docs, `*.md` and other `.github/*` files trigger nothing; a change to `ci.yml` itself, a push to `main` and a manual run trigger everything. |
| `backend` | `backend/**` | `sqlc generate` + generated code must be committed, golangci-lint (pinned v2), Postgres integration tests with coverage in the job summary, Redocly OpenAPI lint, `scripts/openapi_routes.py`. |
| `frontend` | `frontend/**` or `backend/**` (contracts) | install, typecheck, lint, unit tests, `scripts/check-i18n.mjs`. |
| `e2e` | backend or frontend change, `scripts/e2e.sh`, `compose.local.yml`, `.env.example` | Playwright smoke suite via `scripts/e2e.sh`; report and stack logs uploaded on failure. |
| `env` | `deploy/env/**`, `compose*.yml`, `.env.example`, `.env.server.example`, `Makefile`, the env scripts, `backend/internal/config/**`, or a backend diff line with an env read | `make env-check` (no Docker daemon needed): `.env.server.example` matches the schema, `.env.example` has every schema key, every backend env read is in the schema, `scripts/prod-env.test.sh`, and `scripts/check-compose-env.sh` (+ its tests): every backend env read reaches the backend/worker containers or has a decision in `deploy/env/compose-env.allow`. See [design/env-core.md](./design/env-core.md). |
| `image-build` | app or infra change, not on `main` | Builds every image in `deploy/images.json` with buildx, no push. GHCR publishing on `main` comes with TEC-1094. |
| `mobile`, `mobile-release` | only when the repo variable `MOBILE_ENABLED` is `true` (and `mobile/**` / backend changed) | Expo checks and EAS release; ComCode has no `mobile/` yet. Enable under Settings → Secrets and variables → Actions → Variables. |
| `ci` | always | Fails if any needed job failed or was cancelled; skipped jobs (path-filtered or mobile gated off) pass. Make this the required status check. |
| `auto-pr` | never (`if: false`) | Ported from the boilerplate but off: in ComCode all work goes through PRs into `cc-collective-branch` and the owner merges to `main` by hand. |

Caches (all remote, GitHub Actions cache): Go modules and build cache (`setup-go`, keyed on `backend/go.sum`), pnpm store (`setup-node` `cache: pnpm`, keyed on `frontend/pnpm-lock.yaml`), golangci-lint, Playwright browsers, and buildx layers (`type=gha`, one scope per image).

### Postgres integration tests

Tests that call `pgtest.Open` (`backend/internal/testutil/pgtest`) run only when `TEST_DATABASE_URL` is set and are skipped otherwise. Each test package gets a fresh database cloned from a template that has every migration applied (golang-migrate). The backend job runs `postgres:18-alpine` as a job service container (data on tmpfs, `fsync`/`synchronous_commit`/`full_page_writes` off), runs `go test -coverpkg=./... -coverprofile=…` against it and writes the total coverage to the job summary. Locally:

```sh
docker run -d --name pg-it -e POSTGRES_USER=test -e POSTGRES_PASSWORD=test -e POSTGRES_DB=test -p 127.0.0.1:56432:5432 postgres:18-alpine
cd backend && TEST_DATABASE_URL=postgres://test:test@127.0.0.1:56432/test?sslmode=disable go test ./...
docker rm -f pg-it
```

New integration test packages add `func TestMain(m *testing.M) { pgtest.Main(m) }` so the package database is dropped afterwards. Tests in one package share that database: create your own rows with the fixture helpers (`CreateUser`, `CreateProduct`, `CreateCard`) instead of assuming empty tables.

### E2E smoke tests

Playwright tests in `frontend/e2e` cover the critical paths: password login, login with an authenticator (TOTP), profile creation, a subscriber card order, marking it paid in `/platform/orders/pipeline` (which provisions the card), and activating that card from `/cards`.

`scripts/e2e.sh` runs them the way CI's `e2e` job does. It starts Postgres, Redis and SeaweedFS from `compose.local.yml` under its own project name and ports (56433, 56380, 58333), migrates, runs the API on `:58080` and the production Next.js build on `:53000`, then removes everything on exit. It can run next to `make local-dev`. Extra arguments go to `playwright test` (`scripts/e2e.sh auth.spec.ts`). `E2E_SKIP_BUILD=1` reuses the last `pnpm build`. Stack logs land in `frontend/e2e/.state/logs`. With `CI` set on Linux (the GitHub-hosted runner) it installs Chromium with its system libraries (`playwright install --with-deps`).

Against your running dev stack instead:

```sh
make create-super-admin            # admin@example.com / Password1, used to create test users
cd frontend && pnpm e2e
```

How the suite works:

- `e2e/global-setup.ts` creates three fresh users per run through the platform users API: a subscriber, a subscriber with TOTP enabled, and a super admin operator. Unique emails keep runs independent of plan quotas and per-email login rate limits, so the database never has to be reset.
- The UI runs in English. Each test user's saved locale is `en`.
- Activation codes are never returned by the order or card APIs, so the order test reads the provisioned card from Postgres. Settings: `E2E_DATABASE_URL`, or the `DB_*` variables.
- Other settings: `E2E_BASE_URL` (default `http://localhost:3000`), `E2E_API_URL` (default `API_URL`), and `E2E_ADMIN_EMAIL` / `E2E_ADMIN_PASSWORD`.

## Super admin

From the repo root (loads `.env`):

```sh
make create-super-admin
make create-super-admin SA_EMAIL=you@example.com SA_PASSWORD='Str0ngPassword' SA_NAME=Ada SA_SURNAME=Admin
```

The same target exists under `backend/` (`make -C backend create-super-admin …`). The make target keeps a local-only default (`admin@example.com` / `Password1`); the binary itself has no defaults.

The binary requires an email (`-email` or `SA_EMAIL`) and a password (`-password` or `SA_PASSWORD`) that passes the platform password policy (at least 8 characters with upper case, lower case and a digit); `-name` / `-surname` are optional. Running it again for the same email resets that user's password and restores the role.

Production has no source tree or `make`; the backend image ships the binary at `/app/create-super-admin`. After the first deploy, on the host:

```sh
read -rs SA_PASSWORD && export SA_PASSWORD
docker compose --env-file .env.server -f compose.prod.yml exec -e SA_PASSWORD backend \
  /app/create-super-admin -email you@example.com
unset SA_PASSWORD
```

In Dokploy, open a terminal on the backend container and run `/app/create-super-admin -email you@example.com -password '…'`.

## After a backend contract change

```sh
make -C backend sqlc
make -C backend openapi-lint
cd frontend && pnpm api:generate && pnpm typecheck
```
