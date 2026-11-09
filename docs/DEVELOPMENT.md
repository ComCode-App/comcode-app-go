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

1. Create the external network once: `docker network create dokploy-network`
2. `cp .env.example .env.server` and replace every secret (`${VAR:?…}` is required)
3. `make prod-config` then `make prod-up`

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
- `NEXT_PUBLIC_*` values are baked into the frontend image at build time: rebuild after changing them.

### Object storage (SeaweedFS)

`comcode-seaweedfs` (`seaweedfs` in `compose.local.yml`) runs master, volume, filer and the S3 gateway in one container (`chrislusf/seaweedfs`, pinned). `comcode-seaweedfs-init` creates `S3_BUCKET` through the S3 API.

- Only the S3 port (8333) listens outside the container. Master, volume and filer have no auth and are bound to the container loopback.
- `S3_ACCESS_KEY` / `S3_SECRET_KEY` are the admin identity. Anonymous users may only `GET` objects in `S3_BUCKET` (public media); listing, writes and deletes need the key.
- In Dokploy, route the media domain to service `comcode-seaweedfs`, port `8333` (was `minio:9000`); see Dokploy domains.
- Any other S3-compatible store works with `STORAGE_DRIVER=s3` and `S3_ENDPOINT`. `minio` is accepted as a legacy alias.
- Driver check against a live store: `STORAGE_IT_ENDPOINT=http://127.0.0.1:8333 S3_ACCESS_KEY=… S3_SECRET_KEY=… S3_BUCKET=… go test ./internal/platform/storage -run TestS3CompatibleBackend`.
- Moving from MinIO: there is no prod data. Locally, copy objects with any S3 client (`rclone sync minio:bucket seaweed:bucket`) or start fresh; the old `*-minio` volume can be removed.

`APP_ENCRYPTION_KEY` (32-byte, raw or base64) encrypts GitHub App secrets at rest. Generate: `openssl rand -base64 32`. Replace the example key in production.

## CI

GitHub-hosted minutes are not used. `.github/workflows/ci.yml` runs on a **self-hosted macOS runner** (`runs-on: [self-hosted, comcode]`) installed on the dev machine as a launchd service.

| Item | Value |
|------|-------|
| Runner dir | `~/actions-runner-comcode` |
| Labels | `self-hosted`, `macOS`, `ARM64`, `comcode` |
| Service | `./svc.sh status` / `start` / `stop` (from the runner dir) |
| Logs | `~/Library/Logs/actions.runner.furkanmeclis-comcode-app-go.*` and `_diag/` |

Jobs queue while the machine is asleep or the service is stopped. Go module cache and pnpm store persist on the host, so remote caches are disabled.

### Postgres integration tests

Tests that call `pgtest.Open` (`backend/internal/testutil/pgtest`) run only when `TEST_DATABASE_URL` is set and are skipped otherwise. Each test package gets a fresh database cloned from a template that has every migration applied (golang-migrate). The backend job starts a throwaway `postgres:18-alpine` container on `127.0.0.1:56432`, runs `go test -coverpkg=./... -coverprofile=…`, writes the total coverage to the job summary and always removes the container. Locally:

```sh
docker run -d --name pg-it -e POSTGRES_USER=test -e POSTGRES_PASSWORD=test -e POSTGRES_DB=test -p 127.0.0.1:56432:5432 postgres:18-alpine
cd backend && TEST_DATABASE_URL=postgres://test:test@127.0.0.1:56432/test?sslmode=disable go test ./...
docker rm -f pg-it
```

New integration test packages add `func TestMain(m *testing.M) { pgtest.Main(m) }` so the package database is dropped afterwards. Tests in one package share that database: create your own rows with the fixture helpers (`CreateUser`, `CreateProduct`, `CreateCard`) instead of assuming empty tables.

Re-register (e.g. new machine):

```sh
cd ~/actions-runner-comcode
./config.sh remove --token "$(gh api -X POST repos/furkanmeclis/comcode-app-go/actions/runners/remove-token --jq .token)"
./config.sh --unattended --url https://github.com/furkanmeclis/comcode-app-go \
  --token "$(gh api -X POST repos/furkanmeclis/comcode-app-go/actions/runners/registration-token --jq .token)" \
  --name "$(scutil --get LocalHostName)-comcode" --labels comcode,macOS,ARM64
./svc.sh install && ./svc.sh start
```

### E2E smoke tests

Playwright tests in `frontend/e2e` cover the critical paths: password login, login with an authenticator (TOTP), profile creation, a subscriber card order, marking it paid in `/platform/orders/pipeline` (which provisions the card), and activating that card from `/cards`.

`scripts/e2e.sh` runs them the way CI's `e2e` job does. It starts Postgres, Redis and SeaweedFS from `compose.local.yml` under its own project name and ports (56433, 56380, 58333), migrates, runs the API on `:58080` and the production Next.js build on `:53000`, then removes everything on exit. It can run next to `make local-dev`. Extra arguments go to `playwright test` (`scripts/e2e.sh auth.spec.ts`). `E2E_SKIP_BUILD=1` reuses the last `pnpm build`. Stack logs land in `frontend/e2e/.state/logs`.

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
