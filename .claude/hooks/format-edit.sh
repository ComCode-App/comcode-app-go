#!/bin/bash
# PostToolUse (Edit|Write): format the edited file with the repo's formatter.
# Best effort: a missing tool or a syntax error never blocks the session.
file=$(jq -r '.tool_input.file_path // empty')
{ [ -z "$file" ] || [ ! -f "$file" ]; } && exit 0
root="${CLAUDE_PROJECT_DIR:-$(pwd)}"
rel="${file#"$root"/}"

case "$rel" in
  backend/internal/database/db/*) exit 0 ;;
  backend/*.go)
    command -v gofmt >/dev/null && gofmt -w "$file" 2>/dev/null ;;
  frontend/*.ts|frontend/*.tsx|frontend/*.js|frontend/*.mjs|frontend/*.json|frontend/*.css|frontend/*.md)
    case "$rel" in frontend/src/generated/*) exit 0 ;; esac
    prettier="$root/frontend/node_modules/.bin/prettier"
    # Run from frontend/ so .prettierrc and .prettierignore apply.
    [ -x "$prettier" ] && (cd "$root/frontend" && "$prettier" --write --log-level silent "$file" 2>/dev/null) ;;
esac
exit 0
