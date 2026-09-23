-- name: record-bounce
SELECT 1;

-- name: query-bounces
SELECT COUNT(*) OVER () total,b.id,b.type,b.source,b.meta,b.created_at,b.subscriber_id,
 s.uuid subscriber_uuid,s.email,s.status subscriber_status,
 CASE WHEN b.campaign_id IS NOT NULL THEN json_object('id',b.campaign_id,'name',c.name) END campaign
FROM bounces b LEFT JOIN subscribers s ON s.id=b.subscriber_id LEFT JOIN campaigns c ON c.id=b.campaign_id
WHERE ($1=0 OR b.id=$1) AND ($2=0 OR b.campaign_id=$2) AND ($3=0 OR b.subscriber_id=$3) AND ($4='' OR b.source=$4)
ORDER BY %order% LIMIT CASE WHEN $6<1 THEN -1 ELSE $6 END OFFSET $5;

-- name: delete-bounces
DELETE FROM bounces WHERE $2=1 OR id IN (SELECT value FROM json_each(listmonk_array($1)));

-- name: delete-bounces-by-subscriber
DELETE FROM bounces WHERE subscriber_id=(SELECT id FROM subscribers WHERE CASE WHEN $1>0 THEN id=$1 ELSE uuid=$2 END);

-- name: blocklist-bounced-subscribers
SELECT 1;
