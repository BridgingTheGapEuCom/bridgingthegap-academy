-- name: CreateUser :one
INSERT INTO identity.users (status) VALUES ($1) RETURNING *;

-- name: GetUser :one
SELECT * FROM identity.users WHERE id = $1;

-- name: UpdateUserStatus :one
UPDATE identity.users SET status = $2, updated_at = now() WHERE id = $1 RETURNING *;

-- name: CreateUserEmail :one
INSERT INTO identity.user_emails (user_id, normalized_email, display_email, is_primary)
VALUES ($1, $2, $3, $4) RETURNING *;

-- name: GetUserEmailByNormalized :one
SELECT * FROM identity.user_emails WHERE normalized_email = $1;

-- name: GetPrimaryUserEmail :one
SELECT * FROM identity.user_emails WHERE user_id = $1 AND is_primary;

-- name: MarkUserEmailVerified :one
UPDATE identity.user_emails SET verified_at = $2 WHERE id = $1 AND verified_at IS NULL RETURNING *;

-- name: CreateLocalPasswordCredential :one
INSERT INTO identity.local_password_credentials (user_id, password_hash)
VALUES ($1, $2) RETURNING *;

-- name: GetLocalPasswordCredential :one
SELECT * FROM identity.local_password_credentials WHERE user_id = $1;

-- name: ReplaceLocalPasswordHash :one
UPDATE identity.local_password_credentials SET password_hash = $2, updated_at = now()
WHERE user_id = $1 RETURNING *;

-- name: AssignGlobalRole :one
INSERT INTO identity.global_role_assignments (user_id, role, granted_by)
VALUES ($1, $2, $3) RETURNING *;

-- name: ListActiveGlobalRoles :many
SELECT * FROM identity.global_role_assignments WHERE user_id = $1 AND revoked_at IS NULL ORDER BY granted_at DESC;

-- name: HasActiveGlobalRole :one
SELECT EXISTS(SELECT 1 FROM identity.global_role_assignments WHERE user_id = $1 AND role = $2 AND revoked_at IS NULL);

-- name: RevokeGlobalRole :one
UPDATE identity.global_role_assignments SET revoked_at = now()
WHERE user_id = $1 AND role = $2 AND revoked_at IS NULL RETURNING *;

-- name: CreateSession :one
INSERT INTO identity.sessions (user_id, token_digest, expires_at, csrf_token)
VALUES ($1, $2, $3, $4) RETURNING *;

-- name: GetSessionCSRFToken :one
SELECT s.csrf_token FROM identity.sessions s
JOIN identity.users u ON u.id = s.user_id
WHERE s.id = $1 AND s.revoked_at IS NULL AND s.expires_at > now() AND u.status = 'ACTIVE';

-- name: InitializeSessionCSRFToken :one
UPDATE identity.sessions s SET csrf_token = $2
FROM identity.users u
WHERE s.id = $1 AND s.user_id = u.id AND s.csrf_token IS NULL
  AND s.revoked_at IS NULL AND s.expires_at > now() AND u.status = 'ACTIVE'
RETURNING s.csrf_token;

-- name: GetSessionByDigest :one
SELECT * FROM identity.sessions WHERE token_digest = $1;

-- name: GetSessionByID :one
SELECT * FROM identity.sessions WHERE id = $1;

-- name: RevokeSession :one
UPDATE identity.sessions SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL RETURNING *;

-- name: RevokeUserSessions :execrows
UPDATE identity.sessions SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL;

-- name: UpdateSessionLastSeen :one
UPDATE identity.sessions SET last_seen_at = now() WHERE id = $1 AND revoked_at IS NULL RETURNING *;
