// Package privatecutover contains the explicit, offline-only SaaS removal
// transaction. The running server never calls this package.
package privatecutover

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lib/pq"
)

const (
	ConfirmationPrefix   = "DROP-SAAS-DATA-KEEP-USER-"
	privateSchemaVersion = 1
	advisoryLockKey      = "exapi:migrate-private-only:v1"
)

// CutoverOptions controls the destructive, offline-only migration. A backup
// target must be supplied unless AssertNoManagedBackups is explicitly set.
// The assertion is intentionally separate from an empty path so an omitted
// environment variable can never silently skip backup handling.
type CutoverOptions struct {
	BackupDir              string
	AssertNoManagedBackups bool
	// ReportPath is the production destination for an atomic, durable 0600
	// report. Output is retained only for compatibility with Run and tests.
	ReportPath string
	Output     io.Writer
}

// ManagedBackupRecordsPreserved is reported when the application still has
// S3 backup metadata in settings. The cutover deliberately does not delete
// those objects: the local backup directory is not an S3 object-store
// namespace, and the offline recovery set must remain untouched.
const managedBackupRecordsSettingKey = "backup_records"

// SaaSTables is intentionally an audited allowlist. CASCADE handles legacy
// join tables whose only purpose was the removed customer/commercial model.
var SaaSTables = []string{
	"payment_audit_logs",
	"payment_orders",
	"payment_provider_instances",
	"subscription_plans",
	"user_subscriptions",
	"promo_code_usages",
	"promo_codes",
	"redeem_codes",
	"user_affiliate_ledger",
	"user_affiliates",
	"announcement_reads",
	"announcements",
	"user_attribute_values",
	"user_attribute_definitions",
	"user_platform_quotas",
	"pending_auth_sessions",
	"auth_identity_channels",
	"auth_identities",
	"passkey_credentials",
	"passkey_user_handles",
	"security_secrets",
}

// SaaSSettingKeys and SaaSSettingPrefixes are audited erasure allowlists used
// for both plaintext and protected setting stores. Operational mail, backup,
// gateway, scheduler, monitoring, and upstream OAuth-account settings remain.
var SaaSSettingKeys = []string{
	"registration_enabled",
	"registration_email_suffix_whitelist",
	"promo_code_enabled",
	"invitation_code_enabled",
	"password_reset_enabled",
	"email_verify_enabled",
	"login_agreement_enabled",
	"login_agreement_mode",
	"login_agreement_updated_at",
	"login_agreement_revision",
	"login_agreement_documents",
	"totp_enabled",
	"passkey_enabled",
	"session_binding_enabled",
	"step_up_enabled",
	"default_balance",
	"default_concurrency",
	"default_subscriptions",
	"default_user_rpm_limit",
	"default_platform_quotas",
	"force_email_on_third_party_signup",
	"balance_low_notify_enabled",
	"balance_low_notify_threshold",
	"balance_low_notify_recharge_url",
	"subscription_expiry_notify_enabled",
	"purchase_subscription_enabled",
	"purchase_subscription_url",
	"available_channels_enabled",
	"model_plaza_enabled",
	"model_plaza_require_auth",
	"model_plaza_description",
	"custom_menu_items",
	"affiliate_enabled",
	"affiliate_admin_recharge_enabled",
	"admin_api_key",
}

var SaaSSettingPrefixes = []string{
	"affiliate_rebate_",
	"auth_source_default_",
	"payment_",
	"linuxdo_connect_",
	"dingtalk_connect_",
	"wechat_connect_",
	"oidc_connect_",
	"github_oauth_",
	"google_oauth_",
	"turnstile_",
	"tencent_captcha_",
	"aliyun_captcha_",
}

type MigrationReport struct {
	SchemaVersion           int       `json:"private_schema_version"`
	OperatorID              int64     `json:"operator_id"`
	CutoverAt               time.Time `json:"cutover_at"`
	DeletedUsers            int64     `json:"deleted_users"`
	DroppedTables           []string  `json:"dropped_tables"`
	PurgedBackups           []string  `json:"purged_backups"`
	ManagedBackupsPreserved int       `json:"managed_backup_records_preserved"`
	PurgedSettings          []string  `json:"purged_settings"`
	PurgedProtected         []string  `json:"purged_protected_settings"`
	Confirmation            string    `json:"confirmation"`
	ReportSHA256            string    `json:"report_sha256"`
	ReportHMACSHA256        string    `json:"report_hmac_sha256"`
}

func ExpectedConfirmation(operatorID int64) string {
	return ConfirmationPrefix + strconv.FormatInt(operatorID, 10)
}

