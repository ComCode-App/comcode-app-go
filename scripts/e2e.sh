#!/usr/bin/env bash
# Runs the Playwright E2E smoke suite (frontend/e2e) against a throwaway stack:
# Postgres, Redis and SeaweedFS from compose.local.yml under their own project name and ports,
# the Go API built from source, and the production Next.js build. Everything is removed on exit.
#
#   scripts/e2e.sh                 # whole suite
#   scripts/e2e.sh auth.spec.ts    # extra args go to `playwright test`
#
# Needs Docker, Go, Node + pnpm. Ports are chosen so the stack can run next to `make local-dev`.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

# Defaults from .env.example; the overrides below isolate the E2E stack.
while IFS= read -r line || [ -n "$line" ]; do
  case "$line" in \#*|'') continue ;; esac
  if [[ "$line" =~ ^([A-Za-z_][A-Za-z0-9_]*)=(.*)$ ]]; then
    export "${BASH_REMATCH[1]}=${BASH_REMATCH[2]}"
  fi
done < .env.example

E2E_ID="${E2E_ID:-$$}"
export NAME_PREFIX="comcode-e2e-${E2E_ID}"
export COMPOSE_PROJECT_NAME="$NAME_PREFIX"
export LOCAL_NETWORK_NAME="$NAME_PREFIX"
export POSTGRES_VOLUME_NAME="${NAME_PREFIX}-postgres"
export REDIS_VOLUME_NAME="${NAME_PREFIX}-redis"
export SEAWEEDFS_VOLUME_NAME="${NAME_PREFIX}-seaweedfs"

export POSTGRES_PORT="${E2E_POSTGRES_PORT:-56433}"
export REDIS_PORT="${E2E_REDIS_PORT:-56380}"
export S3_API_PORT="${E2E_S3_PORT:-58334}"
API_PORT="${E2E_API_PORT:-58081}"
WEB_PORT="${E2E_WEB_PORT:-53000}"

export DB_PORT="$POSTGRES_PORT" DB_USER=e2e DB_PASSWORD=e2e DB_NAME=comcode_e2e
export REDIS_ADDR="127.0.0.1:${REDIS_PORT}"
export S3_ENDPOINT="http://127.0.0.1:${S3_API_PORT}"
export STORAGE_PUBLIC_BASE_URL="http://127.0.0.1:${S3_API_PORT}/${S3_BUCKET}"
export APP_ENV=test APP_HTTP_ADDR="127.0.0.1:${API_PORT}" LOG_LEVEL=warn
export SEARCH_ENABLED=false CENTRIFUGO_ENABLED=false NEXT_PUBLIC_REALTIME_ENABLED=false
export API_URL="http://127.0.0.1:${API_PORT}/v1"
export PUBLIC_API_URL="http://127.0.0.1:${API_PORT}"
export PUBLIC_FRONTEND_URL="http://localhost:${WEB_PORT}"
export AUTH_URL="$PUBLIC_FRONTEND_URL"
export CORS_ALLOWED_ORIGINS="$PUBLIC_FRONTEND_URL"
export E2E_BASE_URL="$PUBLIC_FRONTEND_URL"
export E2E_API_URL="$API_URL"
export E2E_ADMIN_EMAIL="e2e-admin@example.test" E2E_ADMIN_PASSWORD="E2eAdmin-Password1"

# Not under test-results/: Playwright empties that directory on start.
LOG_DIR="${E2E_LOG_DIR:-$ROOT/frontend/e2e/.state/logs}"
mkdir -p "$LOG_DIR"
COMPOSE=(docker compose -f compose.local.yml)
PIDS=()

cleanup() {
  local code=$?
  for pid in "${PIDS[@]}"; do kill "$pid" 2>/dev/null || true; done
  "${COMPOSE[@]}" down -v --remove-orphans >/dev/null 2>&1 || true
  exit "$code"
}
trap cleanup EXIT

wait_http() {
  local url="$1" name="$2"
  for _ in $(seq 1 120); do
    curl -fsS -o /dev/null "$url" 2>/dev/null && return 0
    sleep 1
  done
  echo "$name did not come up at $url" >&2
  return 1
}

echo "==> infra ($NAME_PREFIX)"
"${COMPOSE[@]}" up -d --wait postgres redis seaweedfs
"${COMPOSE[@]}" up seaweedfs-init --exit-code-from seaweedfs-init >/dev/null

echo "==> migrate"
(cd backend && go run -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.20.1 \
  -path migrations \
  -database "postgres://${DB_USER}:${DB_PASSWORD}@127.0.0.1:${DB_PORT}/${DB_NAME}?sslmode=disable" up)

echo "==> backend"
(cd backend && go build -o bin/e2e-api ./cmd/server && go build -o bin/e2e-create-super-admin ./cmd/create-super-admin)
backend/bin/e2e-create-super-admin -email "$E2E_ADMIN_EMAIL" -password "$E2E_ADMIN_PASSWORD" >/dev/null
backend/bin/e2e-api >"$LOG_DIR/api.log" 2>&1 &
PIDS+=($!)
wait_http "http://127.0.0.1:${API_PORT}/readyz" "API"

echo "==> frontend"
cd frontend
pnpm install --frozen-lockfile
if [ "${E2E_SKIP_BUILD:-0}" != "1" ]; then
  pnpm build
fi
# Same layout as the production image (frontend/Dockerfile).
cp -R public .next/standalone/
cp -R .next/static .next/standalone/.next/
PORT="$WEB_PORT" HOSTNAME=127.0.0.1 node .next/standalone/server.js >"$LOG_DIR/web.log" 2>&1 &
PIDS+=($!)
wait_http "http://127.0.0.1:${WEB_PORT}/login" "frontend"

echo "==> playwright"
if [ -z "${PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD:-}" ]; then
  # A fresh Linux CI runner (GitHub-hosted ubuntu) also needs Chromium's system libraries.
  if [ -n "${CI:-}" ] && [ "$(uname -s)" = "Linux" ]; then
    pnpm exec playwright install --with-deps chromium >/dev/null
  else
    pnpm exec playwright install chromium >/dev/null
  fi
fi
pnpm e2e "$@"
