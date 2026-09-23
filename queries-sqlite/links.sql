-- name: create-link
INSERT INTO links (uuid, url) VALUES($1, $2)
ON CONFLICT (url) DO UPDATE SET url=excluded.url RETURNING uuid;

-- name: get-link-url
SELECT url FROM links WHERE uuid=$1;

-- name: register-link-click
INSERT INTO link_clicks (campaign_id, subscriber_id, link_id)
VALUES((SELECT id FROM campaigns WHERE uuid=$2),
       (SELECT id FROM subscribers WHERE $3 != '' AND uuid=$3),
       (SELECT id FROM links WHERE uuid=$1))
RETURNING (SELECT url FROM links WHERE uuid=$1);