func ParseConfirmation(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(value, ConfirmationPrefix) {
		return 0, fmt.Errorf("confirmation must equal %s<operator-id>", ConfirmationPrefix)
	}
	operatorID, err := strconv.ParseInt(strings.TrimPrefix(value, ConfirmationPrefix), 10, 64)
	if err != nil || operatorID <= 0 {
		return 0, fmt.Errorf("confirmation contains an invalid operator id")
	}
	return operatorID, nil
}

func SignReport(report MigrationReport, key []byte) ([]byte, error) {
	if len(key) < 32 {
		return nil, errors.New("report signing key must be at least 32 bytes")
	}
	report.ReportSHA256 = ""
	report.ReportHMACSHA256 = ""
	unsigned, err := json.Marshal(report)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(unsigned)
	report.ReportSHA256 = hex.EncodeToString(digest[:])
	signer := hmac.New(sha256.New, key)
	_, _ = signer.Write(unsigned)
	report.ReportHMACSHA256 = hex.EncodeToString(signer.Sum(nil))
	return json.Marshal(report)
}

// Run executes the cutover in a serializable transaction and only then purges
// pre-cutover backup files. The caller must supply the exact confirmation, a
// 32-byte-or-longer report key, and an explicit backup directory. No caller in
// the online server invokes it.
func Run(ctx context.Context, db *sql.DB, confirmation string, reportKey []byte, backupDir string, now func() time.Time, output io.Writer) (MigrationReport, error) {
	return RunWithOptions(ctx, db, confirmation, reportKey, CutoverOptions{
		BackupDir: backupDir,
		Output:    output,
	}, now)
}

