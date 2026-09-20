# Maps — offline build, immutable publish, rollback

Sprint 12 keeps map generation outside the online Search API. Production serves immutable PMTiles and style files from nginx; the Go API only exposes the active validated map version.

## Artifact layout

The host directory `data/maps` is mounted read-only as `/maps` in backend and nginx:

```text
data/maps/
  tiles/
    ru-2026-09-20.pmtiles
  styles/
    ru-2026-09-20.json
  manifests/
    ru-2026-09-20.json
```

Never overwrite an activated file. A new OSM update gets new filenames and a new map version. Old files stay available so rollback remains possible.

## Build from OpenStreetMap

Use an OSM `.osm.pbf` extract from a source whose redistribution terms you have reviewed. tilemaker can generate PMTiles directly from OSM PBF; map generation is an offline operator job, not a production request path.

Example using tilemaker's Docker image:

```bash
docker run --rm -v "$PWD:/data" ghcr.io/systemed/tilemaker:master \
  /data/region-latest.osm.pbf \
  --output /data/ru-2026-09-20.pmtiles \
  --config /data/config-openmaptiles.json \
  --process /data/process-openmaptiles.lua
```

For a production build, pin the tilemaker image to a reviewed release or digest instead of a moving tag.

## Style contract

The style must be MapLibre Style Specification v8. `source_name` in the manifest is the exact style source key, not a human-readable title.

Example source:

```json
{
  "version": 8,
  "sources": {
    "osm": {
      "type": "vector",
      "url": "pmtiles:///maps/tiles/ru-2026-09-20.pmtiles"
    }
  },
  "layers": []
}
```

The validator rejects style imports and external HTTP(S) tile, glyph or sprite dependencies. Keep any additional style assets self-hosted and immutable.

## Generate manifest

Place the PMTiles and style files under `data/maps`, then generate the manifest from the actual PMTiles header and file hashes:

```bash
docker compose run --rm backend mapctl manifest \
  ru-2026-09-20 \
  tiles/ru-2026-09-20.pmtiles \
  styles/ru-2026-09-20.json \
  osm \
  "© OpenStreetMap contributors" \
  > data/maps/manifests/ru-2026-09-20.json
```

The command reads the PMTiles v3 header, derives bounds/minzoom/maxzoom/center, calculates SHA-256 and file sizes, validates the style binding and prints the resulting JSON.

## Register and activate

Apply database migrations first, then:

```bash
docker compose run --rm backend mapctl register manifests/ru-2026-09-20.json
docker compose run --rm backend mapctl activate ru-2026-09-20
docker compose run --rm backend mapctl status
```

Activation performs the full integrity check again before the database pointer changes. `/api/map/config` exposes only the active version.

## Rollback

```bash
docker compose run --rm backend mapctl rollback
```

Rollback also validates the previous artifacts before switching. The database keeps both `active_version` and `previous_version`, so a failed release does not require editing styles or filenames in place.

## Runtime serving

nginx serves `/maps/*` directly with byte-range support and immutable caching. PMTiles requests never stream through Go. The browser registers the `pmtiles://` protocol in MapLibre and reads ranges from the same-origin `/maps/...pmtiles` URL.

## Release checklist

1. New immutable PMTiles/style filenames.
2. Generate manifest from files, never hand-enter SHA/size/bounds.
3. Register version.
4. Open the style and verify the configured source key.
5. Activate version.
6. Check `/api/map/config` and `/map`.
7. Verify a real PMTiles request returns `206 Partial Content` for a `Range` request.
8. Keep the previous artifact set until at least the next successful map release.
9. If the map is broken, run `mapctl rollback`; do not overwrite the active files.
