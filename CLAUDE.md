@AGENTS.md

## Claude Code

Project setup checked into the repo:

- **Skills**: `.claude/skills/*` are symlinks to `.agents/skills/*`, so Claude and other agents share one copy. Add a skill under `.agents/skills/<name>/SKILL.md`, then link it: `ln -s ../../.agents/skills/<name> .claude/skills/<name>`.
- **Hooks** (`.claude/hooks/`, registered under `hooks` in `.claude/settings.json`):
  - before an edit, `guard-edit.sh` blocks real `.env*` files, lock files / `go.sum`, sqlc output (`backend/internal/database/db`) and `frontend/src/generated/api.d.ts`; regenerate those instead.
  - after an edit, `format-edit.sh` runs `gofmt` on Go files and Prettier on frontend files.
- **MCP servers** (`.mcp.json`, approve once per machine): `shadcn` (component registry, runs in `frontend/`), `context7` (current library docs), `playwright` (browser checks; output in the git-ignored `.playwright-mcp/`).
- **Subagent**: `comcode-reviewer` checks a diff against the hard rules in AGENTS.md before a PR.
- **Before saying done**: follow the `verify-changes` skill. Schema work: `db-migration` skill.
- **Tenancy**: personal and company **accounts** (`account_id`, active account scope). There is no tenant core: no organizations, `/t/{slug}`, `X-Organization` or `TENANCY_*`.
- **Local ports**: Next.js 54002, API 58080 (Postgres 55432, Redis 56379, SeaweedFS 58333). Never start a dev server on port 3000.
- **Shipping**: branch `cc/<issue-id>-<short-name>` from `cc-collective-branch` and open the PR into `cc-collective-branch` (AGENTS.md → Branches and releases). A green `main` push publishes GHCR images and tags `v0.N`, and Dokploy deploys the tag (`docs/DEVELOPMENT.md` → Production).