// RunWithOptions is the options-based entry point used by the offline command.
// AssertNoManagedBackups is an explicit operator assertion for installations
// that never configured application-managed backups; it is not inferred from
// an empty path.
func RunWithOptions(ctx context.Context, db *sql.DB, confirmation string, reportKey []byte, options CutoverOptions, now func() time.Time) (MigrationReport, error) {
	if db == nil {
		return MigrationReport{}, errors.New("database is required")
	}
	if err := validateBackupOptions(options); err != nil {
		return MigrationReport{}, err
	}
	if err := validateReportOptions(options); err != nil {
		return MigrationReport{}, err
	}
	operatorID, err := ParseConfirmation(confirmation)
	if err != nil {
		return MigrationReport{}, err
	}
	if len(reportKey) < 32 {
		return MigrationReport{}, errors.New("EXAPI_MIGRATION_REPORT_KEY must be at least 32 bytes")
	}
	if now == nil {
		now = time.Now
	}
	cutoverAt := now().UTC()

	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return MigrationReport{}, fmt.Errorf("begin serializable cutover: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, advisoryLockKey); err != nil {
		return MigrationReport{}, fmt.Errorf("acquire cutover advisory lock: %w", err)
	}
	var activeOperatorID int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM users WHERE role = 'admin' AND status = 'active' AND deleted_at IS NULL ORDER BY id ASC LIMIT 1 FOR UPDATE`).Scan(&activeOperatorID)
	if err != nil {
		return MigrationReport{}, fmt.Errorf("select lowest-id active admin: %w", err)
	}
	if activeOperatorID != operatorID {
		return MigrationReport{}, fmt.Errorf("confirmation is for operator %d, but lowest-id active admin is %d", operatorID, activeOperatorID)
	}
	managedBackupRecordsPreserved, err := managedBackupRecordCount(ctx, tx)
	if err != nil {
		return MigrationReport{}, fmt.Errorf("inspect application-managed backups: %w", err)
	}
	if options.AssertNoManagedBackups && managedBackupRecordsPreserved != 0 {
		return MigrationReport{}, fmt.Errorf("managed backup metadata contains %d record(s); cannot assert no managed backups", managedBackupRecordsPreserved)
	}
	purgedSettings, err := purgeSettingRows(ctx, tx, "settings")
	if err != nil {
		return MigrationReport{}, fmt.Errorf("purge SaaS settings: %w", err)
	}
	purgedProtected, err := purgeSettingRows(ctx, tx, "protected_settings")
	if err != nil {
		return MigrationReport{}, fmt.Errorf("purge protected SaaS settings: %w", err)
	}

	for _, table := range SaaSTables {
		if _, err := tx.ExecContext(ctx, `DROP TABLE IF EXISTS `+quoteIdentifier(table)+` CASCADE`); err != nil {
			return MigrationReport{}, fmt.Errorf("drop SaaS table %s: %w", table, err)
		}
	}
	var deletedUsers int64
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE id <> $1`, operatorID).Scan(&deletedUsers); err != nil {
		return MigrationReport{}, fmt.Errorf("count customer users: %w", err)
	}
	// usage_cleanup_tasks.created_by has an ON DELETE RESTRICT foreign key.
	// Reassign those retained operational tasks before deleting users so an
	// otherwise unrelated historical cleanup task cannot abort the cutover.
	if err := reassignUsageCleanupTasks(ctx, tx, operatorID); err != nil {
		return MigrationReport{}, fmt.Errorf("reassign usage cleanup tasks: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM users WHERE id <> $1`, operatorID); err != nil {
		return MigrationReport{}, fmt.Errorf("delete customer users: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET role = 'admin', status = 'active', deleted_at = NULL, password_hash = '!', balance = 0, frozen_balance = 0, totp_secret_encrypted = NULL, totp_enabled = false, totp_enabled_at = NULL, signup_source = 'email', last_login_at = NULL, balance_notify_enabled = false, balance_notify_threshold = NULL, balance_notify_extra_emails = '[]', total_recharged = 0 WHERE id = $1`, operatorID); err != nil {
		return MigrationReport{}, fmt.Errorf("retain operator: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE groups SET subscription_type = 'standard', daily_limit_usd = NULL, weekly_limit_usd = NULL, monthly_limit_usd = NULL WHERE subscription_type <> 'standard' OR daily_limit_usd IS NOT NULL OR weekly_limit_usd IS NOT NULL OR monthly_limit_usd IS NOT NULL`); err != nil {
		return MigrationReport{}, fmt.Errorf("normalize operational groups: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS exapi_single_active_operator_idx ON users (role) WHERE role = 'admin' AND status = 'active' AND deleted_at IS NULL`); err != nil {
		return MigrationReport{}, fmt.Errorf("enforce singleton operator: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS exapi_private_state (id boolean PRIMARY KEY DEFAULT true CHECK (id), private_schema_version integer NOT NULL, operator_id bigint NOT NULL REFERENCES users(id), cutover_at timestamptz NOT NULL, report_sha256 text NOT NULL DEFAULT '')`); err != nil {
		return MigrationReport{}, fmt.Errorf("create private state: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO exapi_private_state (id, private_schema_version, operator_id, cutover_at) VALUES (true, $1, $2, $3) ON CONFLICT (id) DO UPDATE SET private_schema_version = EXCLUDED.private_schema_version, operator_id = EXCLUDED.operator_id, cutover_at = EXCLUDED.cutover_at`, privateSchemaVersion, operatorID, cutoverAt); err != nil {
		return MigrationReport{}, fmt.Errorf("record private schema version: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return MigrationReport{}, fmt.Errorf("commit private cutover: %w", err)
	}
	committed = true

	purged, err := purgePreCutoverBackups(options.BackupDir, cutoverAt)
	if err != nil {
		return MigrationReport{}, err
	}
	report := MigrationReport{
		SchemaVersion:           privateSchemaVersion,
		OperatorID:              operatorID,
		CutoverAt:               cutoverAt,
		DeletedUsers:            deletedUsers,
		DroppedTables:           append([]string(nil), SaaSTables...),
		PurgedBackups:           purged,
		ManagedBackupsPreserved: managedBackupRecordsPreserved,
		PurgedSettings:          purgedSettings,
		PurgedProtected:         purgedProtected,
		Confirmation:            confirmation,
	}
	signed, err := SignReport(report, reportKey)
	if err != nil {
		return MigrationReport{}, err
	}
	if err := json.Unmarshal(signed, &report); err != nil {
		return MigrationReport{}, err
	}
	// Install the signed report durably before recording its digest. Readiness
	// treats a nonempty digest as proof that the report exists, so persisting the
	// marker first would allow a report write failure to produce a false-ready
	// runtime. The report writer is intentionally required and atomic.
	if err := writeDurableReport(options.ReportPath, options.Output, signed); err != nil {
		return MigrationReport{}, err
	}
	if err := recordReportDigest(ctx, db, report.ReportSHA256); err != nil {
		return MigrationReport{}, err
	}
	return report, nil
}

func validateBackupOptions(options CutoverOptions) error {
	directory := strings.TrimSpace(options.BackupDir)
	if directory == "" && !options.AssertNoManagedBackups {
		return errors.New("an explicit --backup-dir is required; use --no-managed-backups only after verifying no application-managed backups exist")
	}
	if directory != "" {
		info, err := os.Stat(directory)
		if err != nil {
			return fmt.Errorf("validate backup directory: %w", err)
		}
		if !info.IsDir() {
			return fmt.Errorf("validate backup directory: %s is not a directory", directory)
		}
	}
	return nil
}

func validateReportOptions(options CutoverOptions) error {
	path := strings.TrimSpace(options.ReportPath)
	if path == "" {
		if options.Output == nil {
			return errors.New("migration report path is required")
		}
		return nil
	}
	directory := filepath.Dir(filepath.Clean(path))
	info, err := os.Stat(directory)
	if err != nil {
		return fmt.Errorf("validate migration report directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("validate migration report directory: %s is not a directory", directory)
	}
	return nil
}

func managedBackupRecordCount(ctx context.Context, tx *sql.Tx) (int, error) {
	var raw string
	err := tx.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = $1`, managedBackupRecordsSettingKey).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if strings.TrimSpace(raw) == "" {
		return 0, nil
	}
	var records []json.RawMessage
	if err := json.Unmarshal([]byte(raw), &records); err != nil {
		return 0, fmt.Errorf("backup records JSON is corrupt: %w", err)
	}
	return len(records), nil
}

func reassignUsageCleanupTasks(ctx context.Context, tx *sql.Tx, operatorID int64) error {
	_, err := tx.ExecContext(ctx, `UPDATE usage_cleanup_tasks SET created_by = $1 WHERE created_by <> $1`, operatorID)
	return err
}

func writeDurableReport(path string, fallback io.Writer, report []byte) error {
	path = strings.TrimSpace(path)
	if path == "" {
		if fallback == nil {
			return errors.New("migration report output is required")
		}
		if _, err := fallback.Write(report); err != nil {
			return fmt.Errorf("write migration report: %w", err)
		}
		return nil
	}

	cleanPath := filepath.Clean(path)
	directory := filepath.Dir(cleanPath)
	temporary, err := os.CreateTemp(directory, "."+filepath.Base(cleanPath)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary migration report: %w", err)
	}
	temporaryPath := temporary.Name()
	installed := false
	defer func() {
		_ = temporary.Close()
		if !installed {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return fmt.Errorf("protect temporary migration report: %w", err)
	}
	if _, err := temporary.Write(report); err != nil {
		return fmt.Errorf("write migration report: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync migration report: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close migration report: %w", err)
	}
	if err := os.Rename(temporaryPath, cleanPath); err != nil {
		return fmt.Errorf("install migration report: %w", err)
	}
	installed = true

	directoryHandle, err := os.Open(directory)
	if err != nil {
		return fmt.Errorf("open migration report directory: %w", err)
	}
	defer directoryHandle.Close()
	if err := directoryHandle.Sync(); err != nil {
		return fmt.Errorf("sync migration report directory: %w", err)
	}
	return nil
}

func purgeSettingRows(ctx context.Context, tx *sql.Tx, table string) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `DELETE FROM `+quoteIdentifier(table)+` WHERE key = ANY($1) OR EXISTS (SELECT 1 FROM unnest($2::text[]) AS prefix WHERE key LIKE prefix || '%') RETURNING key`, pq.Array(SaaSSettingKeys), pq.Array(SaaSSettingPrefixes))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Strings(keys)
	return keys, nil
}

// VerifyRuntimeState fails closed until the explicit offline cutover has
// completed, its signed report digest is durable, and exactly one active
// operator is the only retained user.
func VerifyRuntimeState(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return errors.New("database is required")
	}
	var valid bool
	err := db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM exapi_private_state state
			WHERE state.id = true
			  AND state.private_schema_version = $1
			  AND btrim(state.report_sha256) <> ''
			  AND (SELECT COUNT(*) FROM users) = 1
			  AND (SELECT COUNT(*) FROM users WHERE role = 'admin' AND status = 'active' AND deleted_at IS NULL) = 1
			  AND EXISTS (
				SELECT 1 FROM users operator
				WHERE operator.id = state.operator_id
				  AND operator.role = 'admin'
				  AND operator.status = 'active'
				  AND operator.deleted_at IS NULL
			  )
		)
	`, privateSchemaVersion).Scan(&valid)
	if err != nil {
		return fmt.Errorf("query private runtime state: %w", err)
	}
	if !valid {
		return errors.New("private runtime state is incomplete; run migrate-private-only offline and verify its signed report")
	}
	return nil
}

func recordReportDigest(ctx context.Context, db *sql.DB, digest string) error {
	if strings.TrimSpace(digest) == "" {
		return errors.New("migration report digest is required")
	}
	result, err := db.ExecContext(ctx, `UPDATE exapi_private_state SET report_sha256 = $1 WHERE id = true`, digest)
	if err != nil {
		return fmt.Errorf("record migration report digest: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("verify migration report digest persistence: %w", err)
	}
	if updated != 1 {
		return fmt.Errorf("record migration report digest: expected one private state row, updated %d", updated)
	}
	return nil
}

func quoteIdentifier(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

func purgePreCutoverBackups(directory string, cutoverAt time.Time) ([]string, error) {
	directory = strings.TrimSpace(directory)
	if directory == "" {
		return nil, nil
	}
	var purged []string
	err := filepath.WalkDir(directory, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.ModTime().Before(cutoverAt) {
			if err := os.Remove(path); err != nil {
				return err
			}
			purged = append(purged, path)
		}
		return nil
	})
	return purged, err
}
