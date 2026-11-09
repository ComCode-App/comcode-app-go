# Contributing

ComCode is a product built from the nextjs-go-boilerplate starter. Keep the conventions strict so modules stay easy to extend; [AGENTS.md](./AGENTS.md) holds the hard rules.

## Working rules

- Prefer the nearest app boundary (`backend/` or `frontend/`) before widening scope.
- Branch `cc/<issue-id>-<short-name>` from `cc-collective-branch` and open the PR into `cc-collective-branch`, never into `main` ([docs/DEVELOPMENT.md](./docs/DEVELOPMENT.md) → CI → Branch flow).
- Canonical env is the repo-root `.env`. Production env comes from `deploy/env/schema` (`make prod-env-init`).
- Do not treat `.next/` or generated output as source. Commit `backend/internal/database/db` after `sqlc generate` and `frontend/src/generated/api.d.ts` after `pnpm api:generate`; never edit either by hand.
- New or changed `/v1` routes update `backend/docs/openapi.yaml` in the same change.
- Owned app data carries `account_id` and is scoped to the active account (personal or company); see [docs/TENANCY_AND_ACCESS.md](./docs/TENANCY_AND_ACCESS.md).
- New migrations continue ComCode's numbering (`ls backend/migrations | tail -2`).
- Media URLs are API-owned. Never assemble S3 (SeaweedFS) keys in the browser.
- If you change a convention (envelope, RBAC, shells, env), update the matching file under `docs/` or `backend/docs/` in the same change.
- Agent checklists live under `.agents/skills/` (index: [AGENTS.md](./AGENTS.md)); Claude Code setup: [CLAUDE.md](./CLAUDE.md).

## Before a PR

Run the checks CI runs: [.agents/skills/verify-changes](./.agents/skills/verify-changes/SKILL.md).

## Backend

Start from [backend/API_CONVENTIONS.md](./backend/API_CONVENTIONS.md) and [backend/DATABASE_RULES.md](./backend/DATABASE_RULES.md).

```sh
gofmt -l backend          # must print nothing
make -C backend test
make -C backend lint
make -C backend openapi-lint
```

## Frontend

Node 22.13+ (`frontend/.nvmrc`) and pnpm 10.15.1.

```sh
cd frontend
pnpm typecheck
pnpm lint
pnpm test
pnpm format:check
```

From repo root: `make check-i18n` (missing translation keys). Date fields use the shared pickers (`.agents/skills/admin-date-picker`). Do not add native `<input type="date" />`.

## Docs

Product docs live under `docs/`; boilerplate sync decisions are recorded in [docs/BOILERPLATE_SYNC.md](./docs/BOILERPLATE_SYNC.md).
