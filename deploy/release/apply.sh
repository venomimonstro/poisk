#!/bin/sh
set -eu

if [ "$#" -ne 1 ]; then
  echo "usage: deploy/release/apply.sh <version>" >&2
  exit 2
fi

version="$1"
workdir="$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
cd "$workdir"

docker compose run --rm backend releasectl preflight "$version"
docker compose run --rm backend releasectl activate "$version"

env_file="$(mktemp)"
trap 'rm -f "$env_file"' EXIT

docker compose run --rm backend releasectl env active > "$env_file"
# Only three controlled keys are emitted by releasectl env.
set -a
. "$env_file"
set +a
export BACKEND_IMAGE FRONTEND_IMAGE RELEASE_VERSION

docker compose pull backend indexer webmaster-worker organizations-worker organization-indexer address-indexer resource-monitor frontend || true
docker compose up -d --no-build backend indexer webmaster-worker organizations-worker organization-indexer address-indexer resource-monitor frontend nginx

echo "release $RELEASE_VERSION applied"
