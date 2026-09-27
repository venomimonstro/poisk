#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"
export MIGRATIONS_DIR="${MIGRATIONS_DIR:-$ROOT_DIR/db/migrations}"

: "${RELEASE_VERSION:?RELEASE_VERSION is required}"
: "${READINESS_GIT_SHA:?READINESS_GIT_SHA is required}"
: "${POSTGRES_PASSWORD:?candidate POSTGRES_PASSWORD is required for candidate precheck and evidence binding}"
: "${READINESS_UPGRADE_POSTGRES_HOST:?READINESS_UPGRADE_POSTGRES_HOST is required}"
: "${READINESS_UPGRADE_POSTGRES_DB:?READINESS_UPGRADE_POSTGRES_DB is required}"
: "${READINESS_UPGRADE_POSTGRES_USER:?READINESS_UPGRADE_POSTGRES_USER is required}"
: "${READINESS_UPGRADE_POSTGRES_PASSWORD:?READINESS_UPGRADE_POSTGRES_PASSWORD is required}"
: "${READINESS_UPGRADE_FROM_SCHEMA:?READINESS_UPGRADE_FROM_SCHEMA is required}"

if [[ ! "$READINESS_GIT_SHA" =~ ^[0-9a-f]{40}$ ]]; then
  echo "READINESS_GIT_SHA must be a lowercase 40-character Git SHA" >&2
  exit 2
fi
if [[ "$(git rev-parse HEAD)" != "$READINESS_GIT_SHA" ]]; then
  echo "checked-out HEAD does not match READINESS_GIT_SHA" >&2
  exit 2
fi
if [[ -n "$(git status --porcelain --untracked-files=no)" ]]; then
  echo "tracked working tree changes are not allowed for release evidence" >&2
  exit 2
fi
if ! command -v curl >/dev/null 2>&1; then
  echo "curl is required for upgrade boot smoke" >&2
  exit 2
fi

ARTIFACT_ROOT="${READINESS_ARTIFACT_DIR:-${TMPDIR:-/tmp}/poisk-readiness/${RELEASE_VERSION}-${READINESS_GIT_SHA}}"
mkdir -p "$ARTIFACT_ROOT"
APP_BIN="$ARTIFACT_ROOT/poisk-app"
LOG="$ARTIFACT_ROOT/upgrade-from-${READINESS_UPGRADE_FROM_SCHEMA}.log"
PORT="${READINESS_UPGRADE_APP_PORT:-18081}"
if [[ ! "$PORT" =~ ^[0-9]+$ ]] || (( PORT < 1024 || PORT > 65535 )); then
  echo "READINESS_UPGRADE_APP_PORT must be between 1024 and 65535" >&2
  exit 2
fi

: > "$LOG"
if [[ ! -x "$APP_BIN" ]]; then
  go build -trimpath -o "$APP_BIN" ./cmd/app >>"$LOG" 2>&1
fi

# Candidate binding is checked against canonical POSTGRES_* variables.
"$APP_BIN" readinessctl candidate >>"$LOG" 2>&1
# This mutates only the explicitly configured disposable upgrade copy.
"$APP_BIN" readinessctl upgrade-db >>"$LOG" 2>&1

CHILD_PID=""
cleanup() {
  if [[ -n "$CHILD_PID" ]] && kill -0 "$CHILD_PID" 2>/dev/null; then
    kill -TERM "$CHILD_PID" 2>/dev/null || true
    wait "$CHILD_PID" 2>/dev/null || true
  fi
}
trap cleanup EXIT INT TERM

APP_ENV=development \
APP_ADDR="127.0.0.1:$PORT" \
PUBLIC_BASE_URL="" \
MAIL_INTERNET_ENABLED=false \
POSTGRES_HOST="$READINESS_UPGRADE_POSTGRES_HOST" \
POSTGRES_PORT="${READINESS_UPGRADE_POSTGRES_PORT:-5432}" \
POSTGRES_DB="$READINESS_UPGRADE_POSTGRES_DB" \
POSTGRES_USER="$READINESS_UPGRADE_POSTGRES_USER" \
POSTGRES_PASSWORD="$READINESS_UPGRADE_POSTGRES_PASSWORD" \
POSTGRES_SSLMODE="${READINESS_UPGRADE_POSTGRES_SSLMODE:-disable}" \
MIGRATIONS_DIR="$MIGRATIONS_DIR" \
"$APP_BIN" api >>"$LOG" 2>&1 &
CHILD_PID=$!

BASE_URL="http://127.0.0.1:$PORT"
READY=0
for _ in $(seq 1 40); do
  if ! kill -0 "$CHILD_PID" 2>/dev/null; then
    echo "upgraded API exited before health check" >>"$LOG"
    exit 1
  fi
  if curl -fsS --max-time 2 "$BASE_URL/health/live" >>"$LOG" 2>&1 && \
     curl -fsS --max-time 2 "$BASE_URL/health/ready" >>"$LOG" 2>&1; then
    READY=1
    break
  fi
  sleep 1
done
if [[ "$READY" != "1" ]]; then
  echo "upgraded API did not become ready" >>"$LOG"
  exit 1
fi

echo "upgrade_health=PASS checked_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)" >>"$LOG"
cleanup
CHILD_PID=""
trap - EXIT INT TERM

if [[ "${READINESS_RECORD:-0}" == "1" ]]; then
  "$APP_BIN" readinessctl candidate
  "$APP_BIN" readinessctl record-file UPGRADE PASS "$LOG" "${READINESS_ACTOR:-upgrade-gate}" "{\"runner\":\"scripts/upgrade_gate.sh\",\"from_schema\":${READINESS_UPGRADE_FROM_SCHEMA},\"retained_data\":true,\"boot_health\":true}"
fi

echo "UPGRADE PASS artifact=$LOG"
echo "Disposable upgraded database was intentionally left intact for inspection: $READINESS_UPGRADE_POSTGRES_DB"
