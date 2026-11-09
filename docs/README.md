# Documentation

Human and agent entry points for this boilerplate. Prefer these files over leftover product notes — there are none left on purpose.

| Doc | Audience |
|-----|----------|
| [ARCHITECTURE.md](./ARCHITECTURE.md) | Anyone new to the repo |
| [DEVELOPMENT.md](./DEVELOPMENT.md) | Local setup and production |
| [EXTENDING.md](./EXTENDING.md) | Adding a feature on top of the starter |
| [OWNER_CHECKLIST.md](./OWNER_CHECKLIST.md) | Owner's one-time GitHub, GHCR and Dokploy settings (Turkish checklist) |
| [design/env-core.md](./design/env-core.md) | Production env schema, `prod-env.sh`, env CI checks, boot guard |
| [BOILERPLATE_SYNC.md](./BOILERPLATE_SYNC.md) | Boilerplate sync record and method for the next sync |
| [../AGENTS.md](../AGENTS.md) | Agent skill index |
| [../CONTRIBUTING.md](../CONTRIBUTING.md) | Day-to-day contribution rules |
| [../backend/README.md](../backend/README.md) | Go API |
| [../backend/docs/README.md](../backend/docs/README.md) | Auth, errors, OpenAPI |
| [../frontend/README.md](../frontend/README.md) | Next.js BFF |

Canonical config is the **repo-root** `.env` (not `backend/.env` or `frontend/.env.local`). Production env is generated from `deploy/env/schema` (`make prod-env-init`).

Agent playbooks: [`.agents/skills/`](../.agents/skills).
