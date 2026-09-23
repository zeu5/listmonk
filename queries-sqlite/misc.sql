-- name: get-dashboard-charts
SELECT data FROM mat_dashboard_charts;

-- name: get-dashboard-counts
SELECT data FROM mat_dashboard_counts;

-- name: get-settings
SELECT json_group_object(key, json(value)) AS settings
FROM (SELECT * FROM settings ORDER BY key);

-- name: update-settings
UPDATE settings AS s SET value=json_extract(c.value, '$'), updated_at=CURRENT_TIMESTAMP
FROM json_each($1) AS c WHERE s.key=c.key;

-- name: update-settings-by-key
UPDATE settings SET value=$2, updated_at=CURRENT_TIMESTAMP WHERE key=$1;

-- name: get-db-info
SELECT json_object('version', sqlite_version(),
    'size_mb', ROUND((SELECT page_count * page_size FROM pragma_page_count(), pragma_page_size()) / 1048576.0, 2)) AS info;
