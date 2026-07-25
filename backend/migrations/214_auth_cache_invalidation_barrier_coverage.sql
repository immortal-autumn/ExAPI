-- Ensure every mutable API-key field used by authentication snapshots creates a
-- transactionally durable barrier event. The global-key normalizer installed by
-- migration 213 prevents reusable key material or protected placeholders from
-- becoming cache identifiers.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

CREATE OR REPLACE FUNCTION enqueue_api_key_auth_cache_invalidation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        PERFORM enqueue_auth_cache_invalidation(OLD.key);
        RETURN OLD;
    END IF;

    IF OLD.key IS DISTINCT FROM NEW.key
       OR OLD.key_digest IS DISTINCT FROM NEW.key_digest
       OR OLD.status IS DISTINCT FROM NEW.status
       OR OLD.deleted_at IS DISTINCT FROM NEW.deleted_at
       OR OLD.user_id IS DISTINCT FROM NEW.user_id
       OR OLD.group_id IS DISTINCT FROM NEW.group_id
       OR OLD.ip_whitelist IS DISTINCT FROM NEW.ip_whitelist
       OR OLD.ip_blacklist IS DISTINCT FROM NEW.ip_blacklist
       OR OLD.expires_at IS DISTINCT FROM NEW.expires_at
       OR OLD.quota IS DISTINCT FROM NEW.quota
       OR OLD.rate_limit_5h IS DISTINCT FROM NEW.rate_limit_5h
       OR OLD.rate_limit_1d IS DISTINCT FROM NEW.rate_limit_1d
       OR OLD.rate_limit_7d IS DISTINCT FROM NEW.rate_limit_7d THEN
        PERFORM enqueue_auth_cache_invalidation(OLD.key);
        IF NEW.deleted_at IS NULL AND NEW.key IS DISTINCT FROM OLD.key THEN
            PERFORM enqueue_auth_cache_invalidation(NEW.key);
        END IF;
    END IF;
    RETURN NEW;
END;
$$;
