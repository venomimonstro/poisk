#!/bin/sh
set -eu

workdir="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
cd "$workdir"

previous_env="$(mktemp)"
active_env="$(mktemp)"
trap 'rm -f "$previous_env" "$active_env"' EXIT

# Resolve both immutable image pairs before changing containers or registry state.
docker compose run --rm backend releasectl env previous > "$previous_env"
docker compose run --rm backend releasectl env active > "$active_env"

set -a
. "$previous_env"
set +a
export BACKEND_IMAGE FRONTEND_IMAGE RELEASE_VERSION

docker compose pull backend indexer webmaster-worker organizations-worker organization-indexer address-indexer resource-monitor frontend
docker compose up -d --no-build backend indexer webmaster-worker organizations-worker organization-indexer address-indexer resource-monitor frontend nginx

healthy=no
i=0
while [ "$i" -lt 30 ]; do
  if docker compose exec -T nginx wget -q -O - http://backend:8080/health/ready >/dev/null 2>&1; then
    healthy=yes
    break
  fi
  i=$((i+1))
  sleep 2
done

if [ "$healthy" != "yes" ]; then
  echo "previous release failed health check; registry remains unchanged" >&2
  set -a
  . "$active_env"
  set +a
  export BACKEND_IMAGE FRONTEND_IMAGE RELEASE_VERSION
  docker compose up -d --no-build backend indexer webmaster-worker organizations-worker organization-indexer address-indexer resource-monitor frontend nginx || true
  exit 1
fi

# Previous binary is healthy; only now flip active/previous registry pointers.
docker compose run --rm backend releasectl rollback
echo "rolled back and activated release $RELEASE_VERSION"
