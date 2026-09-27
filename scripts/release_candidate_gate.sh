#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

: "${RELEASE_VERSION:?RELEASE_VERSION is required}"
: "${READINESS_GIT_SHA:?READINESS_GIT_SHA is required}"

if [[ ! "$READINESS_GIT_SHA" =~ ^[0-9a-f]{40}$ ]]; then
  echo "READINESS_GIT_SHA must be a lowercase 40-character Git SHA" >&2
  exit 2
fi

HEAD_SHA="$(git rev-parse HEAD)"
if [[ "$HEAD_SHA" != "$READINESS_GIT_SHA" ]]; then
  echo "HEAD ($HEAD_SHA) does not match READINESS_GIT_SHA ($READINESS_GIT_SHA)" >&2
  exit 2
fi

if [[ -n "$(git status --porcelain --untracked-files=no)" ]]; then
  echo "tracked working tree changes are not allowed for release evidence" >&2
  exit 2
fi

ARTIFACT_ROOT="${READINESS_ARTIFACT_DIR:-${TMPDIR:-/tmp}/poisk-readiness/${RELEASE_VERSION}-${READINESS_GIT_SHA}}"
mkdir -p "$ARTIFACT_ROOT"
APP_BIN="$ARTIFACT_ROOT/poisk-app"

sha256_file() {
  local path="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$path" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$path" | awk '{print $1}'
  else
    echo "sha256sum or shasum is required" >&2
    return 127
  fi
}

run_logged() {
  local log="$1"
  shift
  set +e
  "$@" > >(tee -a "$log") 2> >(tee -a "$log" >&2)
  local rc=$?
  set -e
  return "$rc"
}

record_evidence() {
  local kind="$1"
  local artifact="$2"
  local actor="${READINESS_ACTOR:-release-gate}"
  [[ "${READINESS_RECORD:-0}" == "1" ]] || return 0
  [[ -n "${POSTGRES_DSN:-}" ]] || {
    echo "POSTGRES_DSN is required when READINESS_RECORD=1" >&2
    return 2
  }
  "$APP_BIN" readinessctl record-file "$kind" PASS "$artifact" "$actor" '{"runner":"scripts/release_candidate_gate.sh"}'
}

BUILD_LOG="$ARTIFACT_ROOT/build-unit.log"
: > "$BUILD_LOG"
{
  echo "release_version=$RELEASE_VERSION"
  echo "git_commit=$READINESS_GIT_SHA"
  echo "started_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  go version
  node --version
  npm --version
} | tee -a "$BUILD_LOG"

run_logged "$BUILD_LOG" go vet ./...
run_logged "$BUILD_LOG" go test ./...
run_logged "$BUILD_LOG" go build -trimpath -o "$APP_BIN" ./cmd/app
(
  cd web
  run_logged "$BUILD_LOG" npm ci
  run_logged "$BUILD_LOG" npm run lint
  run_logged "$BUILD_LOG" npm run build
)
echo "completed_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)" | tee -a "$BUILD_LOG"
record_evidence BUILD_UNIT "$BUILD_LOG"

echo "BUILD_UNIT PASS artifact=$BUILD_LOG sha256=$(sha256_file "$BUILD_LOG")"

if [[ -n "${TEST_DATABASE_URL:-}" ]]; then
  INTEGRATION_LOG="$ARTIFACT_ROOT/integration.log"
  : > "$INTEGRATION_LOG"
  run_logged "$INTEGRATION_LOG" go test -tags=integration ./...
  record_evidence INTEGRATION "$INTEGRATION_LOG"
  echo "INTEGRATION PASS artifact=$INTEGRATION_LOG sha256=$(sha256_file "$INTEGRATION_LOG")"

  SECURITY_LOG="$ARTIFACT_ROOT/security-regression.log"
  : > "$SECURITY_LOG"
  run_logged "$SECURITY_LOG" go test -tags=integration \
    ./internal/admin/... \
    ./internal/identity/... \
    ./internal/reviews/... \
    ./internal/webmaster/... \
    ./internal/growth/claim/... \
    ./internal/billing/... \
    ./internal/crawler/... \
    ./internal/mail/...
  record_evidence SECURITY_REGRESSION "$SECURITY_LOG"
  echo "SECURITY_REGRESSION PASS artifact=$SECURITY_LOG sha256=$(sha256_file "$SECURITY_LOG")"
else
  echo "TEST_DATABASE_URL is not set: INTEGRATION and SECURITY_REGRESSION were not run or recorded." >&2
fi

if [[ "${READINESS_RECORD:-0}" == "1" ]]; then
  "$APP_BIN" readinessctl check || true
else
  echo "Evidence recording disabled. Set READINESS_RECORD=1 and POSTGRES_DSN to record PASS evidence."
fi

cat <<EOF
Release-candidate local gate completed.
Artifacts: $ARTIFACT_ROOT
This runner does NOT create FRESH_INSTALL, UPGRADE, BROWSER_SMOKE, EDGE_TLS_PROXY, Capacity, Recovery or MTA evidence.
Those checks remain separate because they require isolated/real deployment environments.
EOF
