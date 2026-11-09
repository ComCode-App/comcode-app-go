---
name: db-migration
description: >-
  Add or change the database schema: sequential golang-migrate files that
  continue ComCode's numbering, sqlc queries and regenerated code, account
  isolation lint, repository wiring. Use for any new table, column, index,
  seed row, permission slug or query in backend/.
---

# Database migration

Rules live in [backend/DATABASE_RULES.md](../../../backend/DATABASE_RULES.md). This is the checklist.

## 1. Create the files

```bash
cd backend && make migrate-create NAME=<snake_case_description>
```

- Sequential names only (`0000NN_x.up.sql` / `.down.sql`). Never timestamps.
- Numbering continues ComCode's own sequence: `ls backend/migrations | tail -2`, then the next free number. When porting from the boilerplate (nextjs-go-boilerplate), **never** copy its migration number; renumber. If another branch took the same number before yours merged, rebase and renumber.
- Both `up` and `down`. A lossy `down` must say so in a comment (`-- Data-loss note: …`).

## 2. Write the SQL

- `BIGSERIAL id`, public `uuid UUID NOT NULL DEFAULT gen_random_uuid()` when exposed.
- `created_at` / `updated_at timestamptz`, plus the `set_updated_at()` trigger.
- Foreign keys required; `numeric` for money; `text` + `CHECK` for enums.
- Changing a CHECK constraint: `DROP CONSTRAINT` + `ADD CONSTRAINT` with the full list.
- Seed rows: `INSERT … ON CONFLICT … DO NOTHING` so re-runs are safe.
- New permission slug: migration insert **and** `internal/platform/rbac` **and** `frontend/src/config/permissions.ts` (hard rule in AGENTS.md).
- Account-owned data (rows that belong to a personal or company account) carries `account_id` and the table goes into `internal/database/account_tables.txt` in the same change. There is no organization / tenant table; never add `organization_id`.

## 3. Queries + codegen

- Add queries to `internal/database/queries/<area>.sql` (`-- name: X :one|:many|:exec|:execrows`).
- Every query on a table in `account_tables.txt` filters that table by `account_id`, or carries `-- account-lint: cross-account <reason>` directly above `-- name:` (DATABASE_RULES.md → Account isolation lint). `go test ./internal/database/` enforces it without a database.
- `sqlc generate` (or `make sqlc`): never edit `internal/database/db/*.go` by hand (the edit hook blocks it). CI regenerates and fails on a diff.
- Merge conflicts in generated files: take either side, then `sqlc generate` again.

## 4. Verify

```bash
cd backend && make migrate-up && go build ./... && go test ./...
```

Integration tests need `TEST_DATABASE_URL`. Round-trip on a throwaway database before shipping:

```bash
docker run -d --rm --name mig-test -e POSTGRES_PASSWORD=t -p 127.0.0.1:55439:5432 postgres:18-alpine
migrate -path migrations -database "postgres://postgres:t@127.0.0.1:55439/postgres?sslmode=disable" up
migrate -path migrations -database "postgres://postgres:t@127.0.0.1:55439/postgres?sslmode=disable" down 1
migrate -path migrations -database "postgres://postgres:t@127.0.0.1:55439/postgres?sslmode=disable" up
docker stop mig-test
```

## 5. API surface

New or changed `/v1` route → `backend/docs/openapi.yaml` in the same change, `python3 backend/scripts/openapi_routes.py` passes, then `cd frontend && pnpm api:generate` and commit `src/generated/api.d.ts`.
