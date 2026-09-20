#!/bin/sh
set -eu

workdir="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
cd "$workdir"

previous_env="$(mktemp)"
trap 'rm -f "$previous_env"' EXIT

# Resolve immutable previous images before flipping registry state.
docker compose run --rm backend releasectl env previous > "$previous_env"
docker compose run --rm backend releasectl rollback

set -a
. "$previous_env"
set +a
export BACKEND_IMAGE FRONTEND_IMAGE RELEASE_VERSION

docker compose pull backend indexer webmaster-worker organizations-worker organization-indexer address-indexer resource-monitor frontend || true
docker compose up -d --no-build backend indexer webmaster-worker organizations-worker organization-indexer address-indexer resource-monitor frontend nginx

echo "rolled back to release $RELEASE_VERSION"
