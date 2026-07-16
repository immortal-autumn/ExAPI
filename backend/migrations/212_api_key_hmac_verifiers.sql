-- Add versioned one-way verifiers and non-secret display prefixes for gateway API keys.
-- Existing plaintext rows remain temporarily for an explicit dual-read migration; the
-- application must not delete them until every row has a verified digest and rollback
-- evidence exists. New writes are switched separately by repository code.
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS key_digest VARCHAR(256),
    ADD COLUMN IF NOT EXISTS key_prefix VARCHAR(16) NOT NULL DEFAULT '';

CREATE UNIQUE INDEX IF NOT EXISTS idx_api_keys_key_digest
    ON api_keys(key_digest)
    WHERE key_digest IS NOT NULL;

ALTER TABLE deleted_api_key_audits
    ADD COLUMN IF NOT EXISTS key_digest VARCHAR(256);

CREATE INDEX IF NOT EXISTS idx_deleted_api_key_audits_key_digest
    ON deleted_api_key_audits(key_digest)
    WHERE key_digest IS NOT NULL;

COMMENT ON COLUMN deleted_api_key_audits.key_digest IS
    'Versioned purpose-bound HMAC verifier used to attribute a submitted deleted key without retaining reusable key material';

COMMENT ON COLUMN api_keys.key_digest IS
    'Versioned purpose-bound HMAC verifier for gateway authentication; never reusable as a gateway key';
COMMENT ON COLUMN api_keys.key_prefix IS
    'Non-secret display prefix used for operator identification';
