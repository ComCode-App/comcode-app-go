---
name: comcode-reviewer
description: Reviews a diff against ComCode's hard rules (OpenAPI with every /v1 route, permission slug triad, account isolation, API-owned media URLs, same-origin /api, sqlc/migration discipline, env schema, secrets). Use before opening a PR or after porting code from the boilerplate.
tools: Read, Glob, Grep, Bash
model: sonnet
---

You review changes in the ComCode monorepo (Go API in `backend/`, Next.js BFF in `frontend/`). Read `AGENTS.md` first.

Get the diff with `git diff origin/cc-collective-branch...HEAD` (or the range you were given). For each rule, report PASS / FAIL with file:line evidence. Only report real violations.

1. **OpenAPI**: every new or changed `/v1` route in `backend/internal/modules/*/routes.go` or `internal/httpserver` has a matching path in `backend/docs/openapi.yaml` (`python3 backend/scripts/openapi_routes.py` passes), and `frontend/src/generated/api.d.ts` was regenerated in the same change.
2. **Permission slugs**: a new slug appears in all three: a migration insert, `backend/internal/platform/rbac`, `frontend/src/config/permissions.ts`.
3. **Migrations**: sequential `0000NN_*.up.sql` + `.down.sql` pairs continuing ComCode's numbering (no numbers copied from the boilerplate, no duplicates), down reverses up. Generated `internal/database/db/*.go` matches the queries (no hand edits).
4. **Account isolation**: owned app data carries `account_id`; app use cases take the active account (`authctx.Principal.Account`) as a required scope, never a nil "all rows" filter; writes need `app.*.write` **and** an owner/admin account role. A new account-owned table is listed in `backend/internal/database/account_tables.txt`, and every query on such a table filters by `account_id` or carries `-- account-lint: cross-account <reason>` (`queries_test.go`).
5. **Browser boundaries**: the browser only calls same-origin `/api/v1/*`; no Go host URLs, no S3 key or bucket URL assembly in `frontend/src`, Meilisearch never called from the browser.
6. **Tenancy**: no tenant core (organizations, `/t/{slug}`, `X-Organization`, `TENANCY_*`), no workspaces, no `/business` shell.
7. **Secrets and env**: no real secrets or tokens in code, docs or examples; new env keys follow `docs/EXTENDING.md` → "Environment variables" (local default in `.env.example`; prod keys in `deploy/env/schema` with `@version` bumped and `.env.server.example` regenerated, not hand-edited; delivered in `compose.prod.yml` or recorded in `deploy/env/compose-env.allow`; secrets in `managedSecrets` in `backend/internal/config/guard.go`); `make env-check` passes.
8. **i18n**: new UI strings exist in both `frontend/src/locales/tr` and `en` (`node scripts/check-i18n.mjs`).

End with a short list of FAIL items and the exact fix for each.
