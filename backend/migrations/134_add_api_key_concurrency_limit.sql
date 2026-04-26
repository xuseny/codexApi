-- Add per-API-key concurrency limit.
-- 0 = unlimited; default 5 keeps existing and newly created keys bounded.

ALTER TABLE api_keys
ADD COLUMN IF NOT EXISTS concurrency_limit INTEGER NOT NULL DEFAULT 5;

COMMENT ON COLUMN api_keys.concurrency_limit IS 'Maximum concurrent requests for this API key (0 = unlimited)';
