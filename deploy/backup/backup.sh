#!/bin/sh
set -eu
umask 077

: "${POSTGRES_HOST:=postgres}"
: "${POSTGRES_PORT:=5432}"
: "${POSTGRES_DB:=poisk}"
: "${POSTGRES_USER:=poisk}"
: "${POSTGRES_PASSWORD:?POSTGRES_PASSWORD is required}"
: "${BACKUP_ROOT:=/backups}"

export PGPASSWORD="$POSTGRES_PASSWORD"
stamp="$(date -u +%Y%m%dT%H%M%SZ)"
dir="$BACKUP_ROOT/$stamp"
mkdir -p "$dir"

pg_dump --host="$POSTGRES_HOST" --port="$POSTGRES_PORT" --username="$POSTGRES_USER" \
  --format=custom --compress=6 --no-owner --no-privileges --file="$dir/postgres.dump" "$POSTGRES_DB"

pg_restore --list "$dir/postgres.dump" > "$dir/postgres.list"
cp /safe-config/docker-compose.yml "$dir/docker-compose.yml"
cp /safe-config/default.conf "$dir/nginx-default.conf"

sha256sum "$dir/postgres.dump" "$dir/postgres.list" "$dir/docker-compose.yml" "$dir/nginx-default.conf" > "$dir/SHA256SUMS"
cat > "$dir/MANIFEST" <<EOF
backup_format=poisk-v1
created_at=$stamp
database=$POSTGRES_DB
postgres_host=$POSTGRES_HOST
secrets_included=false
manticore_included=false
EOF

printf '%s\n' "$dir"
