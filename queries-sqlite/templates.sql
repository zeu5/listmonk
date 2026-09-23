-- name: get-templates
SELECT id, name, type, subject,
    CASE WHEN $2=0 THEN body ELSE '' END AS body,
    CASE WHEN $2=0 THEN body_source ELSE NULL END AS body_source,
    is_default, created_at, updated_at
FROM templates WHERE ($1=0 OR id=$1) AND ($3='' OR type=$3)
ORDER BY created_at;

-- name: create-template
INSERT INTO templates (name, type, subject, body, body_source)
VALUES($1, $2, $3, $4, $5) RETURNING id;

-- name: update-template
UPDATE templates SET name=CASE WHEN $2!='' THEN $2 ELSE name END,
    subject=CASE WHEN $3!='' THEN $3 ELSE subject END,
    body=CASE WHEN $4!='' THEN $4 ELSE body END,
    body_source=CASE WHEN $5!='' THEN $5 ELSE body_source END,
    updated_at=CURRENT_TIMESTAMP WHERE id=$1;

-- name: set-default-template
UPDATE templates SET is_default=(id=$1)
WHERE type IN ('campaign', 'campaign_visual');

-- name: delete-template
DELETE FROM templates
WHERE id=$1 AND is_default=0 AND (SELECT COUNT(*) FROM templates)>1
RETURNING id;
