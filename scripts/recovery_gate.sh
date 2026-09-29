#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"
export MIGRATIONS_DIR="${MIGRATIONS_DIR:-$ROOT_DIR/db/migrations}"

: "${RELEASE_VERSION:?RELEASE_VERSION is required}"
: "${READINESS_GIT_SHA:?READINESS_GIT_SHA is required}"
: "${POSTGRES_PASSWORD:?candidate POSTGRES_PASSWORD is required}"
: "${READINESS_BACKUP_DIR:?READINESS_BACKUP_DIR is required}"
: "${READINESS_RESTORE_POSTGRES_HOST:?READINESS_RESTORE_POSTGRES_HOST is required}"
: "${READINESS_RESTORE_POSTGRES_DB:?READINESS_RESTORE_POSTGRES_DB is required}"
: "${READINESS_RESTORE_POSTGRES_USER:?READINESS_RESTORE_POSTGRES_USER is required}"
: "${READINESS_RESTORE_POSTGRES_PASSWORD:?READINESS_RESTORE_POSTGRES_PASSWORD is required}"
: "${READINESS_RESTORE_CONFIRM:?READINESS_RESTORE_CONFIRM is required}"
: "${READINESS_RESTORE_MAIL_BLOB_DIR:?READINESS_RESTORE_MAIL_BLOB_DIR is required}"
: "${READINESS_RESTORE_BLOB_CONFIRM:?READINESS_RESTORE_BLOB_CONFIRM is required}"

for cmd in psql pg_restore tar curl cmp sort find awk sed; do command -v "$cmd" >/dev/null 2>&1 || { echo "$cmd is required" >&2; exit 2; }; done
if [[ ! "$READINESS_GIT_SHA" =~ ^[0-9a-f]{40}$ ]] || [[ "$(git rev-parse HEAD)" != "$READINESS_GIT_SHA" ]]; then echo "release commit mismatch" >&2; exit 2; fi
if [[ -n "$(git status --porcelain --untracked-files=no)" ]]; then echo "tracked working tree changes are not allowed" >&2; exit 2; fi

candidate_db="${POSTGRES_DB:-poisk}"
restore_db="$READINESS_RESTORE_POSTGRES_DB"
lower_restore="${restore_db,,}"
if [[ "$restore_db" == "$candidate_db" ]]; then echo "restore DB must differ from candidate DB" >&2; exit 2; fi
if [[ "$lower_restore" != *readiness* && "$lower_restore" != *test* ]]; then echo "restore DB name must contain readiness or test" >&2; exit 2; fi
if [[ "$READINESS_RESTORE_CONFIRM" != "RESTORE:$restore_db" ]]; then echo "READINESS_RESTORE_CONFIRM must equal RESTORE:$restore_db" >&2; exit 2; fi
blob_dir="$READINESS_RESTORE_MAIL_BLOB_DIR"
if [[ "$READINESS_RESTORE_BLOB_CONFIRM" != "RESTORE:$blob_dir" ]]; then echo "READINESS_RESTORE_BLOB_CONFIRM must equal RESTORE:<blob-dir>" >&2; exit 2; fi
if [[ ! -d "$blob_dir" || -L "$blob_dir" ]]; then echo "restore blob directory must be an existing real directory" >&2; exit 2; fi
case "${blob_dir,,}" in *readiness*|*test*) ;; *) echo "restore blob directory path must contain readiness or test" >&2; exit 2;; esac
if find "$blob_dir" -mindepth 1 -maxdepth 1 -print -quit | grep -q .; then echo "restore blob directory must be empty" >&2; exit 2; fi

