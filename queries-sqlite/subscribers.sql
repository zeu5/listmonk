-- subscribers
-- name: get-subscriber
-- Get a single subscriber by id or UUID or email.
SELECT * FROM subscribers WHERE
    CASE
        WHEN $1 > 0 THEN id = $1
        WHEN $2 != '' THEN uuid = $2
        WHEN $3 != '' THEN email = $3
    END;

-- name: has-subscriber-list
-- Used for checking access permission by list.
SELECT s.id AS subscriber_id,
    CASE
        WHEN EXISTS (SELECT 1 FROM subscriber_lists sl WHERE sl.subscriber_id = s.id AND sl.list_id IN (SELECT value FROM json_each(listmonk_array($2))))
        THEN 1
        ELSE 0
    END AS has
FROM subscribers s WHERE s.id IN (SELECT value FROM json_each(listmonk_array($1)));

-- name: get-subscribers-by-emails
-- Get subscribers by emails.
SELECT * FROM subscribers WHERE email IN (SELECT value FROM json_each(listmonk_array($1)));

-- name: get-subscriber-lists
WITH sub AS (
    SELECT id FROM subscribers WHERE CASE WHEN $1 > 0 THEN id = $1 ELSE uuid = $2 END
)
SELECT * FROM lists
    LEFT JOIN subscriber_lists ON (lists.id = subscriber_lists.list_id)
    WHERE subscriber_id = (SELECT id FROM sub)
    -- Optional list IDs or UUIDs to filter.
    AND (CASE WHEN json_array_length(listmonk_array($3)) > 0 THEN id IN (SELECT value FROM json_each(listmonk_array($3)))
          WHEN json_array_length(listmonk_array($4)) > 0 THEN uuid IN (SELECT value FROM json_each(listmonk_array($4)))
          ELSE 1
    END)
    AND (CASE WHEN $5 != '' THEN subscriber_lists.status = $5 ELSE 1 END)
    AND (CASE WHEN $6 != '' THEN lists.optin = $6 ELSE 1 END)
    ORDER BY id;

-- name: get-subscriber-lists-lazy
-- Get lists associations of subscribers given a list of subscriber IDs.
-- This query is used to lazy load given a list of subscriber IDs.
-- The query returns results in the same order as the given subscriber IDs, and for non-existent subscriber IDs,
-- the query still returns a row with 0 values. Thus, for lazy loading, the application simply iterate on the results in
-- the same order as the list of campaigns it would've queried and attach the results.
WITH subs AS (
 SELECT sl.subscriber_id,json_group_array(json_object('id',l.id,'uuid',l.uuid,'name',l.name,'type',l.type,'optin',l.optin,'status',l.status,'tags',json(listmonk_array(l.tags)),'description',l.description,'created_at',l.created_at,'updated_at',l.updated_at,'subscription_status',sl.status,'subscription_created_at',sl.created_at,'subscription_updated_at',sl.updated_at,'subscription_meta',json(sl.meta))) lists
 FROM subscriber_lists sl JOIN lists l ON l.id=sl.list_id
 WHERE sl.subscriber_id IN (SELECT value FROM json_each(listmonk_array($1))) GROUP BY sl.subscriber_id)
SELECT CAST(ids.value AS INTEGER) subscriber_id,COALESCE(subs.lists,'[]') lists
FROM json_each(listmonk_array($1)) ids LEFT JOIN subs ON subs.subscriber_id=ids.value ORDER BY ids.key;

-- name: get-subscriptions
-- Retrieves all lists a subscriber is attached to.
-- if $3 is set to 1, all lists are fetched including the subscriber's subscriptions.
-- subscription_status, and subscription_created_at are null in that case.
WITH sub AS (
    SELECT id FROM subscribers WHERE CASE WHEN $1 > 0 THEN id = $1 ELSE uuid = $2 END
)
SELECT lists.*,
    subscriber_lists.status as subscription_status,
    subscriber_lists.created_at as subscription_created_at,
    subscriber_lists.meta as subscription_meta
    FROM lists LEFT JOIN subscriber_lists
    ON (subscriber_lists.list_id = lists.id AND subscriber_lists.subscriber_id = (SELECT id FROM sub))
    WHERE CASE WHEN $3 = 1 THEN 1 ELSE subscriber_lists.status IS NOT NULL END
    ORDER BY subscriber_lists.status;

