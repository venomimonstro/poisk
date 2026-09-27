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

if [ "$BACKUP_QUIESCED" != "yes" ]; then
  echo "backup refused: set BACKUP_QUIESCED=yes only after stopping API/mail mutators and workers" >&2
  exit 2
fi
[ -d "$MAIL_BLOB_DIR" ] || { echo "backup refused: mail blob directory missing: $MAIL_BLOB_DIR" >&2; exit 2; }

start_epoch="$(date +%s)"
export PGPASSWORD="$POSTGRES_PASSWORD"
stamp="$(date -u +%Y%m%dT%H%M%SZ)"
dir="$BACKUP_ROOT/$stamp"
mkdir -p "$dir"
cleanup(){ [ -d "$dir" ] && [ ! -f "$dir/COMPLETE" ] && rm -rf "$dir"; }
trap cleanup EXIT INT TERM

# In a quiesced window, DB metadata and blob files must already agree. Refuse
# to bless an existing corruption as a valid backup artifact.
db_blob_list="$dir/mail-db-blobs.list"
psql --host="$POSTGRES_HOST" --port="$POSTGRES_PORT" --username="$POSTGRES_USER" --dbname="$POSTGRES_DB" \
  -v ON_ERROR_STOP=1 -Atc "SELECT storage_key::text || '.blob' FROM mail_attachment_blobs ORDER BY storage_key" > "$db_blob_list"
missing=0
while IFS= read -r name; do
  [ -z "$name" ] && continue
  if [ ! -f "$MAIL_BLOB_DIR/$name" ]; then
    echo "backup refused: DB references missing mail blob $name" >&2
    missing=$((missing+1))
  fi
done < "$db_blob_list"
[ "$missing" -eq 0 ] || exit 3
schema_version="$(psql --host="$POSTGRES_HOST" --port="$POSTGRES_PORT" --username="$POSTGRES_USER" --dbname="$POSTGRES_DB" -v ON_ERROR_STOP=1 -Atc "SELECT COALESCE(max(version),0) FROM schema_migrations")"

pg_dump --host="$POSTGRES_HOST" --port="$POSTGRES_PORT" --username="$POSTGRES_USER" \
  --format=custom --compress=6 --no-owner --no-privileges --file="$dir/postgres.dump" "$POSTGRES_DB"
pg_restore --list "$dir/postgres.dump" > "$dir/postgres.list"

blob_list="$dir/mail-blobs.list"
find "$MAIL_BLOB_DIR" -maxdepth 1 -type f -name '*.blob' -printf '%f\n' | LC_ALL=C sort > "$blob_list"
blob_count="$(wc -l < "$blob_list" | tr -d ' ')"
db_blob_count="$(wc -l < "$db_blob_list" | tr -d ' ')"
blob_bytes="$(find "$MAIL_BLOB_DIR" -maxdepth 1 -type f -name '*.blob' -exec stat -c '%s' {} \; | awk '{s+=$1} END{print s+0}')"
tar -C "$MAIL_BLOB_DIR" -czf "$dir/mail-blobs.tar.gz" -T "$blob_list"

cp /safe-config/docker-compose.yml "$dir/docker-compose.yml"
cp /safe-config/default.conf "$dir/nginx-default.conf"
(
  cd "$dir"
  sha256sum postgres.dump postgres.list mail-db-blobs.list mail-blobs.list mail-blobs.tar.gz docker-compose.yml nginx-default.conf > SHA256SUMS
)
end_epoch="$(date +%s)"
duration_ms="$(( (end_epoch-start_epoch)*1000 ))"
[ "$duration_ms" -gt 0 ] || duration_ms=1
cat > "$dir/MANIFEST" <<EOF
backup_format=poisk-v2
created_at=$stamp
database=$POSTGRES_DB
postgres_host=$POSTGRES_HOST
database_schema=$schema_version
secrets_included=false
manticore_included=false
mail_blobs_included=true
mail_blob_count=$blob_count
mail_db_blob_count=$db_blob_count
mail_blob_bytes=$blob_bytes
quiesced=true
duration_ms=$duration_ms
EOF
printf 'ok\n' > "$dir/COMPLETE"
trap - EXIT INT TERM
printf '%s\n' "$dir"
