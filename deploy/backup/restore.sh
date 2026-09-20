#!/bin/sh
set -eu
umask 077

: "${POSTGRES_HOST:=postgres}"
: "${POSTGRES_PORT:=5432}"
: "${POSTGRES_DB:=poisk}"
: "${POSTGRES_USER:=poisk}"
: "${POSTGRES_PASSWORD:?POSTGRES_PASSWORD is required}"
: "${RESTORE_DIR:?RESTORE_DIR is required, e.g. /backups/20260920T120000Z}"
: "${RESTORE_CONFIRM:?RESTORE_CONFIRM is required}"

if [ "$RESTORE_CONFIRM" != "RESTORE:$POSTGRES_DB" ]; then
  echo "restore refused: RESTORE_CONFIRM must equal RESTORE:$POSTGRES_DB" >&2
  exit 2
fi
case "$RESTORE_DIR" in
  /backups/*) ;;
  *) echo "restore refused: RESTORE_DIR must be below /backups" >&2; exit 2 ;;
esac
[ -f "$RESTORE_DIR/MANIFEST" ] || { echo "restore refused: MANIFEST missing" >&2; exit 2; }
[ -f "$RESTORE_DIR/SHA256SUMS" ] || { echo "restore refused: SHA256SUMS missing" >&2; exit 2; }
[ -f "$RESTORE_DIR/postgres.dump" ] || { echo "restore refused: postgres.dump missing" >&2; exit 2; }
grep -qx 'backup_format=poisk-v1' "$RESTORE_DIR/MANIFEST" || { echo "restore refused: unsupported backup format" >&2; exit 2; }

(cd "$RESTORE_DIR" && sha256sum -c SHA256SUMS)
export PGPASSWORD="$POSTGRES_PASSWORD"
pg_restore --list "$RESTORE_DIR/postgres.dump" >/dev/null

count="$(psql --host="$POSTGRES_HOST" --port="$POSTGRES_PORT" --username="$POSTGRES_USER" --dbname="$POSTGRES_DB" -v ON_ERROR_STOP=1 -Atc "SELECT count(*) FROM information_schema.tables WHERE table_schema='public'")"
restore_mode="create"
if [ "$count" != "0" ]; then
  if [ "${RESTORE_ALLOW_NONEMPTY:-no}" != "yes" ]; then
    echo "restore refused: target database is not empty; set RESTORE_ALLOW_NONEMPTY=yes after verification" >&2
    exit 3
  fi
  restore_mode="replace"
fi

if [ "$restore_mode" = "replace" ]; then
  pg_restore --host="$POSTGRES_HOST" --port="$POSTGRES_PORT" --username="$POSTGRES_USER" --dbname="$POSTGRES_DB" \
    --no-owner --no-privileges --clean --if-exists "$RESTORE_DIR/postgres.dump"
else
  pg_restore --host="$POSTGRES_HOST" --port="$POSTGRES_PORT" --username="$POSTGRES_USER" --dbname="$POSTGRES_DB" \
    --no-owner --no-privileges "$RESTORE_DIR/postgres.dump"
fi

psql --host="$POSTGRES_HOST" --port="$POSTGRES_PORT" --username="$POSTGRES_USER" --dbname="$POSTGRES_DB" -v ON_ERROR_STOP=1 <<'SQL'
SELECT count(*) AS migrations FROM schema_migrations;
SELECT count(*) AS domains FROM domains;
SELECT count(*) AS urls FROM urls;
SELECT count(*) AS organizations FROM organizations;
SELECT count(*) AS addresses FROM addresses;
SQL

echo "restore verification completed; rebuild disposable Manticore indexes next"