-- name: insert-subscriber
WITH sub AS (
    INSERT INTO subscribers (uuid, email, name, status, attribs)
    VALUES($1, $2, $3, $4, $5)
    RETURNING id, status
),
listIDs AS (
    SELECT id FROM lists WHERE
        (CASE WHEN json_array_length(listmonk_array($6)) > 0 THEN id IN (SELECT value FROM json_each(listmonk_array($6)))
              ELSE uuid IN (SELECT value FROM json_each(listmonk_array($7))) END)
),
subs AS (
    INSERT INTO subscriber_lists (subscriber_id, list_id, status)
    VALUES(
        (SELECT id FROM sub),
        UNNEST(ARRAY(SELECT id FROM listIDs)),
        (CASE WHEN $4='blocklisted' THEN 'unsubscribed' ELSE $8 END)
    )
    ON CONFLICT (subscriber_id, list_id) DO UPDATE
        SET updated_at=CURRENT_TIMESTAMP,
            status=(
                CASE WHEN $4='blocklisted' OR (SELECT status FROM sub)='blocklisted'
                THEN 'unsubscribed'
                ELSE $8 END
            )
)
SELECT id from sub;

-- name: upsert-subscriber
-- Upserts a subscriber where existing subscribers get their names and attributes overwritten.
-- If $7 = 1, update name/attribs. If $8 = 1, update subscription status.
WITH sub AS (
    INSERT INTO subscribers as s (uuid, email, name, attribs, status)
    VALUES($1, $2, $3, $4, 'enabled')
    ON CONFLICT (email)
    DO UPDATE SET
        name=(CASE WHEN $7 THEN $3 ELSE s.name END),
        attribs=(CASE WHEN $7 THEN $4 ELSE s.attribs END),
        updated_at=CURRENT_TIMESTAMP
    RETURNING uuid, id, status
),
subs AS (
    INSERT INTO subscriber_lists (subscriber_id, list_id, status)
    SELECT sub.id, listID, CASE WHEN sub.status = 'blocklisted' THEN 'unsubscribed' ELSE $6 END
    FROM sub, UNNEST($5) AS listID
    ON CONFLICT (subscriber_id, list_id) DO UPDATE
    SET updated_at = CURRENT_TIMESTAMP,
        status = CASE WHEN $8 THEN EXCLUDED.status ELSE subscriber_lists.status END
)
SELECT uuid, id from sub;

-- name: upsert-blocklist-subscriber
-- Upserts a subscriber where the update will only set the status to blocklisted
-- unlike upsert-subscribers where name and attributes are updated. In addition, all
-- existing subscriptions are marked as 'unsubscribed'.
-- This is used in the bulk importer.
WITH sub AS (
    INSERT INTO subscribers (uuid, email, name, attribs, status)
    VALUES($1, $2, $3, $4, 'blocklisted')
    ON CONFLICT (email) DO UPDATE SET status='blocklisted', updated_at=CURRENT_TIMESTAMP
    RETURNING id
)
UPDATE subscriber_lists SET status='unsubscribed', updated_at=CURRENT_TIMESTAMP
    WHERE subscriber_id = (SELECT id FROM sub);

-- name: update-subscriber
UPDATE subscribers SET
    email=(CASE WHEN $2 != '' THEN $2 ELSE email END),
    name=(CASE WHEN $3 != '' THEN $3 ELSE name END),
    status=(CASE WHEN $4 != '' THEN $4 ELSE status END),
    attribs=(CASE WHEN $5 != '' THEN $5 ELSE attribs END),
    updated_at=CURRENT_TIMESTAMP
WHERE id = $1;

