\set ON_ERROR_STOP on

-- Optional deployment-specific assertions. The repository-owned validator in
-- restore-checks.required.sql always runs first and cannot be replaced by this
-- file. Copy this example into tmp/, add assertions derived from the signed
-- pre-backup manifest, and pass it as RESTORE_VERIFY_SQL_FILE.

-- Replace these placeholders with reviewed minimums for this recovery set.
SELECT COUNT(*) >= 1 AS expected_users_present FROM users;
SELECT COUNT(*) >= 0 AS expected_accounts_checked FROM accounts;
