---
name: verify-changes
description: >-
  Run the same checks CI runs (backend sqlc/lint/vet/test, OpenAPI lint and
  route check, frontend typecheck/lint/test/i18n, env checks) before
  committing or opening a PR. Use before saying a change is done.
---

# Verify changes

CI (`.github/workflows/ci.yml`) runs path-gated jobs on every push and pull request; `ci` is the single required check. Run the matching subset locally first.

## Backend (any `backend/**` change)

```bash
cd backend
gofmt -l .            # must print nothing
sqlc generate && git diff --exit-code -- internal/database/db   # CI fails on stale sqlc output
golangci-lint run     # v2.x, same version as CI (backend/TOOLCHAIN.md)
go vet ./...
go test ./...         # includes the account isolation lint (internal/database/queries_test.go)
```

Integration tests need Postgres (CI uses a service container):

```bash
docker run -d --rm --name comcode-test-pg -e POSTGRES_USER=comcode -e POSTGRES_PASSWORD=comcode \
  -e POSTGRES_DB=comcode_test -p 127.0.0.1:55433:5432 postgres:18-alpine
TEST_DATABASE_URL='postgres://comcode:comcode@127.0.0.1:55433/comcode_test?sslmode=disable' go test ./...
docker stop comcode-test-pg
```

When `backend/docs/openapi.yaml` or a route changed:

```bash
make openapi-lint     # redocly lint + scripts/openapi_routes.py (spec matches live routes)
```

## Frontend (any `frontend/**` change, or an OpenAPI change)

```bash
cd frontend
pnpm install --frozen-lockfile
pnpm api:generate     # after OpenAPI changes; commit src/generated/api.d.ts
pnpm typecheck
pnpm lint
pnpm test
pnpm format:check     # Prettier (not a CI gate yet, keep it clean)
cd .. && node scripts/check-i18n.mjs   # tr/en keys match
```

`pnpm build` is slow; run it before a PR that touches routing, config or dependencies. Stale `.next/types` after deleting a route: `rm -rf .next/types` and typecheck again.

## Database

New migration → round-trip it (see the `db-migration` skill).

## Env and compose

```bash
make env-check        # .env.server.example matches deploy/env/schema, every env read reaches compose
```

Needed whenever `deploy/env/**`, `.env*.example`, `compose.prod.yml` or `backend/internal/config` changed.

## CI workflow changes

```bash
actionlint .github/workflows/ci.yml   # go install github.com/rhysd/actionlint/cmd/actionlint@latest
jq -e '.images | length > 0' deploy/images.json
```

Jobs skip by path (the `changes` job). `ci` passes when every job succeeds or is skipped.

## Shipping

Branch `cc/<issue-id>-<short-name>` from `cc-collective-branch`, open the PR into `cc-collective-branch` (never `main`). The owner merges `cc-collective-branch` into `main`; a green `main` push publishes the GHCR images and cuts `v0.N`, and Dokploy deploys that tag. Flow: `docs/DEVELOPMENT.md` → CI.
