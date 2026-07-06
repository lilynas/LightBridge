-- 153_ops_system_logs_add_api_key_id.sql
-- Add api_key_id column to ops_system_logs for API Key filtering

ALTER TABLE ops_system_logs ADD COLUMN IF NOT EXISTS api_key_id BIGINT;

CREATE INDEX IF NOT EXISTS idx_ops_system_logs_api_key_id_created_at
  ON ops_system_logs (api_key_id, created_at DESC)
  WHERE api_key_id IS NOT NULL;