-- name: update-subscriber-with-lists
-- Updates a subscriber's data, and given a list of list_ids, inserts subscriptions
-- for them while deleting existing subscriptions not in the list.
WITH s AS (
    UPDATE subscribers SET
        email=(CASE WHEN $2 != '' THEN $2 ELSE email END),
        name=(CASE WHEN $3 != '' THEN $3 ELSE name END),
        status=(CASE WHEN $4 != '' THEN $4 ELSE status END),
        attribs=(CASE WHEN $5 != '' THEN $5 ELSE attribs END),
        updated_at=CURRENT_TIMESTAMP
    WHERE id = $1 RETURNING id
),
listIDs AS (
    SELECT id FROM lists WHERE
        (CASE WHEN json_array_length(listmonk_array($6)) > 0 THEN id IN (SELECT value FROM json_each(listmonk_array($6)))
              ELSE uuid IN (SELECT value FROM json_each(listmonk_array($7))) END)
),
d AS (
    DELETE FROM subscriber_lists WHERE $9 = 1 AND subscriber_id = $1
        AND list_id != ALL(SELECT id FROM listIDs)
        AND (json_array_length(listmonk_array($10)) = 0 OR list_id IN (SELECT value FROM json_each(listmonk_array($10))))
)
INSERT INTO subscriber_lists (subscriber_id, list_id, status)
    VALUES(
        (SELECT id FROM s),
        UNNEST(ARRAY(SELECT id FROM listIDs)),
        (CASE WHEN $4='blocklisted' THEN 'unsubscribed' ELSE $8 END)
    )
    ON CONFLICT (subscriber_id, list_id) DO UPDATE
    SET status = (
        CASE
            WHEN $4='blocklisted' THEN 'unsubscribed'
            -- When $11 (allow resubscribe) is 1, override existing statuses except confirmed (used by
            -- public subscription form).
            WHEN subscriber_lists.status = 'confirmed' THEN 'confirmed'
            WHEN $11 = 1 THEN $8
            -- When subscriber is edited from the admin form, retain the status. Otherwise, a blocklisted
            -- subscriber when being re-enabled, their subscription statuses change.
            WHEN subscriber_lists.status = 'unsubscribed' THEN 'unsubscribed'
            ELSE $8
        END
    );

-- name: delete-subscribers
-- Delete one or more subscribers by ID or UUID.
DELETE FROM subscribers WHERE CASE WHEN json_array_length(listmonk_array($1)) > 0 THEN id IN (SELECT value FROM json_each(listmonk_array($1))) ELSE uuid IN (SELECT value FROM json_each(listmonk_array($2))) END;

-- name: delete-blocklisted-subscribers
DELETE FROM subscribers WHERE status = 'blocklisted';

-- name: delete-orphan-subscribers
DELETE FROM subscribers WHERE NOT EXISTS
    (SELECT 1 FROM subscriber_lists b WHERE b.subscriber_id = subscribers.id);

-- name: blocklist-subscribers
UPDATE subscribers SET status='blocklisted', updated_at=CURRENT_TIMESTAMP
WHERE id IN (SELECT value FROM json_each(listmonk_array($1)));

-- name: add-subscribers-to-lists
INSERT INTO subscriber_lists (subscriber_id, list_id, status)
SELECT s.value,l.value,CASE WHEN $3!='' THEN $3 ELSE 'unconfirmed' END
FROM json_each(listmonk_array($1)) s CROSS JOIN json_each(listmonk_array($2)) l WHERE 1
ON CONFLICT(subscriber_id,list_id) DO UPDATE SET status=CASE WHEN $3!='' THEN $3 ELSE subscriber_lists.status END;

-- name: delete-subscriptions
DELETE FROM subscriber_lists WHERE subscriber_id IN (SELECT value FROM json_each(listmonk_array($1)))
AND list_id IN (SELECT value FROM json_each(listmonk_array($2)));

-- name: confirm-subscription-optin
WITH subID AS (
    SELECT id FROM subscribers WHERE uuid = $1
),
listIDs AS (
    SELECT id FROM lists WHERE uuid IN (SELECT value FROM json_each(listmonk_array($2)))
)
UPDATE subscriber_lists SET status='confirmed', meta=meta || $3, updated_at=CURRENT_TIMESTAMP
    WHERE subscriber_id = (SELECT id FROM subID) AND list_id IN (SELECT id FROM listIDs);