ARTIFACT_ROOT="${READINESS_ARTIFACT_DIR:-${TMPDIR:-/tmp}/poisk-readiness/${RELEASE_VERSION}-${READINESS_GIT_SHA}}"
mkdir -p "$ARTIFACT_ROOT"
APP_BIN="$ARTIFACT_ROOT/poisk-app"
LOG="$ARTIFACT_ROOT/recovery.log"
PORT="${READINESS_RESTORE_APP_PORT:-18082}"
BUILD_LDFLAGS="-X github.com/venomimonstro/poisk/internal/buildinfo.GitCommit=$READINESS_GIT_SHA -X github.com/venomimonstro/poisk/internal/buildinfo.ReleaseVersion=$RELEASE_VERSION"
[[ "$PORT" =~ ^[0-9]+$ ]] && (( PORT >= 1024 && PORT <= 65535 )) || { echo "invalid READINESS_RESTORE_APP_PORT" >&2; exit 2; }
: > "$LOG"
rm -f "$APP_BIN"
go build -trimpath -ldflags "$BUILD_LDFLAGS" -o "$APP_BIN" ./cmd/app >>"$LOG" 2>&1

"$APP_BIN" readinessctl candidate >>"$LOG" 2>&1
"$ROOT_DIR/deploy/backup/verify.sh" "$READINESS_BACKUP_DIR" >>"$LOG" 2>&1
"$APP_BIN" recoveryctl inspect-artifact "$READINESS_BACKUP_DIR" >>"$LOG" 2>&1
backup_duration="$(sed -n 's/^duration_ms=//p' "$READINESS_BACKUP_DIR/MANIFEST")"
[[ "$backup_duration" =~ ^[1-9][0-9]*$ ]] || { echo "backup duration missing" >&2; exit 2; }

export PGPASSWORD="$READINESS_RESTORE_POSTGRES_PASSWORD"
export PGSSLMODE="${READINESS_RESTORE_POSTGRES_SSLMODE:-disable}"
PG=(--host="$READINESS_RESTORE_POSTGRES_HOST" --port="${READINESS_RESTORE_POSTGRES_PORT:-5432}" --username="$READINESS_RESTORE_POSTGRES_USER" --dbname="$restore_db")
public_count="$(psql "${PG[@]}" -v ON_ERROR_STOP=1 -Atc "SELECT count(*) FROM pg_tables WHERE schemaname='public' AND tablename<>'spatial_ref_sys'")"
[[ "$public_count" == "0" ]] || { echo "restore target DB must be empty" >&2; exit 3; }

start_epoch="$(date +%s)"
pg_restore "${PG[@]}" --no-owner --no-privileges "$READINESS_BACKUP_DIR/postgres.dump" >>"$LOG" 2>&1
tar -xzf "$READINESS_BACKUP_DIR/mail-blobs.tar.gz" -C "$blob_dir"

expected_versions="$ARTIFACT_ROOT/recovery-expected-migrations.list"
applied_versions="$ARTIFACT_ROOT/recovery-applied-migrations.list"
find "$MIGRATIONS_DIR" -maxdepth 1 -type f -name '*.sql' -printf '%f\n' | sed -n 's/^0*\([0-9][0-9]*\)_.*/\1/p' | sort -n > "$expected_versions"
psql "${PG[@]}" -v ON_ERROR_STOP=1 -Atc "SELECT version FROM schema_migrations ORDER BY version" > "$applied_versions"
cmp -s "$expected_versions" "$applied_versions" || { echo "restored migration set mismatch" >&2; diff -u "$expected_versions" "$applied_versions" >>"$LOG" || true; exit 4; }

manifest_schema="$(sed -n 's/^database_schema=//p' "$READINESS_BACKUP_DIR/MANIFEST")"
restored_schema="$(psql "${PG[@]}" -v ON_ERROR_STOP=1 -Atc "SELECT COALESCE(max(version),0) FROM schema_migrations")"
[[ "$restored_schema" == "$manifest_schema" ]] || { echo "restored schema mismatch" >&2; exit 4; }

