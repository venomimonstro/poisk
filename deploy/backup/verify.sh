#!/bin/sh
set -eu

if [ "$#" -ne 1 ]; then
  echo "usage: verify.sh /backups/<timestamp>" >&2
  exit 2
fi
backup_dir="$1"
case "$backup_dir" in /backups/*) ;; *) echo "invalid backup path" >&2; exit 2;; esac
for required in MANIFEST SHA256SUMS COMPLETE postgres.dump postgres.list mail-db-blobs.list mail-blobs.list mail-blobs.tar.gz docker-compose.yml nginx-default.conf; do
  [ -f "$backup_dir/$required" ] || { echo "$required missing" >&2; exit 2; }
done

grep -qx 'backup_format=poisk-v2' "$backup_dir/MANIFEST" || { echo "unsupported backup format" >&2; exit 2; }
grep -qx 'secrets_included=false' "$backup_dir/MANIFEST" || { echo "backup manifest does not prove secrets exclusion" >&2; exit 2; }
grep -qx 'mail_blobs_included=true' "$backup_dir/MANIFEST" || { echo "mail blobs absent" >&2; exit 2; }
grep -qx 'quiesced=true' "$backup_dir/MANIFEST" || { echo "backup was not quiesced" >&2; exit 2; }
grep -Eq '^database_schema=[0-9]+$' "$backup_dir/MANIFEST" || { echo "invalid database_schema in manifest" >&2; exit 2; }
grep -Eq '^mail_blob_count=[0-9]+$' "$backup_dir/MANIFEST" || { echo "invalid mail_blob_count in manifest" >&2; exit 2; }
grep -Eq '^mail_db_blob_count=[0-9]+$' "$backup_dir/MANIFEST" || { echo "invalid mail_db_blob_count in manifest" >&2; exit 2; }

grep -qx 'ok' "$backup_dir/COMPLETE" || { echo "backup COMPLETE marker is invalid" >&2; exit 2; }
(cd "$backup_dir" && sha256sum -c SHA256SUMS)
pg_restore --list "$backup_dir/postgres.dump" >/dev/null

# Both blob manifests must contain only canonical UUID blob names.
for list in mail-db-blobs.list mail-blobs.list; do
  if grep -Ev '^([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})\.blob$' "$backup_dir/$list" >/dev/null; then
    echo "invalid mail blob filename in $list" >&2
    exit 2
  fi
done

# Archive contents must exactly match the filesystem blob list.
tar_list="$(mktemp)"
trap 'rm -f "$tar_list"' EXIT INT TERM
tar -tzf "$backup_dir/mail-blobs.tar.gz" | sed 's#^\./##' | LC_ALL=C sort > "$tar_list"
if ! cmp -s "$backup_dir/mail-blobs.list" "$tar_list"; then
  echo "mail blob archive does not match mail-blobs.list" >&2
  exit 2
fi

manifest_blob_count="$(sed -n 's/^mail_blob_count=//p' "$backup_dir/MANIFEST")"
manifest_db_blob_count="$(sed -n 's/^mail_db_blob_count=//p' "$backup_dir/MANIFEST")"
actual_blob_count="$(wc -l < "$backup_dir/mail-blobs.list" | tr -d ' ')"
actual_db_blob_count="$(wc -l < "$backup_dir/mail-db-blobs.list" | tr -d ' ')"
[ "$manifest_blob_count" = "$actual_blob_count" ] || { echo "mail_blob_count mismatch" >&2; exit 2; }
[ "$manifest_db_blob_count" = "$actual_db_blob_count" ] || { echo "mail_db_blob_count mismatch" >&2; exit 2; }

rm -f "$tar_list"
trap - EXIT INT TERM
printf 'backup_verified=%s format=poisk-v2 schema=%s\n' "$backup_dir" "$(sed -n 's/^database_schema=//p' "$backup_dir/MANIFEST")"
