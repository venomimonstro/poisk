# Production Hardening Runbook

## Admin bootstrap

1. Generate a 32-byte random secret and base64-encode it. Put it only in production secret storage as `ADMIN_SECRET_KEY_B64`.
2. Apply migrations.
3. Create the first admin from the backend container:

```bash
docker compose run --rm backend adminctl create admin@example.com SUPERADMIN
```

4. Enable TOTP and store the recovery codes offline:

```bash
docker compose run --rm backend adminctl setup-2fa <admin-id>
```

The public application has no admin registration route. `/api/admin` fails closed when the admin secret is unavailable.

## Admin mutation contract

Domain policy/status mutations are two-phase:

1. `POST /api/admin/domains/preview` with session + CSRF.
2. Review before/after state.
3. `POST /api/admin/domains/apply` with the one-time preview token.

Preview tokens are tied to admin + session, expire after five minutes and are consumed atomically with the mutation. Every applied mutation writes `audit_log`.

## Resource pressure

`resource-monitor` samples the mounted filesystem and cgroup memory. State is stored in `system_settings.resource_pressure`.

- NORMAL: normal operation.
- HIGH: warning only.
- CRITICAL: no new crawler or Manticore index outbox leases.
- stale pressure record (>60 seconds): treated as CRITICAL by lease SQL.

Existing leased tasks are allowed to finish. Search/API traffic is not disabled by the resource monitor.

## Backup

Create a deterministic PostgreSQL/config backup:

```bash
docker compose --profile ops run --rm backup
```

The command prints the backup directory. The backup includes `postgres.dump`, `postgres.list`, safe deployment config, `MANIFEST` and `SHA256SUMS`. `.env` and application secrets are intentionally excluded. Manticore is intentionally excluded because indexes are disposable/rebuildable.

Verify a backup without restoring it:

```bash
docker compose --profile ops run --rm backup sh /ops/verify.sh /backups/<timestamp>
```

## Restore

Restore is destructive only with explicit confirmation. Before a real restore, verify the archive and take a fresh backup of the current database if possible.

For an empty target database:

```bash
RESTORE_DIR=/backups/<timestamp> \
RESTORE_CONFIRM=RESTORE:poisk \
docker compose --profile ops run --rm restore
```

For an intentional replacement of a non-empty target:

```bash
RESTORE_DIR=/backups/<timestamp> \
RESTORE_CONFIRM=RESTORE:poisk \
RESTORE_ALLOW_NONEMPTY=yes \
docker compose --profile ops run --rm restore
```

The restore script validates checksums and the PostgreSQL custom archive before any database mutation. `--clean --if-exists` is used only when `RESTORE_ALLOW_NONEMPTY=yes` exactly.

After restore, rebuild all disposable Manticore indexes from PostgreSQL canonical data:

```bash
docker compose run --rm backend indexctl rebuild-web
docker compose run --rm backend orgctl rebuild-index
docker compose run --rm backend addressctl rebuild-index
```

Never restore stale Manticore files over restored PostgreSQL canonical data.

## Release staging

Build and publish immutable backend/frontend images first. Prefer registry digest references. Stage the manifest using the candidate backend image so the stored index-schema constants come from the candidate binary, not the currently active binary.

Example:

```bash
BACKEND_IMAGE=registry.example/poisk/backend@sha256:<candidate-digest> \
docker compose run --rm backend releasectl stage \
  2026.09.20 <git-sha> 19 <map-version-or-dash> <config-sha256> \
  registry.example/poisk/backend@sha256:<candidate-digest> \
  registry.example/poisk/frontend@sha256:<candidate-digest>
```

Release fields are restricted to shell-safe formats because the registry can export image variables to the deployment scripts.

Run preflight:

```bash
docker compose run --rm backend releasectl preflight 2026.09.20
```

Preflight checks database schema, map compatibility, index schema versions and image metadata.

Activate:

```bash
sh deploy/release/apply.sh 2026.09.20
```

## Rollback

Rollback switches to the previously registered immutable image pair. It does not reverse database migrations or overwrite canonical data.

```bash
sh deploy/release/rollback.sh
```

Before releasing a migration that is not backward compatible with the previous application image, do not rely on application rollback. Such a migration requires an ADR and an explicit forward-recovery plan.

## Restore test procedure

At least before a production milestone:

1. Create a fresh backup.
2. Run `verify.sh`.
3. Start an isolated PostgreSQL/PostGIS target with a separate volume/database.
4. Restore the archive into that isolated target.
5. Verify `schema_migrations`, domains, URLs, organizations and addresses.
6. Point a temporary backend/indexer at the restored target and rebuild all disposable indexes with the three commands above.
7. Verify `/health/ready`, Web Search, one GEO query and one address query.
8. Destroy the isolated restore environment.

A backup that has not passed a restore test is not considered a proven recovery point.

## Incident order

For disk/memory pressure: stop new import/crawl sources, confirm `resource_pressure`, preserve Search availability, create a backup if safe, then reclaim space. Do not manually delete PostgreSQL data or Manticore files while workers are writing.

For a bad application release: use release rollback first. For canonical-data corruption: stop relevant writers, preserve evidence/audit records, restore PostgreSQL only when rollback cannot correct the issue, then rebuild Manticore from PostgreSQL.
