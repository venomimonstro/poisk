#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"
export MIGRATIONS_DIR="${MIGRATIONS_DIR:-$ROOT_DIR/db/migrations}"

: "${PUBLIC_BASE_URL:?PUBLIC_BASE_URL is required}"
: "${RELEASE_VERSION:?RELEASE_VERSION is required}"
: "${READINESS_GIT_SHA:?READINESS_GIT_SHA is required}"

for cmd in curl python3 git; do command -v "$cmd" >/dev/null 2>&1 || { echo "$cmd is required" >&2; exit 2; }; done
if [[ ! "$READINESS_GIT_SHA" =~ ^[0-9a-f]{40}$ ]]; then echo "READINESS_GIT_SHA must be lowercase 40-character SHA" >&2; exit 2; fi
if [[ "$(git rev-parse HEAD)" != "$READINESS_GIT_SHA" ]]; then echo "checked-out HEAD does not match candidate" >&2; exit 2; fi
if [[ -n "$(git status --porcelain --untracked-files=no)" ]]; then echo "tracked working tree changes are not allowed" >&2; exit 2; fi

BASE_URL="${PUBLIC_BASE_URL%/}"
python3 - "$BASE_URL" <<'PY'
import sys, urllib.parse
u=urllib.parse.urlparse(sys.argv[1])
if u.scheme!='https' or not u.hostname or u.username or u.password or u.query or u.fragment or (u.path not in ('','/')):
    raise SystemExit('PUBLIC_BASE_URL must be an HTTPS origin without credentials/query/fragment/path')
PY

ARTIFACT_ROOT="${READINESS_ARTIFACT_DIR:-${TMPDIR:-/tmp}/poisk-readiness/${RELEASE_VERSION}-${READINESS_GIT_SHA}}"
mkdir -p "$ARTIFACT_ROOT"
APP_BIN="$ARTIFACT_ROOT/poisk-app"
LOG="$ARTIFACT_ROOT/edge-tls-proxy.log"
HEALTH_HEADERS="$ARTIFACT_ROOT/edge-health.headers"
HEALTH_BODY="$ARTIFACT_ROOT/edge-health.json"
FRONT_HEADERS="$ARTIFACT_ROOT/edge-frontend.headers"
FRONT_BODY="$ARTIFACT_ROOT/edge-frontend.json"
HTTP_HEADERS="$ARTIFACT_ROOT/edge-http-redirect.headers"
BUILD_LDFLAGS="-X github.com/venomimonstro/poisk/internal/buildinfo.GitCommit=$READINESS_GIT_SHA -X github.com/venomimonstro/poisk/internal/buildinfo.ReleaseVersion=$RELEASE_VERSION"

: > "$LOG"
rm -f "$APP_BIN"
go build -trimpath -ldflags "$BUILD_LDFLAGS" -o "$APP_BIN" ./cmd/app >>"$LOG" 2>&1

# Candidate DB/release manifest must itself be valid before edge evidence can be recorded.
"$APP_BIN" readinessctl candidate >>"$LOG" 2>&1

curl --proto '=https' --tlsv1.2 --max-redirs 0 --fail --show-error --silent \
  -D "$HEALTH_HEADERS" -o "$HEALTH_BODY" "$BASE_URL/health/ready"
curl --proto '=https' --tlsv1.2 --max-redirs 0 --fail --show-error --silent \
  -D "$FRONT_HEADERS" -o "$FRONT_BODY" "$BASE_URL/api/frontend-build"

python3 - "$HEALTH_BODY" "$FRONT_BODY" "$READINESS_GIT_SHA" "$RELEASE_VERSION" <<'PY'
import json,sys
health=json.load(open(sys.argv[1],encoding='utf-8'))
front=json.load(open(sys.argv[2],encoding='utf-8'))
sha,release=sys.argv[3],sys.argv[4]
if health.get('status')!='ready': raise SystemExit('backend is not ready')
build=health.get('build') or {}
if build.get('git_commit')!=sha or build.get('release_version')!=release:
    raise SystemExit(f'backend build identity mismatch: {build!r}')
if front.get('git_commit')!=sha or front.get('release_version')!=release:
    raise SystemExit(f'frontend build identity mismatch: {front!r}')
PY

# HTTPS edge must advertise HSTS. Header names are case-insensitive.
python3 - "$HEALTH_HEADERS" "$FRONT_HEADERS" <<'PY'
import sys
for path in sys.argv[1:]:
    text=open(path,encoding='iso-8859-1').read().lower()
    if '\nstrict-transport-security:' not in '\n'+text:
        raise SystemExit(f'HSTS missing from {path}')
    if '\nx-content-type-options: nosniff' not in '\n'+text:
        raise SystemExit(f'nosniff missing from {path}')
PY

HTTP_URL="$(python3 - "$BASE_URL" <<'PY'
import sys,urllib.parse
u=urllib.parse.urlparse(sys.argv[1])
port=f':{u.port}' if u.port and u.port not in (80,443) else ''
print(f'http://{u.hostname}{port}')
PY
)"
set +e
curl --max-redirs 0 --show-error --silent -D "$HTTP_HEADERS" -o /dev/null "$HTTP_URL/" 
rc=$?
set -e
if [[ $rc -ne 0 ]]; then echo "HTTP redirect probe failed" >&2; exit 3; fi
python3 - "$HTTP_HEADERS" "$BASE_URL" <<'PY'
import re,sys,urllib.parse
text=open(sys.argv[1],encoding='iso-8859-1').read()
lines=text.replace('\r','').split('\n')
status=lines[0].split()[1] if lines and len(lines[0].split())>1 else ''
if status not in {'301','302','307','308'}: raise SystemExit(f'expected HTTP redirect, got {status}')
loc=''
for line in lines[1:]:
    if line.lower().startswith('location:'): loc=line.split(':',1)[1].strip(); break
if not loc: raise SystemExit('HTTP redirect Location missing')
target=urllib.parse.urlparse(urllib.parse.urljoin(sys.argv[2]+'/',loc))
base=urllib.parse.urlparse(sys.argv[2])
if target.scheme!='https' or target.hostname!=base.hostname: raise SystemExit(f'unsafe redirect target: {loc}')
PY

{
  echo "edge_tls_proxy=PASS"
  echo "release_version=$RELEASE_VERSION"
  echo "git_commit=$READINESS_GIT_SHA"
  echo "public_base_url=$BASE_URL"
  echo "backend_identity=PASS"
  echo "frontend_identity=PASS"
  echo "hsts=PASS"
  echo "http_to_https_redirect=PASS"
  echo "checked_at=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
} >> "$LOG"

if [[ "${READINESS_RECORD:-0}" == "1" ]]; then
  "$APP_BIN" readinessctl record-file EDGE_TLS_PROXY PASS "$LOG" "${READINESS_ACTOR:-edge-gate}" '{"runner":"scripts/edge_gate.sh","backend_identity":true,"frontend_identity":true,"hsts":true,"http_redirect":true}'
fi

echo "EDGE_TLS_PROXY PASS artifact=$LOG"