# Validate exact migration content/checksum metadata and release manifest in the restored DB.
POSTGRES_HOST="$READINESS_RESTORE_POSTGRES_HOST" \
POSTGRES_PORT="${READINESS_RESTORE_POSTGRES_PORT:-5432}" \
POSTGRES_DB="$restore_db" \
POSTGRES_USER="$READINESS_RESTORE_POSTGRES_USER" \
POSTGRES_PASSWORD="$READINESS_RESTORE_POSTGRES_PASSWORD" \
POSTGRES_SSLMODE="${READINESS_RESTORE_POSTGRES_SSLMODE:-disable}" \
MIGRATIONS_DIR="$MIGRATIONS_DIR" \
"$APP_BIN" readinessctl candidate >>"$LOG" 2>&1

restored_blobs="$ARTIFACT_ROOT/recovery-restored-db-blobs.list"
psql "${PG[@]}" -v ON_ERROR_STOP=1 -Atc "SELECT storage_key::text || '.blob' FROM mail_attachment_blobs ORDER BY storage_key" > "$restored_blobs"
cmp -s "$READINESS_BACKUP_DIR/mail-db-blobs.list" "$restored_blobs" || { echo "restored DB blob identity mismatch" >&2; exit 4; }
while IFS= read -r name; do [[ -z "$name" || -f "$blob_dir/$name" ]] || { echo "restored blob missing: $name" >&2; exit 4; }; done < "$restored_blobs"

CHILD_PID=""
cleanup(){ if [[ -n "$CHILD_PID" ]] && kill -0 "$CHILD_PID" 2>/dev/null; then kill -TERM "$CHILD_PID" 2>/dev/null || true; wait "$CHILD_PID" 2>/dev/null || true; fi; }
trap cleanup EXIT INT TERM
APP_ENV=development APP_ADDR="127.0.0.1:$PORT" PUBLIC_BASE_URL="" MAIL_INTERNET_ENABLED=false \
POSTGRES_HOST="$READINESS_RESTORE_POSTGRES_HOST" POSTGRES_PORT="${READINESS_RESTORE_POSTGRES_PORT:-5432}" POSTGRES_DB="$restore_db" POSTGRES_USER="$READINESS_RESTORE_POSTGRES_USER" POSTGRES_PASSWORD="$READINESS_RESTORE_POSTGRES_PASSWORD" POSTGRES_SSLMODE="${READINESS_RESTORE_POSTGRES_SSLMODE:-disable}" \
MAIL_BLOB_DIR="$blob_dir" MIGRATIONS_DIR="$MIGRATIONS_DIR" "$APP_BIN" api >>"$LOG" 2>&1 &
CHILD_PID=$!
ready=0
for _ in $(seq 1 40); do
  kill -0 "$CHILD_PID" 2>/dev/null || { echo "restored API exited" >&2; exit 5; }
  if curl -fsS --max-time 2 "http://127.0.0.1:$PORT/health/live" >>"$LOG" 2>&1 && curl -fsS --max-time 2 "http://127.0.0.1:$PORT/health/ready" >>"$LOG" 2>&1; then ready=1; break; fi
  sleep 1
done
[[ "$ready" == "1" ]] || { echo "restored API did not become ready" >&2; exit 5; }
cleanup; CHILD_PID=""; trap - EXIT INT TERM
end_epoch="$(date +%s)"; restore_duration="$(( (end_epoch-start_epoch)*1000 ))"; (( restore_duration > 0 )) || restore_duration=1
echo "recovery_restore=PASS schema=$restored_schema duration_ms=$restore_duration" >>"$LOG"

# Restore candidate DB credentials for evidence writes; PGPASSWORD is irrelevant to the Go app.
unset PGPASSWORD PGSSLMODE
if [[ "${READINESS_RECORD:-0}" == "1" ]]; then
  "$APP_BIN" recoveryctl record-artifact BACKUP "$READINESS_BACKUP_DIR" "$backup_duration"
  "$APP_BIN" recoveryctl record-artifact RESTORE "$READINESS_BACKUP_DIR" "$restore_duration"
fi

echo "RECOVERY PASS artifact=$READINESS_BACKUP_DIR log=$LOG"
echo "Disposable restore DB and blob directory were left intact for inspection: $restore_db $blob_dir"
