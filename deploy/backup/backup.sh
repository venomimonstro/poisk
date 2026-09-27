#!/bin/sh
set -eu
umask 077

: "${POSTGRES_HOST:=postgres}"
: "${POSTGRES_PORT:=5432}"
: "${POSTGRES_DB:=poisk}"
: "${POSTGRES_USER:=poisk}"
: "${POSTGRES_PASSWORD:?POSTGRES_PASSWORD is required}"
: "${BACKUP_ROOT:=/backups}"
: "${MAIL_BLOB_DIR:=/mail-blobs}"
: "${BACKUP_QUIESCED:=no}"

# PostgreSQL metadata and filesystem mail blobs are two parts of one canonical
# dataset. Without a cross-resource snapshot primitive, require an explicit
# maintenance window so uploads/deletes/GC cannot race the backup.
if [ "$BACKUP_QUIESCED" != "yes" ]; then
  echo "backup refused: set BACKUP_QUIESCED=yes only after stopping API/mail mutators and workers" >&2
  exit 2
fi
[ -d "$MAIL_BLOB_DIR" ] || { echo "backup refused: mail blob directory missing: $MAIL_BLOB_DIR" >&2; exit 2; }

export PGPASSWORD="$POSTGRES_PASSWORD"
stamp="$(date -u +%Y%m%dT%H%M%SZ)"
dir="$BACKUP_ROOT/$stamp"
mkdir -p "$dir"
cleanup(){ [ -d "$dir" ] && [ ! -f "$dir/COMPLETE" ] && rm -rf "$dir"; }
trap cleanup EXIT INT TERM

pg_dump --host="$POSTGRES_HOST" --port="$POSTGRES_PORT" --username="$POSTGRES_USER" \
  --format=custom --compress=6 --no-owner --no-privileges --file="$dir/postgres.dump" "$POSTGRES_DB"
pg_restore --list "$dir/postgres.dump" > "$dir/postgres.list"

# Only committed UUID blobs are canonical. Temporary upload files are never
# archived. tar receives relative names from the mounted blob root.
blob_count="$(find "$MAIL_BLOB_DIR" -maxdepth 1 -type f -name '*.blob' | wc -l | tr -d ' ')"
blob_bytes="$(find "$MAIL_BLOB_DIR" -maxdepth 1 -type f -name '*.blob' -exec stat -c '%s' {} \; | awk '{s+=$1} END{print s+0}')"
tar -C "$MAIL_BLOB_DIR" -czf "$dir/mail-blobs.tar.gz" --wildcards --no-recursion '*.blob' 2>/dev/null || {
  # GNU tar exits non-zero when the wildcard matches nothing. Empty blob stores
  # are valid, so create an empty archive in that case.
  if [ "$blob_count" = "0" ]; then tar -C "$MAIL_BLOB_DIR" -czf "$dir/mail-blobs.tar.gz" --files-from /dev/null; else exit 1; fi
}

cp /safe-config/docker-compose.yml "$dir/docker-compose.yml"
cp /safe-config/default.conf "$dir/nginx-default.conf"

sha256sum "$dir/postgres.dump" "$dir/postgres.list" "$dir/mail-blobs.tar.gz" "$dir/docker-compose.yml" "$dir/nginx-default.conf" > "$dir/SHA256SUMS"
cat > "$dir/MANIFEST" <<EOF
backup_format=poisk-v2
created_at=$stamp
database=$POSTGRES_DB
postgres_host=$POSTGRES_HOST
secrets_included=false
manticore_included=false
mail_blobs_included=true
mail_blob_count=$blob_count
mail_blob_bytes=$blob_bytes
quiesced=true
EOF
# COMPLETE is written last and is intentionally excluded from SHA256SUMS.
printf 'ok\n' > "$dir/COMPLETE"
trap - EXIT INT TERM
printf '%s\n' "$dir"
