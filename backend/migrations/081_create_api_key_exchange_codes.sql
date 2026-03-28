-- Independent API key exchange codes for web-based key distribution
CREATE TABLE IF NOT EXISTS api_key_exchange_codes (
    id              BIGSERIAL PRIMARY KEY,
    code            VARCHAR(64) NOT NULL UNIQUE,
    owner_user_id   BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_by      BIGINT REFERENCES users(id) ON DELETE SET NULL,
    group_id        BIGINT REFERENCES groups(id) ON DELETE SET NULL,
    quota           DECIMAL(20, 8) NOT NULL DEFAULT 0,
    expires_in_days INT NOT NULL DEFAULT 0,
    status          VARCHAR(20) NOT NULL DEFAULT 'unused', -- unused/activated/disabled
    api_key_id      BIGINT REFERENCES api_keys(id) ON DELETE SET NULL,
    activated_at    TIMESTAMPTZ,
    activated_ip    VARCHAR(64),
    batch_no        VARCHAR(64) NOT NULL DEFAULT '',
    notes           TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_api_key_exchange_codes_status ON api_key_exchange_codes(status);
CREATE INDEX IF NOT EXISTS idx_api_key_exchange_codes_owner_user_id ON api_key_exchange_codes(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_api_key_exchange_codes_created_by ON api_key_exchange_codes(created_by);
CREATE INDEX IF NOT EXISTS idx_api_key_exchange_codes_group_id ON api_key_exchange_codes(group_id);
CREATE INDEX IF NOT EXISTS idx_api_key_exchange_codes_api_key_id ON api_key_exchange_codes(api_key_id);
CREATE INDEX IF NOT EXISTS idx_api_key_exchange_codes_batch_no ON api_key_exchange_codes(batch_no);
