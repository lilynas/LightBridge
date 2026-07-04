-- 持久化账号可用模型目录。
-- 目录用于模型广场、测试连接、/v1/models 候选和账号级模型范围限制；
-- model_mapping 继续只表达“请求模型 -> 上游模型”的高级映射。

CREATE TABLE IF NOT EXISTS account_model_catalog (
    id            BIGSERIAL PRIMARY KEY,
    account_id    BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    model_id      VARCHAR(255) NOT NULL,
    platform      VARCHAR(50) NOT NULL DEFAULT '',
    source        VARCHAR(50) NOT NULL DEFAULT 'manual',
    display_name  VARCHAR(255) NOT NULL DEFAULT '',
    usage_modes   JSONB NOT NULL DEFAULT '[]'::jsonb,
    last_seen_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sync_batch_id VARCHAR(64),
    sync_status   VARCHAR(20) NOT NULL DEFAULT 'ok',
    sync_error    TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (account_id, model_id, source)
);

CREATE INDEX IF NOT EXISTS idx_account_model_catalog_account_id
    ON account_model_catalog(account_id);

CREATE INDEX IF NOT EXISTS idx_account_model_catalog_model_lower
    ON account_model_catalog(LOWER(model_id));

CREATE TABLE IF NOT EXISTS account_model_sync_state (
    account_id     BIGINT PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    source         VARCHAR(50) NOT NULL DEFAULT 'upstream',
    status         VARCHAR(20) NOT NULL DEFAULT 'ok',
    model_count    INT NOT NULL DEFAULT 0,
    sync_batch_id  VARCHAR(64),
    last_synced_at TIMESTAMPTZ,
    error_message  TEXT,
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 迁移旧的“自映射白名单”到模型目录；非自映射保留在 model_mapping 继续作为高级映射。
INSERT INTO account_model_catalog (
    account_id,
    model_id,
    platform,
    source,
    display_name,
    usage_modes,
    last_seen_at,
    sync_status
)
SELECT
    a.id,
    kv.key,
    COALESCE(NULLIF(a.sub_platform, ''), a.platform),
    'mapping_migration',
    kv.key,
    '["chat"]'::jsonb,
    NOW(),
    'ok'
FROM accounts a
JOIN LATERAL jsonb_each_text(a.credentials->'model_mapping') AS kv(key, value) ON TRUE
WHERE jsonb_typeof(a.credentials->'model_mapping') = 'object'
  AND kv.key = kv.value
  AND kv.key NOT LIKE '%*%'
ON CONFLICT (account_id, model_id, source) DO NOTHING;

-- 清理旧自映射，避免 model_mapping 继续被误用为白名单；高级映射保持不变。
UPDATE accounts a
SET credentials = jsonb_set(
    a.credentials,
    '{model_mapping}',
    COALESCE((
        SELECT jsonb_object_agg(kv.key, kv.value)
        FROM jsonb_each_text(a.credentials->'model_mapping') AS kv(key, value)
        WHERE kv.key <> kv.value OR kv.key LIKE '%*%'
    ), '{}'::jsonb),
    true
)
WHERE jsonb_typeof(a.credentials->'model_mapping') = 'object';
