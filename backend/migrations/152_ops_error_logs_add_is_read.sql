-- 152_ops_error_logs_add_is_read.sql
-- Add is_read column to track read/unread status for error logs

ALTER TABLE ops_error_logs
    ADD COLUMN IF NOT EXISTS is_read BOOLEAN NOT NULL DEFAULT false;

CREATE INDEX IF NOT EXISTS idx_ops_error_logs_unread
    ON ops_error_logs (created_at DESC)
    WHERE is_read = false;

-- Mark existing errors as read (they are historical)
UPDATE ops_error_logs SET is_read = true WHERE is_read = false;
