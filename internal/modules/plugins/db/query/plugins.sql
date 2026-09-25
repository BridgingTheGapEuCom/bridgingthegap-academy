-- name: RegisterVerificationKey :one
INSERT INTO plugins.verification_key (key_id, public_key, purpose, allowed_plugin_ids, enabled)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (key_id) DO NOTHING
RETURNING *;

-- name: GetVerificationKey :one
SELECT * FROM plugins.verification_key WHERE key_id = $1;

-- name: SetVerificationKeyEnabled :execrows
UPDATE plugins.verification_key SET enabled = $2 WHERE key_id = $1;

-- name: RegisterInstalledRelease :one
INSERT INTO plugins.installed_release (
  installation_id, plugin_id, version, artifact_digest, manifest,
  signature_key_id, signature_value, signing_payload, registered_trust, installed_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
ON CONFLICT (plugin_id, version) DO NOTHING
RETURNING *;

-- name: GetInstalledRelease :one
SELECT * FROM plugins.installed_release WHERE plugin_id = $1 AND version = $2;

-- name: ListInstalledReleases :many
SELECT * FROM plugins.installed_release ORDER BY plugin_id, version;

-- name: RegisterInstalledResource :exec
INSERT INTO plugins.installed_resource (
  installation_id, resource_path, sha256_digest, byte_size, content
) VALUES ($1,$2,$3,$4,$5)
ON CONFLICT (installation_id, resource_path) DO NOTHING;

-- name: GetInstalledResource :one
SELECT r.*
FROM plugins.installed_resource r
JOIN plugins.installed_release p ON p.installation_id = r.installation_id
WHERE p.plugin_id = $1 AND p.version = $2 AND p.artifact_digest = $3
  AND r.resource_path = $4;

-- name: SetInstalledReleaseEnabled :one
UPDATE plugins.installed_release
SET enabled = $2, state = CASE WHEN $2 THEN 'INSTALLED' ELSE 'DISABLED' END
WHERE installation_id = $1
RETURNING *;

-- name: CreateReleaseApproval :exec
INSERT INTO plugins.release_approval (
  approval_id, plugin_id, version, artifact_digest, kind, authority_key_id, approved_at
) VALUES ($1,$2,$3,$4,$5,$6,$7);

-- name: RevokeReleaseApproval :execrows
UPDATE plugins.release_approval SET revoked_at = $2
WHERE approval_id = $1 AND revoked_at IS NULL;

-- name: FindActiveReleaseApproval :one
SELECT * FROM plugins.release_approval
WHERE plugin_id = $1 AND version = $2 AND artifact_digest = $3 AND kind = $4
  AND COALESCE(authority_key_id, '') = $5 AND revoked_at IS NULL;

-- name: ListDashboardWidgetPlacements :many
SELECT * FROM plugins.dashboard_widget_placement ORDER BY position, placement_id;

-- name: CreateDashboardWidgetPlacement :one
INSERT INTO plugins.dashboard_widget_placement (placement_id, plugin_id, plugin_version, artifact_digest, widget_id, configuration, position, enabled, revision, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,(SELECT COALESCE(MAX(position)+1,0) FROM plugins.dashboard_widget_placement),$7,$8,$9,$10)
RETURNING *;

-- name: UpdateDashboardWidgetPlacement :one
UPDATE plugins.dashboard_widget_placement SET configuration=$2, enabled=$3, revision=revision+1, updated_at=$4
WHERE placement_id=$1 AND revision=$5 RETURNING *;

-- name: DeleteDashboardWidgetPlacement :execrows
DELETE FROM plugins.dashboard_widget_placement WHERE placement_id=$1 AND revision=$2;
