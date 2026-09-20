#!/bin/sh
set -eu

if [ "$#" -ne 1 ]; then
  echo "usage: deploy/release/apply.sh <version>" >&2
  exit 2
fi

version="$1"
workdir="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
cd "$workdir"

candidate_env="$(mktemp)"
previous_env="$(mktemp)"
trap 'rm -f "$candidate_env" "$previous_env"' EXIT

# Resolve candidate images from the registry using the currently available binary.
docker compose run --rm backend releasectl env-version "$version" > "$candidate_env"
# Keep active images for automatic container rollback if candidate health fails.
if ! docker compose run --rm backend releasectl env active > "$previous_env" 2>/dev/null; then
  : > "$previous_env"
fi

set -a
. "$candidate_env"
set +a
export BACKEND_IMAGE FRONTEND_IMAGE RELEASE_VERSION

# Preflight must be executed by the candidate backend so index schema constants
# represent the binary being deployed, not the previous release.
docker compose pull backend indexer webmaster-worker organizations-worker organization-indexer address-indexer resource-monitor frontend
docker compose run --rm backend releasectl preflight "$version"
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
  echo "candidate health check failed; release registry was not activated" >&2
  if [ -s "$previous_env" ]; then
    set -a
    . "$previous_env"
    set +a
    export BACKEND_IMAGE FRONTEND_IMAGE RELEASE_VERSION
    docker compose up -d --no-build backend indexer webmaster-worker organizations-worker organization-indexer address-indexer resource-monitor frontend nginx || true
  fi
  exit 1
fi

# Candidate is serving successfully; only now make it the active registry release.
docker compose run --rm backend releasectl activate "$version"
echo "release $RELEASE_VERSION applied and activated"
