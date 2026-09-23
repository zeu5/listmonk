DROP VIEW IF EXISTS mat_dashboard_counts;
DROP VIEW IF EXISTS mat_dashboard_charts;
DROP VIEW IF EXISTS mat_list_subscriber_stats;

-- subscribers
DROP TABLE IF EXISTS subscribers;
CREATE TABLE subscribers (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid            TEXT NOT NULL UNIQUE,
    email           TEXT NOT NULL UNIQUE,
    name            TEXT NOT NULL,
    attribs         TEXT NOT NULL DEFAULT '{}',
    status          TEXT NOT NULL DEFAULT 'enabled',
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
DROP INDEX IF EXISTS idx_subs_email; CREATE UNIQUE INDEX idx_subs_email ON subscribers(LOWER(email));
DROP INDEX IF EXISTS idx_subs_status; CREATE INDEX idx_subs_status ON subscribers(status);
DROP INDEX IF EXISTS idx_subs_id_status; CREATE INDEX idx_subs_id_status ON subscribers(id, status);
DROP INDEX IF EXISTS idx_subs_created_at; CREATE INDEX idx_subs_created_at ON subscribers(created_at);
DROP INDEX IF EXISTS idx_subs_updated_at; CREATE INDEX idx_subs_updated_at ON subscribers(updated_at);
CREATE TRIGGER subscribers_after_blocklist AFTER UPDATE OF status ON subscribers
WHEN NEW.status='blocklisted'
BEGIN
    UPDATE subscriber_lists SET status='unsubscribed',updated_at=CURRENT_TIMESTAMP WHERE subscriber_id=NEW.id;
END;

-- lists
DROP TABLE IF EXISTS lists;
CREATE TABLE lists (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid            TEXT NOT NULL UNIQUE,
    name            TEXT NOT NULL,
    type            TEXT NOT NULL,
    optin           TEXT NOT NULL DEFAULT 'single',
    status          TEXT NOT NULL DEFAULT 'active',
    tags            TEXT,
    description     TEXT NOT NULL DEFAULT '',

    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
DROP INDEX IF EXISTS idx_lists_type; CREATE INDEX idx_lists_type ON lists(type);
DROP INDEX IF EXISTS idx_lists_optin; CREATE INDEX idx_lists_optin ON lists(optin);
DROP INDEX IF EXISTS idx_lists_status; CREATE INDEX idx_lists_status ON lists(status);
DROP INDEX IF EXISTS idx_lists_name; CREATE INDEX idx_lists_name ON lists(name);
DROP INDEX IF EXISTS idx_lists_created_at; CREATE INDEX idx_lists_created_at ON lists(created_at);
DROP INDEX IF EXISTS idx_lists_updated_at; CREATE INDEX idx_lists_updated_at ON lists(updated_at);


DROP TABLE IF EXISTS subscriber_lists;
CREATE TABLE subscriber_lists (
    subscriber_id      INTEGER REFERENCES subscribers(id) ON DELETE CASCADE ON UPDATE CASCADE,
    list_id            INTEGER NULL REFERENCES lists(id) ON DELETE CASCADE ON UPDATE CASCADE,
    meta               TEXT NOT NULL DEFAULT '{}',
    status             TEXT NOT NULL DEFAULT 'unconfirmed',

    created_at         TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at         TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY(subscriber_id, list_id)
);
DROP INDEX IF EXISTS idx_sub_lists_sub_id; CREATE INDEX idx_sub_lists_sub_id ON subscriber_lists(subscriber_id);
DROP INDEX IF EXISTS idx_sub_lists_list_id; CREATE INDEX idx_sub_lists_list_id ON subscriber_lists(list_id);
DROP INDEX IF EXISTS idx_sub_lists_status; CREATE INDEX idx_sub_lists_status ON subscriber_lists(status);

-- templates
DROP TABLE IF EXISTS templates;
CREATE TABLE templates (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    name            TEXT NOT NULL,
    type            TEXT NOT NULL DEFAULT 'campaign',
    subject         TEXT NOT NULL,
    body            TEXT NOT NULL,
    body_source     TEXT NULL,
    is_default      BOOLEAN NOT NULL DEFAULT 0,

    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX idx_templates_default ON templates (is_default) WHERE is_default = 1;

-- Keep campaign references valid when deleting a non-default template. SQLite
-- cannot express PostgreSQL's data-changing CTE used by the application query.
CREATE TRIGGER templates_before_delete BEFORE DELETE ON templates
WHEN OLD.is_default = 0
BEGIN
    UPDATE campaigns SET template_id=(SELECT id FROM templates WHERE is_default=1 AND type IN ('campaign', 'campaign_visual') LIMIT 1)
    WHERE template_id=OLD.id;
END;


-- campaigns
DROP TABLE IF EXISTS campaigns;
CREATE TABLE campaigns (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid             TEXT NOT NULL UNIQUE,
    name             TEXT NOT NULL,
    subject          TEXT NOT NULL,
    from_email       TEXT NOT NULL,
    body             TEXT NOT NULL,
    body_source      TEXT NULL,
    altbody          TEXT NULL,
    content_type     TEXT NOT NULL DEFAULT 'richtext',
    send_at          TIMESTAMP,
    headers          TEXT NOT NULL DEFAULT '[]',
    attribs          TEXT NOT NULL DEFAULT '{}',
    status           TEXT NOT NULL DEFAULT 'draft',
    tags             TEXT,

    -- The subscription statuses of subscribers to which a campaign will be sent.
    -- For opt-in campaigns, this will be 'unsubscribed'.
    type TEXT DEFAULT 'regular',

    -- The ID of the messenger backend used to send this campaign.
    messenger        TEXT NOT NULL,
    template_id      INTEGER REFERENCES templates(id) ON DELETE SET NULL,

    -- Progress and stats.
    to_send            INT NOT NULL DEFAULT 0,
    sent               INT NOT NULL DEFAULT 0,
    max_subscriber_id  INT NOT NULL DEFAULT 0,
    last_subscriber_id INT NOT NULL DEFAULT 0,

    -- Publishing.
    archive             BOOLEAN NOT NULL DEFAULT 0,
    archive_slug        TEXT NULL UNIQUE,
    archive_template_id INTEGER REFERENCES templates(id) ON DELETE SET NULL,
    archive_meta        TEXT NOT NULL DEFAULT '{}',

    started_at       TIMESTAMP,
    created_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
DROP INDEX IF EXISTS idx_camps_status; CREATE INDEX idx_camps_status ON campaigns(status);
DROP INDEX IF EXISTS idx_camps_name; CREATE INDEX idx_camps_name ON campaigns(name);
DROP INDEX IF EXISTS idx_camps_created_at; CREATE INDEX idx_camps_created_at ON campaigns(created_at);
DROP INDEX IF EXISTS idx_camps_updated_at; CREATE INDEX idx_camps_updated_at ON campaigns(updated_at);


DROP TABLE IF EXISTS campaign_lists;
CREATE TABLE campaign_lists (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    campaign_id  INTEGER NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE ON UPDATE CASCADE,

    -- Lists may be deleted, so list_id is nullable
    -- and a copy of the original list name is maintained here.
    list_id      INTEGER NULL REFERENCES lists(id) ON DELETE SET NULL ON UPDATE CASCADE,
    list_name    TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX idx_campaign_lists_unique ON campaign_lists (campaign_id, list_id);
CREATE TRIGGER lists_after_name_update AFTER UPDATE OF name ON lists
BEGIN
    UPDATE campaign_lists SET list_name=NEW.name WHERE list_id=NEW.id;
END;
DROP INDEX IF EXISTS idx_camp_lists_camp_id; CREATE INDEX idx_camp_lists_camp_id ON campaign_lists(campaign_id);
DROP INDEX IF EXISTS idx_camp_lists_list_id; CREATE INDEX idx_camp_lists_list_id ON campaign_lists(list_id);

DROP TABLE IF EXISTS campaign_views;
CREATE TABLE campaign_views (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    campaign_id      INTEGER NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE ON UPDATE CASCADE,

    -- Subscribers may be deleted, but the view counts should remain.
    subscriber_id    INTEGER NULL REFERENCES subscribers(id) ON DELETE SET NULL ON UPDATE CASCADE,
    created_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
DROP INDEX IF EXISTS idx_views_camp_id; CREATE INDEX idx_views_camp_id ON campaign_views(campaign_id);
DROP INDEX IF EXISTS idx_views_subscriber_id; CREATE INDEX idx_views_subscriber_id ON campaign_views(subscriber_id);
DROP INDEX IF EXISTS idx_views_date; CREATE INDEX idx_views_date ON campaign_views(created_at);

-- media
DROP TABLE IF EXISTS media;
CREATE TABLE media (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid             TEXT NOT NULL UNIQUE,
    provider         TEXT NOT NULL DEFAULT '',
    filename         TEXT NOT NULL,
    content_type     TEXT NOT NULL DEFAULT 'application/octet-stream',
    thumb            TEXT NOT NULL,
    meta             TEXT NOT NULL DEFAULT '{}',
    created_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
DROP INDEX IF EXISTS idx_media_filename; CREATE INDEX idx_media_filename ON media(provider, filename);

-- campaign_media
DROP TABLE IF EXISTS campaign_media;
CREATE TABLE campaign_media (
    campaign_id  INTEGER REFERENCES campaigns(id) ON DELETE CASCADE ON UPDATE CASCADE,

    -- Media items may be deleted, so media_id is nullable
    -- and a copy of the original name is maintained here.
    media_id     INTEGER NULL REFERENCES media(id) ON DELETE SET NULL ON UPDATE CASCADE,

    filename     TEXT NOT NULL DEFAULT ''
);
DROP INDEX IF EXISTS idx_camp_media_id; CREATE UNIQUE INDEX idx_camp_media_id ON campaign_media (campaign_id, media_id);
DROP INDEX IF EXISTS idx_camp_media_camp_id; CREATE INDEX idx_camp_media_camp_id ON campaign_media(campaign_id);


-- links
DROP TABLE IF EXISTS links;
CREATE TABLE links (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    uuid             TEXT NOT NULL UNIQUE,
    url              TEXT NOT NULL UNIQUE,
    created_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

DROP TABLE IF EXISTS link_clicks;
CREATE TABLE link_clicks (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    campaign_id      INTEGER NULL REFERENCES campaigns(id) ON DELETE CASCADE ON UPDATE CASCADE,
    link_id          INTEGER NOT NULL REFERENCES links(id) ON DELETE CASCADE ON UPDATE CASCADE,

    -- Subscribers may be deleted, but the link counts should remain.
    subscriber_id    INTEGER NULL REFERENCES subscribers(id) ON DELETE SET NULL ON UPDATE CASCADE,
    created_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
DROP INDEX IF EXISTS idx_clicks_camp_id; CREATE INDEX idx_clicks_camp_id ON link_clicks(campaign_id);
DROP INDEX IF EXISTS idx_clicks_link_id; CREATE INDEX idx_clicks_link_id ON link_clicks(link_id);
DROP INDEX IF EXISTS idx_clicks_sub_id; CREATE INDEX idx_clicks_sub_id ON link_clicks(subscriber_id);
DROP INDEX IF EXISTS idx_clicks_date; CREATE INDEX idx_clicks_date ON link_clicks(created_at);

-- settings
DROP TABLE IF EXISTS settings;
CREATE TABLE settings (
    key             TEXT NOT NULL UNIQUE,
    value           TEXT NOT NULL DEFAULT '{}',
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
DROP INDEX IF EXISTS idx_settings_key; CREATE INDEX idx_settings_key ON settings(key);
INSERT INTO settings (key, value) VALUES
    ('app.site_name', '"Mailing list"'),
    ('app.root_url', '"http://localhost:9000"'),
    ('app.favicon_url', '""'),
    ('app.from_email', '"listmonk <noreply@listmonk.yoursite.com>"'),
    ('app.logo_url', '""'),
    ('app.concurrency', '10'),
    ('app.message_rate', '10'),
    ('app.batch_size', '1000'),
    ('app.max_send_errors', '1000'),
    ('app.message_sliding_window', 'false'),
    ('app.message_sliding_window_duration', '"1h"'),
    ('app.message_sliding_window_rate', '10000'),
    ('app.cache_slow_queries', 'false'),
    ('app.cache_slow_queries_interval', '"0 3 * * *"'),
    ('app.enable_public_archive', 'true'),
    ('app.enable_public_subscription_page', 'true'),
    ('app.show_optin_page', 'true'),
    ('app.enable_public_archive_rss_content', 'true'),
    ('app.send_optin_confirmation', 'true'),
    ('app.check_updates', 'true'),
    ('app.notify_emails', '[]'),
    ('app.lang', '"en"'),
    ('privacy.individual_tracking', 'false'),
    ('privacy.disable_tracking', 'false'),
    ('privacy.unsubscribe_header', 'true'),
    ('privacy.allow_blocklist', 'true'),
    ('privacy.allow_export', 'true'),
    ('privacy.allow_wipe', 'true'),
    ('privacy.allow_preferences', 'true'),
    ('privacy.exportable', '["profile", "subscriptions", "campaign_views", "link_clicks"]'),
    ('privacy.domain_blocklist', '[]'),
    ('privacy.domain_allowlist', '[]'),
    ('privacy.record_optin_ip', 'false'),
    ('security.captcha', '{"altcha": {"enabled": false, "complexity": 300000}, "hcaptcha": {"enabled": false, "key": "", "secret": ""}}'),
    ('security.oidc', '{"enabled": false, "provider_url": "", "provider_name": "", "client_id": "", "client_secret": "", "auto_create_users": false, "default_user_role_id": null, "default_list_role_id": null}'),
    ('security.trusted_urls', '[]'),
    ('upload.provider', '"filesystem"'),
    ('upload.max_file_size', '5000'),
    ('upload.extensions', '["jpg","jpeg","png","gif","svg","*"]'),
    ('upload.filesystem.upload_path', '"uploads"'),
    ('upload.filesystem.upload_uri', '"/uploads"'),
    ('upload.s3.url', '"https://ap-south-1.s3.amazonaws.com"'),
    ('upload.s3.public_url', '""'),
    ('upload.s3.aws_access_key_id', '""'),
    ('upload.s3.aws_secret_access_key', '""'),
    ('upload.s3.aws_default_region', '"ap-south-1"'),
    ('upload.s3.bucket', '""'),
    ('upload.s3.bucket_domain', '""'),
    ('upload.s3.bucket_path', '"/"'),
    ('upload.s3.bucket_type', '"public"'),
    ('upload.s3.expiry', '"167h"'),
    ('smtp',
        '[{"enabled":true, "host":"smtp.yoursite.com","port":25,"auth_protocol":"cram","username":"username","password":"password","hello_hostname":"","max_conns":10,"idle_timeout":"15s","wait_timeout":"5s","max_msg_retries":2,"msg_retry_delay":"10ms","tls_type":"STARTTLS","tls_skip_verify":false,"email_headers":[], "from_addresses":[]},
          {"enabled":false, "host":"smtp.gmail.com","port":465,"auth_protocol":"login","username":"username@gmail.com","password":"password","hello_hostname":"","max_conns":10,"idle_timeout":"15s","wait_timeout":"5s","max_msg_retries":2,"msg_retry_delay":"10ms","tls_type":"TLS","tls_skip_verify":false,"email_headers":[], "from_addresses":[]}]'),
    ('messengers', '[]'),
    ('bounce.enabled', 'false'),
    ('bounce.webhooks_enabled', 'false'),
    ('bounce.actions', '{"soft": {"count": 2, "action": "none"}, "hard": {"count": 1, "action": "blocklist"}, "complaint" : {"count": 1, "action": "blocklist"}}'),
    ('bounce.ses_enabled', 'false'),
    ('bounce.azure', '{"enabled": false, "shared_secret": "", "shared_secret_header": ""}'),
    ('bounce.sendgrid_enabled', 'false'),
    ('bounce.sendgrid_key', '""'),
    ('bounce.postmark', '{"enabled": false, "username": "", "password": ""}'),
    ('bounce.forwardemail', '{"enabled": false, "key": ""}'),
    ('bounce.lettermint', '{"enabled": false, "key": ""}'),
    ('bounce.mailboxes',
        '[{"enabled":false, "type": "pop", "host":"pop.yoursite.com","port":995,"auth_protocol":"userpass","username":"username","password":"password","return_path": "bounce@listmonk.yoursite.com","scan_interval":"15m","tls_enabled":true,"tls_skip_verify":false}]'),
    ('appearance.admin.custom_css', '""'),
    ('appearance.admin.custom_js', '""'),
    ('appearance.public.custom_css', '""'),
    ('appearance.public.custom_js', '""'),
    ('maintenance.db', '{"vacuum": false, "vacuum_cron_interval": "0 2 * * *"}');

-- bounces
DROP TABLE IF EXISTS bounces;
CREATE TABLE bounces (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    subscriber_id    INTEGER NOT NULL REFERENCES subscribers(id) ON DELETE CASCADE ON UPDATE CASCADE,
    campaign_id      INTEGER NULL REFERENCES campaigns(id) ON DELETE SET NULL ON UPDATE CASCADE,
    type             TEXT NOT NULL DEFAULT 'hard',
    source           TEXT NOT NULL DEFAULT '',
    meta             TEXT NOT NULL DEFAULT '{}',
    created_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
DROP INDEX IF EXISTS idx_bounces_sub_id; CREATE INDEX idx_bounces_sub_id ON bounces(subscriber_id);
DROP INDEX IF EXISTS idx_bounces_camp_id; CREATE INDEX idx_bounces_camp_id ON bounces(campaign_id);
DROP INDEX IF EXISTS idx_bounces_source; CREATE INDEX idx_bounces_source ON bounces(source);
DROP INDEX IF EXISTS idx_bounces_date; CREATE INDEX idx_bounces_date ON bounces(created_at);

-- roles
DROP TABLE IF EXISTS roles;
CREATE TABLE roles (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    type             TEXT NOT NULL DEFAULT 'user',
    parent_id        INTEGER NULL REFERENCES roles(id) ON DELETE CASCADE ON UPDATE CASCADE,
    list_id          INTEGER NULL REFERENCES lists(id) ON DELETE CASCADE ON UPDATE CASCADE,
    permissions      TEXT NOT NULL DEFAULT '{}',
    name             TEXT NULL,
    created_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX idx_roles ON roles (parent_id, list_id);
CREATE UNIQUE INDEX idx_roles_name ON roles (type, name) WHERE name IS NOT NULL;

-- users
DROP TABLE IF EXISTS users;
CREATE TABLE users (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    username         TEXT NOT NULL UNIQUE,
    password_login   BOOLEAN NOT NULL DEFAULT 0,
    password         TEXT NULL,
    email            TEXT NOT NULL UNIQUE,
    name             TEXT NOT NULL,
    avatar           TEXT NULL,
    type             TEXT NOT NULL DEFAULT 'user',
    user_role_id     INTEGER NOT NULL REFERENCES roles(id) ON DELETE RESTRICT,
    list_role_id     INTEGER NULL REFERENCES roles(id) ON DELETE CASCADE,
    status           TEXT NOT NULL DEFAULT 'disabled',
    twofa_type       TEXT NOT NULL DEFAULT 'none',
    twofa_key        TEXT NULL,
    loggedin_at      TIMESTAMP NULL,
    created_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- user sessions
DROP TABLE IF EXISTS sessions;
CREATE TABLE sessions (
    id TEXT NOT NULL PRIMARY KEY,
    data TEXT DEFAULT '{}' NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
);
DROP INDEX IF EXISTS idx_sessions; CREATE INDEX idx_sessions ON sessions (id, created_at);


-- Live statistics views replace PostgreSQL materialized views on SQLite.
CREATE VIEW mat_dashboard_counts AS
WITH subs AS (SELECT COUNT(*) AS num, status FROM subscribers GROUP BY status)
SELECT CURRENT_TIMESTAMP AS updated_at,
    json_object(
        'subscribers', json_object(
            'total', COALESCE((SELECT SUM(num) FROM subs), 0),
            'blocklisted', COALESCE((SELECT num FROM subs WHERE status='blocklisted'), 0),
            'orphans', (SELECT COUNT(s.id) FROM subscribers s LEFT JOIN subscriber_lists sl ON s.id=sl.subscriber_id WHERE sl.subscriber_id IS NULL)
        ),
        'lists', json_object(
            'total', (SELECT COUNT(*) FROM lists),
            'private', (SELECT COUNT(*) FROM lists WHERE type='private'),
            'public', (SELECT COUNT(*) FROM lists WHERE type='public'),
            'optin_single', (SELECT COUNT(*) FROM lists WHERE optin='single'),
            'optin_double', (SELECT COUNT(*) FROM lists WHERE optin='double')
        ),
        'campaigns', json_object(
            'total', (SELECT COUNT(*) FROM campaigns),
            'by_status', json(COALESCE((SELECT json_group_object(status, num) FROM (SELECT status, COUNT(*) num FROM campaigns GROUP BY status)), '{}'))
        ),
        'messages', COALESCE((SELECT SUM(sent) FROM campaigns), 0)
    ) AS data;

CREATE VIEW mat_dashboard_charts AS
SELECT CURRENT_TIMESTAMP AS updated_at,
    json_object(
        'link_clicks', json(COALESCE((SELECT json_group_array(json_object('count', count, 'date', date)) FROM (SELECT COUNT(*) count, date(created_at) date FROM link_clicks WHERE created_at >= datetime('now', '-30 days') GROUP BY date(created_at) ORDER BY date)), '[]')),
        'campaign_views', json(COALESCE((SELECT json_group_array(json_object('count', count, 'date', date)) FROM (SELECT COUNT(*) count, date(created_at) date FROM campaign_views WHERE created_at >= datetime('now', '-30 days') GROUP BY date(created_at) ORDER BY date)), '[]'))
    ) AS data;

CREATE VIEW mat_list_subscriber_stats AS
SELECT CURRENT_TIMESTAMP AS updated_at, lists.id AS list_id, subscriber_lists.status, COUNT(subscriber_lists.status) AS subscriber_count
FROM lists LEFT JOIN subscriber_lists ON subscriber_lists.list_id=lists.id
GROUP BY lists.id, subscriber_lists.status
UNION ALL
SELECT CURRENT_TIMESTAMP, 0, NULL, COUNT(id) FROM subscribers;
