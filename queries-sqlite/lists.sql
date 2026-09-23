-- name: get-lists
SELECT * FROM lists
WHERE ($1='' OR type=$1) AND ($2='' OR status=$2)
  AND ($4=1 OR id IN (SELECT value FROM json_each(listmonk_array($5))))
ORDER BY CASE WHEN $3='id' THEN id END, CASE WHEN $3='name' THEN name END;

-- name: query-lists
WITH ls AS (
    SELECT COUNT(*) OVER () AS total, lists.* FROM lists
    WHERE CASE WHEN $1>0 THEN id=$1 WHEN $2!='' THEN uuid=$2
          WHEN $3!='' THEN name LIKE $3 ELSE 1 END
      AND ($4='' OR type=$4) AND ($5='' OR optin=$5) AND ($6='' OR status=$6)
      AND NOT EXISTS (
          SELECT 1 FROM json_each(listmonk_array($7)) requested
          WHERE requested.value NOT IN (SELECT value FROM json_each(listmonk_array(lists.tags))))
      AND ($8=1 OR id IN (SELECT value FROM json_each(listmonk_array($9))))
    LIMIT CASE WHEN $11<1 THEN -1 ELSE $11 END OFFSET $10
), statuses AS (
    SELECT list_id, json_group_object(status, subscriber_count) AS subscriber_statuses,
           SUM(subscriber_count) AS subscriber_count
    FROM mat_list_subscriber_stats WHERE status IS NOT NULL GROUP BY list_id
)
SELECT ls.*, COALESCE(ss.subscriber_statuses, '{}') subscriber_statuses,
       COALESCE(ss.subscriber_count, 0) subscriber_count
FROM ls LEFT JOIN statuses ss ON ls.id=ss.list_id ORDER BY %order%;

-- name: get-lists-by-optin
SELECT * FROM lists WHERE ($1='' OR optin=$1) AND
CASE WHEN $2 IS NOT NULL THEN id IN (SELECT value FROM json_each(listmonk_array($2)))
     WHEN $3 IS NOT NULL THEN uuid IN (SELECT value FROM json_each(listmonk_array($3))) END
ORDER BY name;

-- name: get-list-types
SELECT id, uuid, type FROM lists WHERE
CASE WHEN $1 IS NOT NULL THEN id IN (SELECT value FROM json_each(listmonk_array($1)))
     WHEN $2 IS NOT NULL THEN uuid IN (SELECT value FROM json_each(listmonk_array($2))) END;

-- name: create-list
INSERT INTO lists (uuid, name, type, optin, status, tags, description)
VALUES($1, $2, $3, $4, $5, $6, $7) RETURNING id;

-- name: update-list
UPDATE lists SET name=CASE WHEN $2!='' THEN $2 ELSE name END,
 type=CASE WHEN $3!='' THEN $3 ELSE type END,
 optin=CASE WHEN $4!='' THEN $4 ELSE optin END,
 status=CASE WHEN $5!='' THEN $5 ELSE status END,
 tags=$6, description=CASE WHEN $7!='' THEN $7 ELSE description END,
 updated_at=CURRENT_TIMESTAMP WHERE id=$1;

-- name: update-lists-date
UPDATE lists SET updated_at=CURRENT_TIMESTAMP
WHERE id IN (SELECT value FROM json_each(listmonk_array($1)));

-- name: delete-lists
DELETE FROM lists WHERE
  (json_array_length(listmonk_array($1))>0 AND id IN (SELECT value FROM json_each(listmonk_array($1)))
   OR json_array_length(listmonk_array($1))=0 AND ($2='' OR name LIKE $2))
  AND ($3=1 OR id IN (SELECT value FROM json_each(listmonk_array($4))));
