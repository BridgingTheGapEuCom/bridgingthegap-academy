-- name: CreateAsset :one
INSERT INTO assets.asset (
    origin, owner_draft_id, original_filename, media_type, byte_size, sha256_digest,
    storage_object_id, lifecycle, created_by_user_id
)
VALUES ('AUTHORING_DRAFT', $1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetAsset :one
SELECT *
FROM assets.asset
WHERE id = $1;

-- name: MarkAssetAvailable :one
UPDATE assets.asset
SET lifecycle = 'AVAILABLE', byte_size = $2, sha256_digest = $3, storage_object_id = $4
WHERE id = $1 AND lifecycle = 'PENDING'
RETURNING *;

-- name: DiscardPendingAsset :execrows
DELETE FROM assets.asset
WHERE id = $1 AND lifecycle = 'PENDING';

-- name: ListAvailableAssetsForDraft :many
SELECT *
FROM assets.asset
WHERE owner_draft_id = $1 AND lifecycle = 'AVAILABLE' AND origin = 'AUTHORING_DRAFT'
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountAvailableAssetsForDraft :one
SELECT count(*)
FROM assets.asset
WHERE owner_draft_id = $1 AND lifecycle = 'AVAILABLE' AND origin = 'AUTHORING_DRAFT';

-- name: CreateImportedAvailableAsset :one
INSERT INTO assets.asset (
    origin, import_id, package_asset_key, original_filename, media_type, byte_size, sha256_digest,
    storage_object_id, lifecycle
)
VALUES ('PACKAGE_IMPORT', $1, $2, $3, $4, $5, $6, $7, 'AVAILABLE')
RETURNING *;
