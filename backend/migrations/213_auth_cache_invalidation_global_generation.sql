-- Protected API keys retain only one-way verifiers. PostgreSQL triggers therefore
-- cannot reconstruct the request-derived cache key that older outbox events used.
-- Normalize every trigger-driven event to a reserved global sentinel; the worker
-- advances the Redis-authoritative cache generation and publishes an L1 eviction.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

CREATE OR REPLACE FUNCTION normalize_auth_cache_invalidation_outbox_key()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    NEW.cache_key := repeat('0', 64);
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_auth_cache_invalidation_outbox_global_key
    ON auth_cache_invalidation_outbox;
CREATE TRIGGER trg_auth_cache_invalidation_outbox_global_key
BEFORE INSERT OR UPDATE OF cache_key ON auth_cache_invalidation_outbox
FOR EACH ROW EXECUTE FUNCTION normalize_auth_cache_invalidation_outbox_key();

-- Events claimed before an upgrade can still carry hashes derived from obsolete
-- plaintext or protected placeholders. Requeue them under the global boundary.
UPDATE auth_cache_invalidation_outbox
SET cache_key = repeat('0', 64),
    claimed_at = NULL,
    claimed_by = NULL,
    available_at = NOW();
