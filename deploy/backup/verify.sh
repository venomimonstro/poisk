#!/bin/sh
set -eu

if [ "$#" -ne 1 ]; then
  echo "usage: verify.sh /backups/<timestamp>" >&2
  exit 2
fi
backup_dir="$1"
case "$backup_dir" in /backups/*) ;; *) echo "invalid backup path" >&2; exit 2;; esac
[ -f "$backup_dir/MANIFEST" ] || { echo "MANIFEST missing" >&2; exit 2; }
[ -f "$backup_dir/SHA256SUMS" ] || { echo "SHA256SUMS missing" >&2; exit 2; }
[ -f "$backup_dir/postgres.dump" ] || { echo "postgres.dump missing" >&2; exit 2; }
grep -qx 'backup_format=poisk-v1' "$backup_dir/MANIFEST" || { echo "unsupported backup format" >&2; exit 2; }
(cd "$backup_dir" && sha256sum -c SHA256SUMS)
pg_restore --list "$backup_dir/postgres.dump" >/dev/null
printf 'backup_verified=%s\n' "$backup_dir"
