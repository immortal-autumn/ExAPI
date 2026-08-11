ALTER TABLE batch_image_jobs
    ADD COLUMN IF NOT EXISTS api_key_quota_tracked BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS api_key_rate_limit_tracked BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS account_type_snapshot VARCHAR(32) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS account_quota_tracked BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE batch_image_jobs AS job
SET api_key_quota_tracked = COALESCE(key.quota, 0) > 0,
    api_key_rate_limit_tracked = COALESCE(key.rate_limit_5h, 0) > 0
        OR COALESCE(key.rate_limit_1d, 0) > 0
        OR COALESCE(key.rate_limit_7d, 0) > 0
FROM api_keys AS key
WHERE job.api_key_id = key.id;

UPDATE batch_image_jobs AS job
SET account_type_snapshot = account.type,
    account_quota_tracked = job.pricing_snapshot_version >= 1
        AND account.type IN ('apikey', 'bedrock')
        AND (
            CASE
                WHEN jsonb_typeof(account.extra->'quota_limit') = 'number'
                    THEN (account.extra->>'quota_limit')::numeric > 0
                WHEN jsonb_typeof(account.extra->'quota_limit') = 'string'
                    AND btrim(account.extra->>'quota_limit') ~ '^[+]?(?:[0-9]+([.][0-9]*)?|[.][0-9]+)([eE][+-]?[0-9]+)?$'
                    THEN btrim(account.extra->>'quota_limit')::numeric > 0
                ELSE FALSE
            END
            OR CASE
                WHEN jsonb_typeof(account.extra->'quota_daily_limit') = 'number'
                    THEN (account.extra->>'quota_daily_limit')::numeric > 0
                WHEN jsonb_typeof(account.extra->'quota_daily_limit') = 'string'
                    AND btrim(account.extra->>'quota_daily_limit') ~ '^[+]?(?:[0-9]+([.][0-9]*)?|[.][0-9]+)([eE][+-]?[0-9]+)?$'
                    THEN btrim(account.extra->>'quota_daily_limit')::numeric > 0
                ELSE FALSE
            END
            OR CASE
                WHEN jsonb_typeof(account.extra->'quota_weekly_limit') = 'number'
                    THEN (account.extra->>'quota_weekly_limit')::numeric > 0
                WHEN jsonb_typeof(account.extra->'quota_weekly_limit') = 'string'
                    AND btrim(account.extra->>'quota_weekly_limit') ~ '^[+]?(?:[0-9]+([.][0-9]*)?|[.][0-9]+)([eE][+-]?[0-9]+)?$'
                    THEN btrim(account.extra->>'quota_weekly_limit')::numeric > 0
                ELSE FALSE
            END
        )
FROM accounts AS account
WHERE job.account_id = account.id;

COMMENT ON COLUMN batch_image_jobs.api_key_quota_tracked IS
    '提交时 API Key 是否启用总额度累计';
COMMENT ON COLUMN batch_image_jobs.api_key_rate_limit_tracked IS
    '提交时 API Key 是否启用 5h/1d/7d 额度窗口累计';
COMMENT ON COLUMN batch_image_jobs.account_type_snapshot IS
    '提交时上游账号类型快照；结算不依赖可删除的账号行';
COMMENT ON COLUMN batch_image_jobs.account_quota_tracked IS
    '提交时上游账号是否要求累计账号额度';
