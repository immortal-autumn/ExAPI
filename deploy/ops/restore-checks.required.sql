\set ON_ERROR_STOP on

-- Repository-owned minimum restore contract. This file is always executed by
-- verify-logical-restore.sh before any deployment-specific assertions.
DO $$
DECLARE
  required_table text;
  migration_count bigint;
  active_admin_count bigint;
BEGIN
  FOREACH required_table IN ARRAY ARRAY[
    'schema_migrations', 'users', 'accounts', 'api_keys', 'groups'
  ] LOOP
    IF to_regclass('public.' || required_table) IS NULL THEN
      RAISE EXCEPTION 'required table is missing after restore: %', required_table;
    END IF;
  END LOOP;

  SELECT COUNT(*) INTO migration_count FROM schema_migrations;
  IF migration_count = 0 THEN
    RAISE EXCEPTION 'schema_migrations is empty after restore';
  END IF;

  IF EXISTS (
    SELECT 1 FROM schema_migrations
    WHERE filename IS NULL OR btrim(filename) = ''
       OR checksum IS NULL OR checksum !~ '^[0-9a-f]{64}$'
  ) THEN
    RAISE EXCEPTION 'schema_migrations contains an invalid filename or checksum';
  END IF;

  SELECT COUNT(*) INTO active_admin_count
  FROM users WHERE role = 'admin' AND status = 'active';
  IF active_admin_count = 0 THEN
    RAISE EXCEPTION 'restore contains no active administrator';
  END IF;
END
$$;

SELECT COUNT(*) AS applied_migrations FROM schema_migrations;
SELECT COUNT(*) AS active_admins FROM users WHERE role = 'admin' AND status = 'active';
SELECT COUNT(*) AS users FROM users;
SELECT COUNT(*) AS accounts FROM accounts;
SELECT COUNT(*) AS api_keys FROM api_keys;
SELECT COUNT(*) AS groups FROM groups;
