-- name: CreateAsset :one
INSERT INTO assets.asset (
    owner_draft_id, original_filename, media_type, byte_size, sha256_digest,
    storage_object_id, lifecycle, created_by_user_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
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
