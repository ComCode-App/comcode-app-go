#!/bin/bash
# PreToolUse (Edit|Write): block edits to files that must not be hand-edited.
# Exit 2 blocks the tool call; stderr is shown to the agent.
file=$(jq -r '.tool_input.file_path // empty')
[ -z "$file" ] && exit 0
root="${CLAUDE_PROJECT_DIR:-$(pwd)}"
rel="${file#"$root"/}"
base=$(basename "$rel")

case "$base" in
  *.example) ;;
  .env|.env.*)
    echo "Blocked: $rel holds real secrets. Edit the matching .env*.example (or deploy/env/schema for production) instead and tell the user which keys to set." >&2
    exit 2 ;;
  go.sum|pnpm-lock.yaml|package-lock.json|yarn.lock)
    echo "Blocked: $rel is generated. Use pnpm add/remove or go get / go mod tidy." >&2
    exit 2 ;;
esac

case "$rel" in
  backend/internal/database/db/*)
    echo "Blocked: $rel is sqlc output. Edit backend/internal/database/queries/*.sql or a migration, then run: cd backend && sqlc generate" >&2
    exit 2 ;;
  frontend/src/generated/api.d.ts)
    echo "Blocked: $rel is generated. Edit backend/docs/openapi.yaml, then run: cd frontend && pnpm api:generate" >&2
    exit 2 ;;
esac
exit 0
