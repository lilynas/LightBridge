-- Add Grok as a first-class quota platform.
--
-- Grok groups and accounts do not need a table-level platform CHECK constraint in
-- the current schema, but user_platform_quotas uses one and must be kept in sync
-- with service.AllowedQuotaPlatforms.

ALTER TABLE user_platform_quotas DROP CONSTRAINT IF EXISTS user_platform_quotas_platform_check;
ALTER TABLE user_platform_quotas ADD CONSTRAINT user_platform_quotas_platform_check
    CHECK (platform IN ('anthropic', 'openai', 'gemini', 'grok', 'antigravity', 'custom'));
