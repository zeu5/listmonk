-- name: get-dashboard-charts
SELECT data FROM mat_dashboard_charts;

-- name: get-dashboard-counts
SELECT data FROM mat_dashboard_counts;

-- name: get-settings
SELECT json_group_object(key, CASE
    WHEN key IN (
        'app.cache_slow_queries',
        'app.check_updates',
        'app.enable_public_archive',
        'app.enable_public_archive_rss_content',
        'app.enable_public_subscription_page',
        'app.message_sliding_window',
        'app.send_optin_confirmation',
        'app.show_optin_page',
        'bounce.enabled',
        'bounce.sendgrid_enabled',
        'bounce.ses_enabled',
        'bounce.webhooks_enabled',
        'privacy.allow_blocklist',
        'privacy.allow_export',
        'privacy.allow_preferences',
        'privacy.allow_wipe',
        'privacy.disable_tracking',
        'privacy.individual_tracking',
        'privacy.record_optin_ip',
        'privacy.unsubscribe_header'
    ) AND value IN ('0', '1') THEN json(CASE value WHEN '1' THEN 'true' ELSE 'false' END)
    WHEN json_valid(value) THEN json(value)
    ELSE json_quote(value)
END) AS settings
FROM (SELECT * FROM settings ORDER BY key);

-- name: update-settings
UPDATE settings AS s SET value=CASE c.type
    WHEN 'text' THEN json_quote(c.value)
    WHEN 'true' THEN 'true'
    WHEN 'false' THEN 'false'
    WHEN 'null' THEN 'null'
    ELSE c.value
END, updated_at=CURRENT_TIMESTAMP
FROM json_each($1) AS c WHERE s.key=c.key;

-- name: update-settings-by-key
UPDATE settings SET value=$2, updated_at=CURRENT_TIMESTAMP WHERE key=$1;

-- name: get-db-info
SELECT json_object('version', sqlite_version(),
    'size_mb', ROUND((SELECT page_count * page_size FROM pragma_page_count(), pragma_page_size()) / 1048576.0, 2)) AS info;
