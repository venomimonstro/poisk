-- name: CreateDomain :one
INSERT INTO domains (host)
VALUES ($1)
ON CONFLICT (host) DO UPDATE SET updated_at = now()
RETURNING *;

-- name: GetDomainByHost :one
SELECT * FROM domains WHERE host = $1;

-- name: SetDomainPolicy :one
UPDATE domains
SET policy = $2,
    updated_at = now()
WHERE domain_id = $1
RETURNING *;

-- name: CreateURL :one
INSERT INTO urls (domain_id, normalized_url, discovered_from_url_id)
VALUES ($1, $2, $3)
ON CONFLICT (normalized_url) DO UPDATE SET
    updated_at = now()
RETURNING *;

-- name: GetURL :one
SELECT * FROM urls WHERE url_id = $1;

-- name: GetURLByNormalizedURL :one
SELECT * FROM urls WHERE normalized_url = $1;

-- name: CreateDocumentVersion :one
INSERT INTO document_versions (
    url_id,
    version,
    http_status,
    content_hash,
    simhash,
    content_length,
    content_type,
    extraction_status,
    metadata,
    fetched_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
RETURNING *;

-- name: GetLatestDocumentVersion :one
SELECT *
FROM document_versions
WHERE url_id = $1
ORDER BY version DESC
LIMIT 1;

-- name: PutSystemSetting :one
INSERT INTO system_settings (key, value, updated_at)
VALUES ($1, $2, now())
ON CONFLICT (key) DO UPDATE SET
    value = EXCLUDED.value,
    updated_at = now()
RETURNING *;

-- name: GetSystemSetting :one
SELECT * FROM system_settings WHERE key = $1;

-- name: AppendAuditLog :one
INSERT INTO audit_log (
    actor_type,
    actor_id,
    action,
    entity_type,
    entity_id,
    details
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;
