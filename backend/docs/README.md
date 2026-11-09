# Backend docs

Entry points for the starter API. Interactive contract: `/docs/` (Scalar) when the server is running.

## Auth & HTTP

| Doc | Description |
|-----|-------------|
| [`auth.md`](auth.md) | Roles, permissions, JWT, platform users/roles |
| [`api/errors.md`](api/errors.md) | Error code catalog |
| [`../API_CONVENTIONS.md`](../API_CONVENTIONS.md) | Envelope, list, meta |
| [`openapi.yaml`](openapi.yaml) | OpenAPI 3.1 contract |

## Data & toolchain

| Doc | Description |
|-----|-------------|
| [`../DATABASE_RULES.md`](../DATABASE_RULES.md) | Database / migration / sqlc |
| [`../TOOLCHAIN.md`](../TOOLCHAIN.md) | Local toolchain |
| [`../README.md`](../README.md) | Commands and curl examples |

Live routes live in `internal/modules/*/routes.go` (auth, notifications, exports, imports, bulk, settings, activity, logs, storage, access, search, integrations/github) plus `/v1/realtime/*`, `/v1/app/config`, `/v1/search`, and `/v1/auth/step-up*`. `scripts/openapi_routes.py` (run by `make openapi-lint` and CI) fails when a registered route is missing from `openapi.yaml` or the spec lists an operation with no route; `--write` adds minimal operations (tag, auth, permission, path params) to refine by hand.
