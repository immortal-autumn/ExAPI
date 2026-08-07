-- Additive bridge storage for secrets. The bridge release dual-reads legacy
-- plaintext but writes only authenticated envelopes/digests.
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

ALTER TABLE proxies
    ADD COLUMN IF NOT EXISTS password_encrypted TEXT;

ALTER TABLE payment_provider_instances
    ADD COLUMN IF NOT EXISTS config_encrypted TEXT;

CREATE TABLE IF NOT EXISTS protected_settings (
    id BIGSERIAL PRIMARY KEY,
    key VARCHAR(100) NOT NULL UNIQUE,
    envelope TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON COLUMN proxies.password_encrypted IS
    'Purpose-bound SUB2API_DATA_ENCRYPTION envelope; preferred over legacy password.';
COMMENT ON COLUMN payment_provider_instances.config_encrypted IS
    'Purpose-bound SUB2API_DATA_ENCRYPTION envelope; preferred over legacy config.';
COMMENT ON TABLE protected_settings IS
    'Explicitly classified encrypted settings and keyed digests; never plaintext.';
