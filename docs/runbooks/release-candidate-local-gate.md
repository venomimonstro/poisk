# Local Release Candidate Gate

This runner is a convenience layer over the canonical Sprint 28 readiness model. It does **not** make a release commercial-ready by itself.

## Preconditions

The candidate must already be staged in the release registry and have a successful preflight. Set:

```bash
export RELEASE_VERSION=<staged-version>
export READINESS_GIT_SHA=$(git rev-parse HEAD)
export POSTGRES_DSN=<candidate-database-dsn>
```

`READINESS_GIT_SHA` must equal the checked-out HEAD and the tracked worktree must be clean.

Before expensive checks, verify exact candidate binding:

```bash
make candidate
```

This checks the exact repository migration set, applied schema, staged release version, build SHA and preflight state. It does not inspect readiness evidence.

## Build/unit gate

Run without recording evidence:

```bash
make release-gate
```

The runner performs:

- `go vet ./...`
- `go test ./...`
- backend binary build with `-trimpath`
- `npm ci`
- frontend TypeScript check
- Next production build

Artifacts are stored outside the repository by default under `/tmp/poisk-readiness/<release>-<commit>/`.

## Integration/security gate

Set an isolated migrated PostgreSQL test DSN:

```bash
export TEST_DATABASE_URL=<isolated-test-dsn>
make release-gate
```

The runner executes the full integration suite and then the security-sensitive package set separately. The test database must not be production and must be disposable.

## Recording evidence

Only after candidate precheck passes:

```bash
export READINESS_RECORD=1
export READINESS_ACTOR=<operator>
make release-gate
```

The runner uses `readinessctl record-file`. The CLI hashes the regular artifact file itself and rejects symlinks, empty artifacts and artifacts larger than the configured safety bound. The stored evidence remains immutable and bound to the exact release, commit and schema.

For a standalone local artifact use:

```bash
./app readinessctl record-file <TYPE> <PASS|FAIL> <artifact-path> <actor> '<details-json>'
```

Use the older `readinessctl record` only for artifacts whose digest is produced by an external trusted system and cannot be opened locally.

## What this runner deliberately does not certify

It does not create evidence for:

- `FRESH_INSTALL`
- `UPGRADE`
- `BROWSER_SMOKE`
- `EDGE_TLS_PROXY`
- Capacity `ISOLATED_1M`
- BACKUP/RESTORE drills
- `MTA_FLOW`

Those require isolated databases or the real deployment/edge/MTA environment. They must be performed separately according to `security-commercial-readiness.md`.

## Final verdict

After all evidence, Quality, Capacity and Recovery records belong to the same exact candidate:

```bash
make readiness
```

Only `ready=true` is a commercial release authorization. Missing, stale, mismatched or failed evidence keeps the project `NOT READY`.
