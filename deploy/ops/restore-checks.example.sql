\set ON_ERROR_STOP on

DO $$
DECLARE
  required_table text;
BEGIN
  FOREACH required_table IN ARRAY ARRAY[
    'schema_migrations', 'users', 'accounts', 'api_keys', 'groups'
  ] LOOP
    IF to_regclass('public.' || required_table) IS NULL THEN
      RAISE EXCEPTION 'required table is missing after restore: %', required_table;
    END IF;
  END LOOP;
END
$$;

SELECT COUNT(*) AS applied_migrations FROM schema_migrations;
SELECT COUNT(*) AS users FROM users;
SELECT COUNT(*) AS accounts FROM accounts;
SELECT COUNT(*) AS api_keys FROM api_keys;
SELECT COUNT(*) AS groups FROM groups;

-- Copy this file into tmp/, add deployment-specific row-count and integrity
-- assertions, then pass that file as RESTORE_VERIFY_SQL_FILE. Never weaken the
-- required-table checks above.
