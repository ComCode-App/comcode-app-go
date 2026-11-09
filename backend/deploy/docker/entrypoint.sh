#!/bin/sh
set -eu

if [ "$#" -gt 0 ] && [ "${1#-}" = "$1" ] && { [ -x "$1" ] || command -v "$1" >/dev/null 2>&1; }; then
  exec "$@"
fi

ROLE="${APP_ROLE:-server}"

if [ "${ROLE}" = "server" ] && [ "${AUTO_MIGRATE:-true}" = "true" ]; then
  : "${DB_HOST:?DB_HOST is required for migrate}"
  : "${DB_PORT:?DB_PORT is required for migrate}"
  : "${DB_USER:?DB_USER is required for migrate}"
  : "${DB_PASSWORD:?DB_PASSWORD is required for migrate}"
  : "${DB_NAME:?DB_NAME is required for migrate}"
  DB_SSLMODE="${DB_SSLMODE:-disable}"

  DB_URL="postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=${DB_SSLMODE}"

  echo "migrate: applying migrations..."
  migrate -path /app/migrations -database "${DB_URL}" up
  echo "migrate: done"
fi

case "${ROLE}" in
  worker)
    exec /app/worker "$@"
    ;;
  migrate)
    : "${DATABASE_URL:?DATABASE_URL is required}"
    exec migrate -path /app/migrations -database "${DATABASE_URL}" up "$@"
    ;;
  seed)
    exec /app/seed "$@"
    ;;
  create-super-admin)
    exec /app/create-super-admin "$@"
    ;;
  search-reindex)
    exec /app/search-reindex "$@"
    ;;
  *)
    exec /app/server "$@"
    ;;
esac
