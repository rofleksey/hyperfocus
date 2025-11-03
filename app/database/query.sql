-- name: CreateUser :exec
INSERT INTO users (username, password_hash, created, roles)
VALUES ($1, $2, $3, $4);

-- name: DeleteUser :exec
DELETE
FROM users
WHERE username = $1;

-- name: GetUser :one
SELECT *
FROM users
WHERE username = $1;

-- name: SearchUsers :many
SELECT *
FROM users
WHERE username ILIKE @query
ORDER BY username
OFFSET $1 LIMIT $2;

-- name: CountUsers :one
SELECT COUNT(*)
FROM users
WHERE username ILIKE @query;

-- name: SetUserPasswordHash :exec
UPDATE users
SET password_hash = $2
WHERE username = $1;

-- name: SetUserRoles :exec
UPDATE users
SET roles = $2
WHERE username = $1;

-- name: CreateStream :exec
INSERT INTO streams(id, updated)
VALUES ($1, $2)
ON CONFLICT (id) DO NOTHING;

-- name: SetStreamOnline :exec
UPDATE streams
SET online = true,
    updated = $2
WHERE id = $1;

-- name: UpdateStreamData :exec
UPDATE streams
SET player_names = $2
WHERE id = $1;

-- name: UpdateStreamUrl :exec
UPDATE streams
SET url = $2
WHERE id = $1;

-- name: UpdateStaleStreams :exec
UPDATE streams
SET online = false
WHERE updated < $1;

-- name: GetOnlineStreams :many
SELECT *
FROM streams
WHERE online = true;

-- name: GetSettings :one
SELECT *
FROM settings
WHERE id = 1;

-- name: UpdateSettings :exec
UPDATE settings
SET api_key           = $1,
    notification_urls = $2,
    autodelete_days   = $3
WHERE id = 1;

-- name: GetSchemaVersion :one
SELECT version
FROM schema_version;

-- name: SetSchemaVersion :exec
UPDATE schema_version
SET version = $1;
