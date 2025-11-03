-- name: CreateStream :exec
INSERT INTO streams(id, updated)
VALUES ($1, $2) ON CONFLICT (id) DO NOTHING;

-- name: SetStreamOnline :exec
UPDATE streams
SET online  = true,
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

-- name: SearchStreamsByNickname :many
SELECT *
FROM streams
WHERE online = true
  AND EXISTS (SELECT 1
              FROM unnest(player_names) AS nickname
              WHERE levenshtein(lower(nickname), lower(@query::VARCHAR(255))) < @distance::INTEGER OR
                                                                                          lower(nickname) LIKE '%' ||
                                                                                                               lower(@query::VARCHAR(255)) ||
                                                                                                               '%')
  LIMIT @max_results::INTEGER;

-- name: GetSchemaVersion :one
SELECT version
FROM schema_version;

-- name: SetSchemaVersion :exec
UPDATE schema_version
SET version = $1;
