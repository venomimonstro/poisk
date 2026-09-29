#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"
export MIGRATIONS_DIR="${MIGRATIONS_DIR:-$ROOT_DIR/db/migrations}"

: "${RELEASE_VERSION:?RELEASE_VERSION is required}"
: "${READINESS_GIT_SHA:?READINESS_GIT_SHA is required}"
: "${POSTGRES_PASSWORD:?candidate POSTGRES_PASSWORD is required for candidate precheck and evidence binding}"
: "${READINESS_FRESH_POSTGRES_HOST:?READINESS_FRESH_POSTGRES_HOST is required}"
: "${READINESS_FRESH_POSTGRES_DB:?READINESS_FRESH_POSTGRES_DB is required}"
: "${READINESS_FRESH_POSTGRES_USER:?READINESS_FRESH_POSTGRES_USER is required}"
: "${READINESS_FRESH_POSTGRES_PASSWORD:?READINESS_FRESH_POSTGRES_PASSWORD is required}"

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
  echo "curl is required for fresh-install boot smoke" >&2
  exit 2
fi

ARTIFACT_ROOT="${READINESS_ARTIFACT_DIR:-${TMPDIR:-/tmp}/poisk-readiness/${RELEASE_VERSION}-${READINESS_GIT_SHA}}"
mkdir -p "$ARTIFACT_ROOT"
APP_BIN="$ARTIFACT_ROOT/poisk-app"
LOG="$ARTIFACT_ROOT/fresh-install.log"
PORT="${READINESS_FRESH_APP_PORT:-18080}"
BUILD_LDFLAGS="-X github.com/venomimonstro/poisk/internal/buildinfo.GitCommit=$READINESS_GIT_SHA -X github.com/venomimonstro/poisk/internal/buildinfo.ReleaseVersion=$RELEASE_VERSION"
if [[ ! "$PORT" =~ ^[0-9]+$ ]] || (( PORT < 1024 || PORT > 65535 )); then
  echo "READINESS_FRESH_APP_PORT must be between 1024 and 65535" >&2
  exit 2
fi

: > "$LOG"
rm -f "$APP_BIN"
go build -trimpath -ldflags "$BUILD_LDFLAGS" -o "$APP_BIN" ./cmd/app >>"$LOG" 2>&1

# Candidate DB remains selected through canonical POSTGRES_* variables here.
"$APP_BIN" readinessctl candidate >>"$LOG" 2>&1
"$APP_BIN" readinessctl fresh-install-db >>"$LOG" 2>&1

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
POSTGRES_HOST="$READINESS_FRESH_POSTGRES_HOST" \
POSTGRES_PORT="${READINESS_FRESH_POSTGRES_PORT:-5432}" \
POSTGRES_DB="$READINESS_FRESH_POSTGRES_DB" \
POSTGRES_USER="$READINESS_FRESH_POSTGRES_USER" \
POSTGRES_PASSWORD="$READINESS_FRESH_POSTGRES_PASSWORD" \
POSTGRES_SSLMODE="${READINESS_FRESH_POSTGRES_SSLMODE:-disable}" \
MIGRATIONS_DIR="$MIGRATIONS_DIR" \
"$APP_BIN" api >>"$LOG" 2>&1 &
CHILD_PID=$!

BASE_URL="http://127.0.0.1:$PORT"
READY=0
for _ in $(seq 1 40); do
  if ! kill -0 "$CHILD_PID" 2>/dev/null; then
    echo "fresh-install API exited before health check" >>"$LOG"
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
  echo "fresh-install API did not become ready" >>"$LOG"
  exit 1
fi

echo "fresh_install_health=PASS checked_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)" >>"$LOG"
cleanup
CHILD_PID=""
trap - EXIT INT TERM

if [[ "${READINESS_RECORD:-0}" == "1" ]]; then
  "$APP_BIN" readinessctl candidate
  "$APP_BIN" readinessctl record-file FRESH_INSTALL PASS "$LOG" "${READINESS_ACTOR:-fresh-install-gate}" '{"runner":"scripts/fresh_install_gate.sh","boot_health":true}'
fi

echo "FRESH_INSTALL PASS artifact=$LOG"
echo "Disposable database was intentionally left intact for inspection: $READINESS_FRESH_POSTGRES_DB"
