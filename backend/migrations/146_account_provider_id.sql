-- Add provider_id as the module-era provider identifier while keeping legacy
-- platform for compatibility during the provider modularization rollout.

ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS provider_id TEXT;

UPDATE accounts
SET provider_id = COALESCE(NULLIF(provider_id, ''), NULLIF(extra->>'provider_id', ''), platform)
WHERE provider_id IS NULL OR provider_id = '';

CREATE INDEX IF NOT EXISTS idx_accounts_provider_id
    ON accounts(provider_id)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_accounts_provider_id_priority
    ON accounts(provider_id, priority)
    WHERE deleted_at IS NULL;