-- name: unsubscribe-subscribers-from-lists
UPDATE subscriber_lists SET status='unsubscribed', updated_at=CURRENT_TIMESTAMP
WHERE subscriber_id IN (SELECT value FROM json_each(listmonk_array($1))) AND list_id IN
(SELECT id FROM lists WHERE CASE WHEN json_array_length(listmonk_array($2))>0 THEN id IN (SELECT value FROM json_each(listmonk_array($2))) ELSE uuid IN (SELECT value FROM json_each(listmonk_array($3))) END);

-- name: unsubscribe-by-campaign
-- Unsubscribes a subscriber given a campaign UUID (from all the lists in the campaign) and the subscriber UUID.
-- If $3 is 1, then all subscriptions of the subscriber is blocklisted
-- and all existing subscriptions, irrespective of lists, unsubscribed.
WITH lists AS (
    SELECT list_id FROM campaign_lists
    LEFT JOIN campaigns ON (campaign_lists.campaign_id = campaigns.id)
    WHERE campaigns.uuid = $1
),
sub AS (
    UPDATE subscribers SET status = (CASE WHEN $3 IS 1 THEN 'blocklisted' ELSE status END)
    WHERE uuid = $2 RETURNING id
)
UPDATE subscriber_lists SET status = 'unsubscribed', updated_at=CURRENT_TIMESTAMP WHERE
    subscriber_id = (SELECT id FROM sub) AND status != 'unsubscribed' AND
    -- If $3 is 0, unsubscribe from the campaign's lists, otherwise all lists.
    CASE WHEN $3 IS 0 THEN list_id IN (SELECT list_id FROM lists) ELSE list_id != 0 END;

-- name: delete-unconfirmed-subscriptions
WITH optins AS (
    SELECT id FROM lists WHERE optin = 'double'
)
DELETE FROM subscriber_lists
    WHERE status = 'unconfirmed' AND list_id IN (SELECT id FROM optins) AND created_at < $1;


-- Partial and RAW queries used to construct arbitrary subscriber
-- queries for segmentation follow.

-- name: query-subscribers
-- raw: 1
-- Unprepared statement for issuring arbitrary WHERE conditions for
-- searching subscribers. While the results are sliced using offset+limit,
-- there's a COUNT() OVER() that still returns the total result count
-- for pagination in the frontend, albeit being a field that'll repeat
-- with every resultant row.
SELECT subscribers.* FROM subscribers
    LEFT JOIN subscriber_lists
    ON (
        -- Optional list filtering.
        (CASE WHEN json_array_length(listmonk_array($1)) > 0 THEN 1 ELSE 0 END)
        AND subscriber_lists.subscriber_id = subscribers.id
        AND ($2 = '' OR subscriber_lists.status = $2)
    )
    WHERE (json_array_length(listmonk_array($1)) = 0 OR subscriber_lists.list_id IN (SELECT value FROM json_each(listmonk_array($1))))
    AND (CASE WHEN $3 != '' THEN name LIKE $3 OR email LIKE $3 ELSE 1 END)
    AND %query%
    ORDER BY %order% LIMIT (CASE WHEN $5 < 1 THEN -1 ELSE $5 END) OFFSET $4;

-- name: query-subscribers-count
-- Replica of query-subscribers for obtaining the results count.
SELECT COUNT(*) AS total FROM subscribers
    LEFT JOIN subscriber_lists
    ON (
        -- Optional list filtering.
        (CASE WHEN json_array_length(listmonk_array($1)) > 0 THEN 1 ELSE 0 END)
        AND subscriber_lists.subscriber_id = subscribers.id
        AND ($2 = '' OR subscriber_lists.status = $2)
    )
    WHERE (json_array_length(listmonk_array($1)) = 0 OR subscriber_lists.list_id IN (SELECT value FROM json_each(listmonk_array($1))))
    AND (CASE WHEN $3 != '' THEN name LIKE $3 OR email LIKE $3 ELSE 1 END)
    AND %query%;

