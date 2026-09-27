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
: "${RESTORE_QUIESCED:=no}"
: "${MAIL_BLOB_DIR:=/mail-blobs}"

if [ "$RESTORE_CONFIRM" != "RESTORE:$POSTGRES_DB" ]; then
  echo "restore refused: RESTORE_CONFIRM must equal RESTORE:$POSTGRES_DB" >&2
  exit 2
fi
if [ "$RESTORE_QUIESCED" != "yes" ]; then
  echo "restore refused: set RESTORE_QUIESCED=yes only after stopping API/mail mutators and workers" >&2
  exit 2
fi
case "$RESTORE_DIR" in
  /backups/*) ;;
  *) echo "restore refused: RESTORE_DIR must be below /backups" >&2; exit 2 ;;
esac
for required in MANIFEST SHA256SUMS COMPLETE postgres.dump mail-blobs.list mail-blobs.tar.gz; do
  [ -f "$RESTORE_DIR/$required" ] || { echo "restore refused: $required missing" >&2; exit 2; }
done
grep -qx 'backup_format=poisk-v2' "$RESTORE_DIR/MANIFEST" || { echo "restore refused: unsupported backup format" >&2; exit 2; }
grep -qx 'mail_blobs_included=true' "$RESTORE_DIR/MANIFEST" || { echo "restore refused: mail blobs absent" >&2; exit 2; }
grep -qx 'quiesced=true' "$RESTORE_DIR/MANIFEST" || { echo "restore refused: backup was not quiesced" >&2; exit 2; }

(cd "$RESTORE_DIR" && sha256sum -c SHA256SUMS)
pg_restore --list "$RESTORE_DIR/postgres.dump" >/dev/null

# Refuse path traversal or unexpected archive content before touching the DB.
if grep -Ev '^([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})\.blob$' "$RESTORE_DIR/mail-blobs.list" >/dev/null; then
  echo "restore refused: invalid mail blob filename in manifest" >&2
  exit 2
fi
tar_list="$(mktemp)"
trap 'rm -f "$tar_list"' EXIT INT TERM
tar -tzf "$RESTORE_DIR/mail-blobs.tar.gz" | sed 's#^\./##' | LC_ALL=C sort > "$tar_list"
if ! cmp -s "$RESTORE_DIR/mail-blobs.list" "$tar_list"; then
  echo "restore refused: mail blob archive does not match manifest list" >&2
  exit 2
fi

[ -d "$MAIL_BLOB_DIR" ] || { echo "restore refused: mail blob volume missing: $MAIL_BLOB_DIR" >&2; exit 2; }
stage="$MAIL_BLOB_DIR/.restore-stage"
rm -rf "$stage"
mkdir -m 700 "$stage"
tar -xzf "$RESTORE_DIR/mail-blobs.tar.gz" -C "$stage"

export PGPASSWORD="$POSTGRES_PASSWORD"
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

# The maintenance window guarantees no live writers. Replace only canonical
# blob/tmp files; staging is preserved until every restored DB reference passes.
find "$MAIL_BLOB_DIR" -maxdepth 1 -type f \( -name '*.blob' -o -name '*.tmp' \) -delete
find "$stage" -maxdepth 1 -type f -name '*.blob' -exec mv {} "$MAIL_BLOB_DIR"/ \;

missing=0
while IFS= read -r storage_key; do
  [ -z "$storage_key" ] && continue
  if [ ! -f "$MAIL_BLOB_DIR/$storage_key.blob" ]; then
    echo "restore verification failed: missing mail blob $storage_key.blob" >&2
    missing=$((missing+1))
  fi
done <<EOF
$(psql --host="$POSTGRES_HOST" --port="$POSTGRES_PORT" --username="$POSTGRES_USER" --dbname="$POSTGRES_DB" -v ON_ERROR_STOP=1 -Atc "SELECT storage_key::text FROM mail_attachment_blobs ORDER BY storage_key")
EOF
if [ "$missing" -ne 0 ]; then
  echo "restore failed: $missing referenced mail blobs are missing; staging retained for investigation" >&2
  exit 4
fi
rm -rf "$stage"

psql --host="$POSTGRES_HOST" --port="$POSTGRES_PORT" --username="$POSTGRES_USER" --dbname="$POSTGRES_DB" -v ON_ERROR_STOP=1 <<'SQL'
SELECT count(*) AS migrations FROM schema_migrations;
SELECT count(*) AS domains FROM domains;
SELECT count(*) AS urls FROM urls;
SELECT count(*) AS organizations FROM organizations;
SELECT count(*) AS addresses FROM addresses;
SELECT count(*) AS mailboxes FROM mailboxes;
SELECT count(*) AS mail_attachment_blobs FROM mail_attachment_blobs;
SQL

rm -f "$tar_list"
trap - EXIT INT TERM
echo "restore verification completed; mail blobs restored; rebuild disposable Manticore indexes next"
