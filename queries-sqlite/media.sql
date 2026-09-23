-- name: insert-media
INSERT INTO media (uuid, filename, thumb, content_type, provider, meta, created_at)
VALUES($1, $2, $3, $4, $5, $6, CURRENT_TIMESTAMP) RETURNING id;

-- name: query-media
SELECT COUNT(*) OVER () AS total, * FROM media
WHERE ($1='' OR filename LIKE $1) AND provider=$2
ORDER BY created_at DESC LIMIT $4 OFFSET $3;

-- name: get-media
SELECT * FROM media WHERE CASE
    WHEN $1 > 0 THEN id=$1 WHEN $2 != '' THEN uuid=$2
    WHEN $3 != '' THEN filename=$3 ELSE 0 END;

-- name: delete-media
DELETE FROM media WHERE id=$1 RETURNING filename;