-- name: query-subscribers-count-all
-- Cached query for getting the "all" subscriber count without arbitrary conditions.
SELECT COALESCE(SUM(subscriber_count), 0) AS total FROM mat_list_subscriber_stats
    WHERE list_id IN (SELECT value FROM json_each(CASE WHEN json_array_length(listmonk_array($1))>0 THEN listmonk_array($1) ELSE '[0]' END))
    AND ($2 = '' OR status = $2);

-- name: query-subscribers-for-export
-- raw: 1
-- Unprepared statement for issuring arbitrary WHERE conditions for
-- searching subscribers to do bulk CSV export.
SELECT subscribers.id,
       subscribers.uuid,
       subscribers.email,
       subscribers.name,
       subscribers.status,
       subscribers.attribs,
       subscribers.created_at,
       subscribers.updated_at
       FROM subscribers
    LEFT JOIN subscriber_lists
    ON (
        -- Optional list filtering.
        (CASE WHEN json_array_length(listmonk_array($1)) > 0 THEN 1 ELSE 0 END)
        AND subscriber_lists.subscriber_id = subscribers.id
        AND ($4 = '' OR subscriber_lists.status = $4)
    )
    WHERE (json_array_length(listmonk_array($1))=0 OR subscriber_lists.list_id IN (SELECT value FROM json_each(listmonk_array($1)))) AND subscribers.id > $2
    AND (CASE WHEN json_array_length(listmonk_array($3)) > 0 THEN id IN (SELECT value FROM json_each(listmonk_array($3))) ELSE 1 END)
    AND (CASE WHEN $5 != '' THEN name LIKE $5 OR email LIKE $5 ELSE 1 END)
    AND %query%
    ORDER BY subscribers.id ASC LIMIT (CASE WHEN $6 < 1 THEN NULL ELSE $6 END);

-- name: query-subscribers-template
-- raw: 1
-- This raw query is reused in multiple queries (blocklist, add to list, delete)
-- etc., so it's kept has a raw template to be injected into other raw queries,
-- and for the same reason, it is not terminated with a semicolon.
--
-- All queries that embed this query should expect
-- $1=1/0 (dry-run or not) and $2=INT (option list IDs).
-- That is, their positional arguments should start from $4.
SELECT subscribers.id FROM subscribers
LEFT JOIN subscriber_lists
ON (
    -- Optional list filtering.
    (CASE WHEN json_array_length(listmonk_array($2)) > 0 THEN 1 ELSE 0 END)
    AND subscriber_lists.subscriber_id = subscribers.id
    AND ($3 = '' OR subscriber_lists.status = $3)
)
WHERE (json_array_length(listmonk_array($2))=0 OR subscriber_lists.list_id IN (SELECT value FROM json_each(listmonk_array($2))))
    AND (CASE WHEN $4 != '' THEN name LIKE $4 OR email LIKE $4 ELSE 1 END)
    AND %query%
LIMIT (CASE WHEN $1 THEN 1 END)

-- name: delete-subscribers-by-query
-- raw: 1
WITH subs AS (%query%)
DELETE FROM subscribers WHERE id IN (SELECT id FROM subs);

-- name: blocklist-subscribers-by-query
-- raw: 1
WITH subs AS (%query%)
UPDATE subscribers SET status='blocklisted',updated_at=CURRENT_TIMESTAMP WHERE id IN (SELECT id FROM subs);

-- name: add-subscribers-to-lists-by-query
-- raw: 1
WITH subs AS (%query%)
INSERT INTO subscriber_lists (subscriber_id, list_id, status)
SELECT subs.id,lists.value,CASE WHEN $6!='' THEN $6 ELSE 'unconfirmed' END FROM subs CROSS JOIN json_each(listmonk_array($5)) lists WHERE 1
ON CONFLICT (subscriber_id,list_id) DO NOTHING;

-- name: delete-subscriptions-by-query
-- raw: 1
WITH subs AS (%query%)
DELETE FROM subscriber_lists
WHERE subscriber_id IN (SELECT id FROM subs) AND list_id IN (SELECT value FROM json_each(listmonk_array($5)));

-- name: unsubscribe-subscribers-from-lists-by-query
-- raw: 1
WITH subs AS (%query%)
UPDATE subscriber_lists SET status='unsubscribed', updated_at=CURRENT_TIMESTAMP
WHERE subscriber_id IN (SELECT id FROM subs) AND list_id IN (SELECT value FROM json_each(listmonk_array($5)));


