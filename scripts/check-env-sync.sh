#!/usr/bin/env bash
# Keeps the env files in line with deploy/env/schema (docs/design/env-core.md):
#   1. .env.server.example is exactly `scripts/prod-env.sh example` output
#      (fix: scripts/prod-env.sh example > .env.server.example)
#   2. every schema key also has a local default in .env.example
#   3. every env var the Go backend reads is in the schema: a managed key, or
#      `@unmanaged KEY reason`; an @unmanaged key the backend no longer reads fails
#   4. every deploy/env/compose-env.allow key is @unmanaged in the schema
#   5. scripts/prod-env.test.sh passes (skip with --no-tests)
# Run: make env-check
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT"
run_tests=1
[[ ${1:-} == --no-tests ]] && run_tests=0
status=0
SCHEMA=deploy/env/schema

T=$(mktemp -d)
trap 'rm -rf "$T"' EXIT

# 1. generated example
scripts/prod-env.sh example >"$T/example"
if ! diff -u --label .env.server.example --label "prod-env.sh example" .env.server.example "$T/example"; then
  echo "check-env-sync: .env.server.example is out of date with $SCHEMA." >&2
  echo "  fix: scripts/prod-env.sh example > .env.server.example" >&2
  status=1
fi

sed -nE 's/^([A-Z][A-Z0-9_]*)[[:space:]]*\|.*/\1/p' "$SCHEMA" | sort -u >"$T/managed"
sed -nE 's/^@unmanaged[[:space:]]+([A-Z][A-Z0-9_]*)[[:space:]].*/\1/p' "$SCHEMA" | sort -u >"$T/unmanaged"
sort -u "$T/managed" "$T/unmanaged" >"$T/schema"

# 2. local defaults
missing=$(comm -23 "$T/managed" \
  <(sed -nE 's/^[[:space:]]*(export[[:space:]]+)?([A-Za-z_][A-Za-z0-9_]*)=.*/\2/p' .env.example | sort -u))
if [[ -n $missing ]]; then
  echo "check-env-sync: schema keys without a local default in .env.example:" >&2
  sed 's/^/  /' <<<"$missing" >&2
  status=1
fi

# 3. backend reads <-> schema. Same read set as scripts/check-compose-env.sh:
# string-literal reads in non-test Go files.
grep -rhoE --include='*.go' --exclude='*_test.go' \
  '\b(getEnv|getInt|getInt64|getBool|getDuration|getFloat|os\.Getenv|os\.LookupEnv)\("[A-Za-z0-9_]+"' backend |
  sed -E 's/.*\("//; s/"$//' | sort -u >"$T/read"
[[ -s $T/read ]] || { echo "check-env-sync: found no env reads under backend/ (pattern out of date?)" >&2; exit 2; }
unknown=$(comm -23 "$T/read" "$T/schema")
if [[ -n $unknown ]]; then
  echo "check-env-sync: the backend reads env vars $SCHEMA does not list:" >&2
  sed 's/^/  /' <<<"$unknown" >&2
  echo "  fix: add each as a managed key (usually 'KEY | empty' under Optional tuning), or '@unmanaged KEY reason'" >&2
  status=1
fi
stale=$(comm -23 "$T/unmanaged" "$T/read")
if [[ -n $stale ]]; then
  echo "check-env-sync: @unmanaged keys the backend no longer reads (drop them from $SCHEMA):" >&2
  sed 's/^/  /' <<<"$stale" >&2
  status=1
fi

# 4. compose-env allowlist keys are not operator values
if [[ -f deploy/env/compose-env.allow ]]; then
  not_unmanaged=$(comm -23 \
    <(sed -nE 's/^([A-Z][A-Z0-9_]*)[[:space:]]*\|.*/\1/p' deploy/env/compose-env.allow | sort -u) \
    "$T/unmanaged")
  if [[ -n $not_unmanaged ]]; then
    echo "check-env-sync: deploy/env/compose-env.allow keys that are not @unmanaged in $SCHEMA:" >&2
    sed 's/^/  /' <<<"$not_unmanaged" >&2
    echo "  a key the containers never receive cannot be an operator value" >&2
    status=1
  fi
fi

# 5. generator tests
if ((run_tests)); then
  scripts/prod-env.test.sh >/dev/null || { echo "check-env-sync: scripts/prod-env.test.sh failed" >&2; status=1; }
fi

((status)) || echo "check-env-sync: ok ($(wc -l <"$T/managed" | tr -d ' ') managed keys, $(wc -l <"$T/read" | tr -d ' ') backend env reads)"
exit "$status"