-- privacy
-- name: export-subscriber-data
WITH prof AS (
    SELECT id, uuid, email, name, attribs, status, created_at, updated_at FROM subscribers WHERE
    CASE WHEN $1 > 0 THEN id = $1 ELSE uuid = $2 END
),
subs AS (
    SELECT subscriber_lists.status AS subscription_status,
            (CASE WHEN lists.type = 'private' THEN 'Private list' ELSE lists.name END) as name,
            lists.type, subscriber_lists.created_at
    FROM lists
    LEFT JOIN subscriber_lists ON (subscriber_lists.list_id = lists.id)
    WHERE subscriber_lists.subscriber_id = (SELECT id FROM prof)
),
views AS (
    SELECT subject as campaign, COUNT(subscriber_id) as views FROM campaign_views
        LEFT JOIN campaigns ON (campaigns.id = campaign_views.campaign_id)
        WHERE subscriber_id = (SELECT id FROM prof)
        GROUP BY campaigns.id ORDER BY campaigns.id
),
clicks AS (
    SELECT url, COUNT(subscriber_id) as clicks FROM link_clicks
        LEFT JOIN links ON (links.id = link_clicks.link_id)
        WHERE subscriber_id = (SELECT id FROM prof)
        GROUP BY links.id ORDER BY links.id
)
SELECT (SELECT email FROM prof) email,
 CAST(COALESCE((SELECT json_group_array(json_object('id',id,'uuid',uuid,'email',email,'name',name,'attribs',json(attribs),'status',status,'created_at',created_at,'updated_at',updated_at)) FROM prof),'[]') AS BLOB) profile,
 CAST(COALESCE((SELECT json_group_array(json_object('subscription_status',subscription_status,'name',name,'type',type,'created_at',created_at)) FROM subs),'[]') AS BLOB) subscriptions,
 CAST(COALESCE((SELECT json_group_array(json_object('campaign',campaign,'views',views)) FROM views),'[]') AS BLOB) campaign_views,
 CAST(COALESCE((SELECT json_group_array(json_object('url',url,'clicks',clicks)) FROM clicks),'[]') AS BLOB) link_clicks;

-- name: get-subscriber-activity
-- Gets the subscriber's campaign views and link clicks with detailed information
-- for display in the Activity tab
WITH views AS (
    SELECT
        c.id,
        c.uuid,
        c.name,
        c.subject,
        COUNT(*) as view_count,
        MAX(cv.created_at) as last_viewed_at
    FROM campaign_views cv
    LEFT JOIN campaigns c ON c.id = cv.campaign_id
    WHERE cv.subscriber_id = $1
    GROUP BY c.id, c.uuid, c.name, c.subject
    ORDER BY last_viewed_at DESC
),
clicks AS (
    SELECT
        l.id as link_id,
        l.url,
        c.id as campaign_id,
        c.uuid as campaign_uuid,
        c.name as campaign_name,
        c.subject as campaign_subject,
        COUNT(*) as click_count,
        MAX(lc.created_at) as last_clicked_at
    FROM link_clicks lc
    LEFT JOIN links l ON l.id = lc.link_id
    LEFT JOIN campaigns c ON c.id = lc.campaign_id
    WHERE lc.subscriber_id = $1
    GROUP BY l.id, l.url, c.id, c.uuid, c.name, c.subject
    ORDER BY last_clicked_at DESC
)
SELECT
 CAST(COALESCE((SELECT json_group_array(json_object('id',id,'uuid',uuid,'name',name,'subject',subject,'view_count',view_count,'last_viewed_at',last_viewed_at)) FROM views),'[]') AS BLOB) campaign_views,
 CAST(COALESCE((SELECT json_group_array(json_object('link_id',link_id,'url',url,'campaign_id',campaign_id,'campaign_uuid',campaign_uuid,'campaign_name',campaign_name,'campaign_subject',campaign_subject,'click_count',click_count,'last_clicked_at',last_clicked_at)) FROM clicks),'[]') AS BLOB) link_clicks;
