// Package privatecutover contains the explicit, offline-only SaaS removal
// transaction. The running server never calls this package.
package privatecutover

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lib/pq"
)

const (
	ConfirmationPrefix   = "DROP-SAAS-DATA-KEEP-USER-"
	privateSchemaVersion = 2
	// Lock identities are intentionally stable across cutover schema revisions
	// so older and newer offline binaries cannot run destructive work together.
	advisoryLockKey          = "exapi:migrate-private-only:v1"
	commandAdvisoryLockKey   = "exapi:migrate-private-only:command:v1"
	commandLockReleaseLimit  = 5 * time.Second
	maxRetainedV1ReportBytes = 1 << 20
	privateStateUpsertSQL    = `INSERT INTO exapi_private_state (id, private_schema_version, operator_id, cutover_at, cutover_evidence)
		VALUES (true, $1, $2, $3, $4)
		ON CONFLICT (id) DO NOTHING`
	privateStateSchemaSQL = `CREATE TABLE IF NOT EXISTS exapi_private_state (
		id boolean PRIMARY KEY DEFAULT true CHECK (id),
		private_schema_version integer NOT NULL,
		operator_id bigint NOT NULL REFERENCES users(id),
		cutover_at timestamptz NOT NULL,
		report_sha256 text NOT NULL DEFAULT '',
		cutover_evidence jsonb NOT NULL DEFAULT '{}'::jsonb
	)`
	privateStateEvidenceColumnSQL = `ALTER TABLE exapi_private_state
		ADD COLUMN IF NOT EXISTS cutover_evidence jsonb NOT NULL DEFAULT '{}'::jsonb`
)

// CutoverOptions controls the destructive, offline-only migration. Legacy
// local backup files require an explicit directory or an explicit assertion
// that no such directory exists. S3 backup_records are independent: they are
// reported and preserved, never treated as local filesystem paths.
type CutoverOptions struct {
	LocalBackupDir       string
	AssertNoLocalBackups bool
	BatchCleanupEvidence BatchCleanupEvidence
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
	SchemaVersion      int       `json:"private_schema_version"`
	OperatorID         int64     `json:"operator_id"`
	CutoverAt          time.Time `json:"cutover_at"`
	DeletedUsers       int64     `json:"deleted_users"`
	DroppedTables      []string  `json:"dropped_tables"`
	PurgedLocalBackups []string  `json:"purged_local_backups"`
	// LocalBackupManifestSHA256 binds the canonical backup root and the full
	// pre-cutover identity of every deletion candidate into the signed report.
	// omitempty preserves verification of reports produced before this field
	// was introduced under the v2 report schema.
	LocalBackupManifestSHA256 string               `json:"local_backup_manifest_sha256,omitempty"`
	UpgradedFromSchemaVersion int                  `json:"upgraded_from_private_schema_version,omitempty"`
	UpgradedAt                *time.Time           `json:"upgraded_at,omitempty"`
	BatchCleanupEvidence      BatchCleanupEvidence `json:"batch_cleanup_evidence"`
	ManagedBackupsPreserved   int                  `json:"managed_backup_records_preserved"`
	PurgedSettings            []string             `json:"purged_settings"`
	PurgedProtected           []string             `json:"purged_protected_settings"`
	Confirmation              string               `json:"confirmation"`
	ReportSHA256              string               `json:"report_sha256"`
	ReportHMACSHA256          string               `json:"report_hmac_sha256"`
}

// cutoverEvidence is written in the same transaction as the destructive
// database changes. It is the immutable source for every retry after that
// transaction commits; retrying must never recount already-deleted rows.
type cutoverEvidence struct {
	Report                         MigrationReport           `json:"report"`
	ReportKeySHA256                string                    `json:"report_key_sha256"`
	LocalBackupCandidates          []string                  `json:"local_backup_candidates"`
	LocalBackupCandidateIdentities []backupCandidateIdentity `json:"local_backup_candidate_identities,omitempty"`
	// LocalBackupDeletionIntents is a write-ahead deletion journal. An absent
	// candidate is accepted only when its intent was committed before unlink.
	LocalBackupDeletionIntents []string            `json:"local_backup_deletion_intents,omitempty"`
	LocalBackupRoot            *backupRootIdentity `json:"local_backup_root,omitempty"`
}

type backupRootIdentity struct {
	Path   string `json:"path"`
	Device uint64 `json:"device"`
	Inode  uint64 `json:"inode"`
}

type backupCandidateIdentity struct {
	Path          string `json:"path"`
	RelativePath  string `json:"relative_path"`
	Device        uint64 `json:"device"`
	Inode         uint64 `json:"inode"`
	Size          int64  `json:"size"`
	MTimeUnixNano int64  `json:"mtime_unix_nano"`
	Mode          uint32 `json:"mode"`
	LinkCount     uint64 `json:"link_count"`
	ContentSHA256 string `json:"content_sha256"`
}

type backupManifest struct {
	Root       *backupRootIdentity       `json:"root,omitempty"`
	Candidates []backupCandidateIdentity `json:"candidates"`
}

func (e cutoverEvidence) empty() bool {
	return e.Report.SchemaVersion == 0 && e.Report.OperatorID == 0 && len(e.LocalBackupCandidates) == 0
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
	unsigned, err := unsignedReportPayload(report)
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

// ParseReportKey accepts the printable 64-hex-character key used by production
// or a backward-compatible raw key of at least 32 bytes. Callers must never log
// the returned key or the input value.
func ParseReportKey(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("EXAPI_MIGRATION_REPORT_KEY is required and must be a 32-byte key or hex encoding")
	}
	if decoded, err := hex.DecodeString(raw); err == nil && len(decoded) >= 32 {
		return decoded, nil
	}
	if len([]byte(raw)) >= 32 {
		return []byte(raw), nil
	}
	digest := sha256.Sum256([]byte(raw))
	return nil, fmt.Errorf("EXAPI_MIGRATION_REPORT_KEY must be at least 32 bytes (sha256 fingerprint=%s)", hex.EncodeToString(digest[:])[:16])
}

// Run executes the cutover in a serializable transaction and only then purges
// pre-cutover backup files. The caller must supply the exact confirmation, a
// 32-byte-or-longer report key, and an explicit backup directory. No caller in
// the online server invokes it.
func Run(ctx context.Context, db *sql.DB, confirmation string, reportKey []byte, localBackupDir string, now func() time.Time, output io.Writer) (MigrationReport, error) {
	return RunWithOptions(ctx, db, confirmation, reportKey, CutoverOptions{
		LocalBackupDir: localBackupDir,
		Output:         output,
	}, now)
}

// RunWithOptions is the options-based entry point used by the offline command.
// AssertNoLocalBackups is an explicit operator assertion for installations
// that never used legacy local backup files; it is not inferred from an empty
// path or from the S3 backup_records setting.
func RunWithOptions(ctx context.Context, db *sql.DB, confirmation string, reportKey []byte, options CutoverOptions, now func() time.Time) (MigrationReport, error) {
	if db == nil {
		return MigrationReport{}, errors.New("database is required")
	}
	if err := validateBackupChoice(options); err != nil {
		return MigrationReport{}, err
	}
	if err := validateReportOptions(options); err != nil {
		return MigrationReport{}, err
	}
	if err := validateReportOutsideLocalBackupRoot(options); err != nil {
		return MigrationReport{}, err
	}
	if err := validateBatchCleanupEvidence(options.BatchCleanupEvidence, time.Time{}); err != nil {
		return MigrationReport{}, err
	}
	operatorID, err := ParseConfirmation(confirmation)
	if err != nil {
		return MigrationReport{}, err
	}
	confirmation = ExpectedConfirmation(operatorID)
	if len(reportKey) < 32 {
		return MigrationReport{}, errors.New("EXAPI_MIGRATION_REPORT_KEY must be at least 32 bytes")
	}
	if now == nil {
		now = time.Now
	}
	return withCommandLock(ctx, db, func(commandConn *sql.Conn) (MigrationReport, error) {
		cutoverAt := now().UTC().Truncate(time.Microsecond)
		return runWithOptionsLocked(ctx, commandConn, confirmation, reportKey, options, operatorID, cutoverAt, nil)
	})
}

func runWithOptionsLocked(ctx context.Context, commandConn *sql.Conn, confirmation string, reportKey []byte, options CutoverOptions, operatorID int64, cutoverAt time.Time, localBackupCandidates []backupCandidateIdentity) (MigrationReport, error) {
	if err := validateBatchCleanupEvidence(options.BatchCleanupEvidence, cutoverAt); err != nil {
		return MigrationReport{}, err
	}
	if err := ensurePrivateStateSchema(ctx, commandConn); err != nil {
		return MigrationReport{}, err
	}
	state, committedEvidence, exists, err := loadPrivateCutoverState(ctx, commandConn)
	if err != nil {
		return MigrationReport{}, err
	}
	if exists {
		if committedEvidence.empty() {
			if state.SchemaVersion == 1 && emptyCutoverEvidenceJSON(state.EvidenceJSON) {
				return upgradePrivateStateV1(ctx, commandConn, confirmation, reportKey, options, operatorID, cutoverAt, state)
			}
			return MigrationReport{}, errors.New("committed private cutover has no preserved evidence; refusing to recompute destructive-operation counts")
		}
		if state.SchemaVersion != privateSchemaVersion || state.OperatorID != committedEvidence.Report.OperatorID ||
			!state.CutoverAt.UTC().Equal(committedEvidence.Report.CutoverAt.UTC()) {
			return MigrationReport{}, errors.New("committed private cutover evidence does not match durable state metadata")
		}
		if err := validateCommittedEvidence(committedEvidence, operatorID, confirmation); err != nil {
			return MigrationReport{}, err
		}
		if !hmac.Equal([]byte(committedEvidence.ReportKeySHA256), []byte(reportKeySHA256(reportKey))) {
			return MigrationReport{}, errors.New("migration report key does not match the committed cutover evidence")
		}
		if err := validateBackupRootChoice(committedEvidence.LocalBackupRoot, options); err != nil {
			return MigrationReport{}, err
		}
		return finalizeCommittedCutover(ctx, commandConn, reportKey, options, committedEvidence)
	}
	var localBackupRoot *backupRootIdentity
	if localBackupCandidates == nil {
		if err := validateBackupOptions(options); err != nil {
			return MigrationReport{}, err
		}
		if strings.TrimSpace(options.LocalBackupDir) != "" {
			identity, candidates, identityErr := snapshotPreCutoverBackups(options.LocalBackupDir, cutoverAt)
			if identityErr != nil {
				return MigrationReport{}, fmt.Errorf("snapshot legacy local backups: %w", identityErr)
			}
			localBackupRoot = &identity
			localBackupCandidates = candidates
		}
	}
	localBackupCandidatePaths := backupCandidatePaths(localBackupCandidates)
	manifestDigest, err := backupManifestSHA256(localBackupRoot, localBackupCandidates)
	if err != nil {
		return MigrationReport{}, fmt.Errorf("encode legacy local backup manifest: %w", err)
	}

	tx, err := commandConn.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
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
	if err := assertNoBatchImageRows(ctx, tx); err != nil {
		return MigrationReport{}, err
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
	baseReport := MigrationReport{
		SchemaVersion:             privateSchemaVersion,
		OperatorID:                operatorID,
		CutoverAt:                 cutoverAt,
		DeletedUsers:              deletedUsers,
		DroppedTables:             append([]string(nil), SaaSTables...),
		ManagedBackupsPreserved:   managedBackupRecordsPreserved,
		PurgedSettings:            purgedSettings,
		PurgedProtected:           purgedProtected,
		Confirmation:              confirmation,
		LocalBackupManifestSHA256: manifestDigest,
		BatchCleanupEvidence:      options.BatchCleanupEvidence,
	}
	evidence := cutoverEvidence{
		Report:                         baseReport,
		ReportKeySHA256:                reportKeySHA256(reportKey),
		LocalBackupCandidates:          localBackupCandidatePaths,
		LocalBackupCandidateIdentities: append([]backupCandidateIdentity(nil), localBackupCandidates...),
		LocalBackupRoot:                localBackupRoot,
	}
	if err := insertCutoverEvidence(ctx, tx, evidence); err != nil {
		return MigrationReport{}, err
	}
	if err := tx.Commit(); err != nil {
		return MigrationReport{}, fmt.Errorf("commit private cutover: %w", err)
	}
	committed = true

	return finalizeCommittedCutover(ctx, commandConn, reportKey, options, evidence)
}

func insertCutoverEvidence(ctx context.Context, db contextExecer, evidence cutoverEvidence) error {
	evidenceJSON, err := json.Marshal(evidence)
	if err != nil {
		return fmt.Errorf("encode private cutover evidence: %w", err)
	}
	report := evidence.Report
	stateResult, err := db.ExecContext(ctx, privateStateUpsertSQL, report.SchemaVersion, report.OperatorID, report.CutoverAt, string(evidenceJSON))
	if err != nil {
		return fmt.Errorf("record private schema version: %w", err)
	}
	inserted, err := stateResult.RowsAffected()
	if err != nil {
		return fmt.Errorf("verify private state evidence persistence: %w", err)
	}
	if inserted != 1 {
		return errors.New("private state appeared during cutover; refusing to commit without the original evidence")
	}
	return nil
}

func ensurePrivateStateSchema(ctx context.Context, db contextExecer) error {
	if _, err := db.ExecContext(ctx, privateStateSchemaSQL); err != nil {
		return fmt.Errorf("create private state: %w", err)
	}
	if _, err := db.ExecContext(ctx, privateStateEvidenceColumnSQL); err != nil {
		return fmt.Errorf("add private state evidence: %w", err)
	}
	return nil
}

type contextQueryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type privateCutoverState struct {
	SchemaVersion int
	OperatorID    int64
	CutoverAt     time.Time
	ReportSHA256  string
	EvidenceJSON  []byte
}

func loadPrivateCutoverState(ctx context.Context, db contextQueryRower) (privateCutoverState, cutoverEvidence, bool, error) {
	var state privateCutoverState
	err := db.QueryRowContext(ctx, `SELECT private_schema_version, operator_id, cutover_at, report_sha256, cutover_evidence FROM exapi_private_state WHERE id = true`).
		Scan(&state.SchemaVersion, &state.OperatorID, &state.CutoverAt, &state.ReportSHA256, &state.EvidenceJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return privateCutoverState{}, cutoverEvidence{}, false, nil
	}
	if err != nil {
		return privateCutoverState{}, cutoverEvidence{}, false, fmt.Errorf("load committed private cutover state: %w", err)
	}
	var evidence cutoverEvidence
	if err := json.Unmarshal(state.EvidenceJSON, &evidence); err != nil {
		return privateCutoverState{}, cutoverEvidence{}, true, fmt.Errorf("decode committed private cutover evidence: %w", err)
	}
	return state, evidence, true, nil
}

type migrationReportV1 struct {
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

func unsignedV1ReportPayload(report migrationReportV1) ([]byte, error) {
	report.ReportSHA256 = ""
	report.ReportHMACSHA256 = ""
	return json.Marshal(report)
}

func emptyCutoverEvidenceJSON(raw []byte) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("{}"))
}

func readAndVerifyRetainedV1Report(path string, key []byte, state privateCutoverState, operatorID int64, confirmation string) (migrationReportV1, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return migrationReportV1{}, errors.New("schema v1 upgrade requires the retained signed report at --report-file")
	}
	before, err := os.Lstat(path)
	if err != nil {
		return migrationReportV1{}, fmt.Errorf("inspect retained schema v1 report: %w", err)
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() || before.Mode().Perm() != 0o600 {
		return migrationReportV1{}, errors.New("retained schema v1 report must be a regular non-symlink 0600 file")
	}
	if err := validateBackupCandidateLinkCount(path, before); err != nil {
		return migrationReportV1{}, fmt.Errorf("validate retained schema v1 report link count: %w", err)
	}
	file, err := os.Open(path)
	if err != nil {
		return migrationReportV1{}, fmt.Errorf("open retained schema v1 report: %w", err)
	}
	defer func() { _ = file.Close() }()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return migrationReportV1{}, errors.New("retained schema v1 report changed while it was opened")
	}
	signed, err := io.ReadAll(io.LimitReader(file, maxRetainedV1ReportBytes+1))
	if err != nil {
		return migrationReportV1{}, fmt.Errorf("read retained schema v1 report: %w", err)
	}
	if len(signed) > maxRetainedV1ReportBytes {
		return migrationReportV1{}, fmt.Errorf("retained schema v1 report exceeds %d bytes", maxRetainedV1ReportBytes)
	}
	after, err := file.Stat()
	if err != nil || !os.SameFile(opened, after) || opened.Size() != after.Size() || opened.Mode() != after.Mode() || !opened.ModTime().Equal(after.ModTime()) {
		return migrationReportV1{}, errors.New("retained schema v1 report changed while it was read")
	}
	if err := validateBackupCandidateLinkCount(path, after); err != nil {
		return migrationReportV1{}, fmt.Errorf("revalidate retained schema v1 report link count: %w", err)
	}

	decoder := json.NewDecoder(bytes.NewReader(signed))
	decoder.DisallowUnknownFields()
	var report migrationReportV1
	if err := decoder.Decode(&report); err != nil {
		return migrationReportV1{}, fmt.Errorf("decode retained schema v1 report: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return migrationReportV1{}, errors.New("decode retained schema v1 report: trailing JSON content")
	}
	if report.SchemaVersion != 1 || report.OperatorID != operatorID || report.Confirmation != confirmation ||
		report.CutoverAt.IsZero() || report.DeletedUsers < 0 || report.ManagedBackupsPreserved < 0 || !slices.Equal(report.DroppedTables, SaaSTables) {
		return migrationReportV1{}, errors.New("retained schema v1 report metadata is invalid")
	}
	unsigned, err := unsignedV1ReportPayload(report)
	if err != nil {
		return migrationReportV1{}, fmt.Errorf("encode retained schema v1 report: %w", err)
	}
	digest := sha256.Sum256(unsigned)
	expectedDigest := hex.EncodeToString(digest[:])
	if !hmac.Equal([]byte(report.ReportSHA256), []byte(expectedDigest)) {
		return migrationReportV1{}, errors.New("retained schema v1 report SHA-256 does not match its payload")
	}
	providedMAC, err := hex.DecodeString(report.ReportHMACSHA256)
	if err != nil || len(providedMAC) != sha256.Size {
		return migrationReportV1{}, errors.New("retained schema v1 report HMAC-SHA-256 is invalid")
	}
	signer := hmac.New(sha256.New, key)
	_, _ = signer.Write(unsigned)
	if !hmac.Equal(providedMAC, signer.Sum(nil)) {
		return migrationReportV1{}, errors.New("retained schema v1 report HMAC-SHA-256 does not match its payload")
	}
	if state.SchemaVersion != 1 || state.OperatorID != report.OperatorID ||
		!state.CutoverAt.UTC().Equal(report.CutoverAt.UTC().Truncate(time.Microsecond)) ||
		!hmac.Equal([]byte(state.ReportSHA256), []byte(report.ReportSHA256)) {
		return migrationReportV1{}, errors.New("retained schema v1 report does not match durable private state")
	}
	return report, nil
}

func upgradePrivateStateV1(ctx context.Context, commandConn *sql.Conn, confirmation string, reportKey []byte, options CutoverOptions, operatorID int64, upgradedAt time.Time, state privateCutoverState) (MigrationReport, error) {
	v1Report, err := readAndVerifyRetainedV1Report(options.ReportPath, reportKey, state, operatorID, confirmation)
	if err != nil {
		return MigrationReport{}, err
	}
	purgedPaths := append([]string(nil), v1Report.PurgedBackups...)
	if !sortedUniqueAbsolutePaths(purgedPaths) {
		return MigrationReport{}, errors.New("retained schema v1 report purged_backups must be sorted unique absolute paths")
	}
	var localBackupRoot *backupRootIdentity
	if options.AssertNoLocalBackups {
		if len(purgedPaths) != 0 {
			return MigrationReport{}, errors.New("retained schema v1 report contains purged backups; --no-local-backups cannot revalidate their root")
		}
	} else {
		identity, err := identifyBackupRoot(options.LocalBackupDir)
		if err != nil {
			return MigrationReport{}, fmt.Errorf("revalidate schema v1 local backup root: %w", err)
		}
		if err := secureValidateBackupRoot(identity); err != nil {
			return MigrationReport{}, fmt.Errorf("securely revalidate schema v1 local backup root: %w", err)
		}
		localBackupRoot = &identity
		for _, path := range purgedPaths {
			relative, err := filepath.Rel(identity.Path, path)
			if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
				return MigrationReport{}, fmt.Errorf("retained schema v1 purged backup is outside the revalidated root: %s", path)
			}
			exists, err := secureBackupPathExists(identity, relative)
			if err != nil {
				return MigrationReport{}, fmt.Errorf("revalidate schema v1 purged backup %s: %w", path, err)
			}
			if exists {
				return MigrationReport{}, fmt.Errorf("schema v1 purged backup reappeared: %s", path)
			}
		}
	}

	upgradeTimestamp := upgradedAt.UTC().Truncate(time.Microsecond)
	report := MigrationReport{
		SchemaVersion:             privateSchemaVersion,
		OperatorID:                v1Report.OperatorID,
		CutoverAt:                 state.CutoverAt.UTC(),
		DeletedUsers:              v1Report.DeletedUsers,
		DroppedTables:             append([]string(nil), v1Report.DroppedTables...),
		PurgedLocalBackups:        append([]string(nil), purgedPaths...),
		UpgradedFromSchemaVersion: 1,
		UpgradedAt:                &upgradeTimestamp,
		BatchCleanupEvidence:      options.BatchCleanupEvidence,
		ManagedBackupsPreserved:   v1Report.ManagedBackupsPreserved,
		PurgedSettings:            append([]string(nil), v1Report.PurgedSettings...),
		PurgedProtected:           append([]string(nil), v1Report.PurgedProtected...),
		Confirmation:              v1Report.Confirmation,
	}
	evidence := cutoverEvidence{
		Report:                report,
		ReportKeySHA256:       reportKeySHA256(reportKey),
		LocalBackupCandidates: append([]string(nil), purgedPaths...),
		LocalBackupRoot:       localBackupRoot,
	}
	if err := validateCommittedEvidence(evidence, operatorID, confirmation); err != nil {
		return MigrationReport{}, fmt.Errorf("construct schema v2 upgrade evidence: %w", err)
	}
	encodedEvidence, err := json.Marshal(evidence)
	if err != nil {
		return MigrationReport{}, fmt.Errorf("encode schema v2 upgrade evidence: %w", err)
	}

	tx, err := commandConn.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return MigrationReport{}, fmt.Errorf("begin schema v1 upgrade: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, advisoryLockKey); err != nil {
		return MigrationReport{}, fmt.Errorf("acquire schema v1 upgrade advisory lock: %w", err)
	}
	var locked privateCutoverState
	if err := tx.QueryRowContext(ctx, `SELECT private_schema_version, operator_id, cutover_at, report_sha256, cutover_evidence FROM exapi_private_state WHERE id = true FOR UPDATE`).
		Scan(&locked.SchemaVersion, &locked.OperatorID, &locked.CutoverAt, &locked.ReportSHA256, &locked.EvidenceJSON); err != nil {
		return MigrationReport{}, fmt.Errorf("lock schema v1 private state: %w", err)
	}
	if locked.SchemaVersion != state.SchemaVersion || locked.OperatorID != state.OperatorID ||
		!locked.CutoverAt.UTC().Equal(state.CutoverAt.UTC()) || !hmac.Equal([]byte(locked.ReportSHA256), []byte(state.ReportSHA256)) ||
		!emptyCutoverEvidenceJSON(locked.EvidenceJSON) {
		return MigrationReport{}, errors.New("schema v1 private state changed during upgrade")
	}
	if err := assertNoBatchImageRows(ctx, tx); err != nil {
		return MigrationReport{}, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE exapi_private_state
		SET private_schema_version = $1, report_sha256 = '', cutover_evidence = $2
		WHERE id = true AND private_schema_version = 1 AND operator_id = $3 AND cutover_at = $4
		  AND report_sha256 = $5 AND cutover_evidence = '{}'::jsonb`,
		privateSchemaVersion, string(encodedEvidence), state.OperatorID, state.CutoverAt, state.ReportSHA256)
	if err != nil {
		return MigrationReport{}, fmt.Errorf("persist schema v2 upgrade evidence: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return MigrationReport{}, fmt.Errorf("verify schema v2 upgrade persistence: %w", err)
	}
	if updated != 1 {
		return MigrationReport{}, errors.New("schema v1 private state changed before upgrade commit")
	}
	if err := tx.Commit(); err != nil {
		return MigrationReport{}, fmt.Errorf("commit schema v1 private state upgrade: %w", err)
	}
	committed = true
	return finalizeCommittedCutover(ctx, commandConn, reportKey, options, evidence)
}

func validateCommittedEvidence(evidence cutoverEvidence, operatorID int64, confirmation string) error {
	report := evidence.Report
	if report.SchemaVersion != privateSchemaVersion || report.CutoverAt.IsZero() ||
		report.DeletedUsers < 0 ||
		report.ManagedBackupsPreserved < 0 ||
		!slices.Equal(report.DroppedTables, SaaSTables) {
		return errors.New("committed private cutover evidence is incomplete or incompatible")
	}
	batchEvidenceReference := report.CutoverAt
	switch {
	case report.UpgradedFromSchemaVersion == 0 && report.UpgradedAt == nil:
	case report.UpgradedFromSchemaVersion == 1 && report.UpgradedAt != nil && !report.UpgradedAt.Before(report.CutoverAt):
		batchEvidenceReference = report.UpgradedAt.UTC()
	default:
		return errors.New("committed private cutover upgrade metadata is invalid")
	}
	if err := validateBatchCleanupEvidence(report.BatchCleanupEvidence, batchEvidenceReference); err != nil {
		return fmt.Errorf("committed private cutover batch cleanup evidence is invalid: %w", err)
	}
	if report.OperatorID != operatorID || report.Confirmation != confirmation {
		return fmt.Errorf("confirmation is for operator %d, but committed private cutover belongs to operator %d", operatorID, report.OperatorID)
	}
	if report.ReportSHA256 != "" || report.ReportHMACSHA256 != "" {
		return errors.New("committed private cutover evidence unexpectedly contains report signatures")
	}
	if len(evidence.ReportKeySHA256) != sha256.Size*2 {
		return errors.New("committed private cutover report-key fingerprint is invalid")
	}
	if _, err := hex.DecodeString(evidence.ReportKeySHA256); err != nil {
		return errors.New("committed private cutover report-key fingerprint is invalid")
	}
	if !sortedUniqueAbsolutePaths(evidence.LocalBackupCandidates) ||
		!sortedUniqueSubset(report.PurgedLocalBackups, evidence.LocalBackupCandidates) ||
		!sortedUniqueSubset(evidence.LocalBackupDeletionIntents, evidence.LocalBackupCandidates) ||
		!sortedSetsDisjoint(report.PurgedLocalBackups, evidence.LocalBackupDeletionIntents) {
		return errors.New("committed private cutover backup evidence is invalid")
	}
	if len(evidence.LocalBackupCandidates) > 0 && evidence.LocalBackupRoot == nil {
		return errors.New("committed private cutover backup root evidence is missing")
	}
	if evidence.LocalBackupRoot != nil {
		if !filepath.IsAbs(evidence.LocalBackupRoot.Path) || filepath.Clean(evidence.LocalBackupRoot.Path) != evidence.LocalBackupRoot.Path ||
			evidence.LocalBackupRoot.Device == 0 || evidence.LocalBackupRoot.Inode == 0 {
			return errors.New("committed private cutover backup root evidence is invalid")
		}
		for _, candidate := range evidence.LocalBackupCandidates {
			relative, err := filepath.Rel(evidence.LocalBackupRoot.Path, candidate)
			if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
				return errors.New("committed private cutover backup candidate is outside its root")
			}
		}
	}
	if len(evidence.LocalBackupCandidateIdentities) == 0 {
		// Reports finalized by the earlier v2 implementation contained path-only
		// evidence. Continue to verify those reports, but never resume deletion
		// from an identity-less manifest.
		if len(evidence.LocalBackupCandidates) > 0 && (report.LocalBackupManifestSHA256 != "" ||
			!slices.Equal(report.PurgedLocalBackups, evidence.LocalBackupCandidates)) ||
			len(evidence.LocalBackupDeletionIntents) > 0 {
			return errors.New("committed private cutover backup evidence lacks candidate identities")
		}
		if report.LocalBackupManifestSHA256 != "" {
			manifestDigest, err := backupManifestSHA256(evidence.LocalBackupRoot, nil)
			if err != nil || !hmac.Equal([]byte(report.LocalBackupManifestSHA256), []byte(manifestDigest)) {
				return errors.New("committed private cutover backup manifest digest is invalid")
			}
		}
		return nil
	}
	if len(evidence.LocalBackupCandidateIdentities) != len(evidence.LocalBackupCandidates) || evidence.LocalBackupRoot == nil {
		return errors.New("committed private cutover backup candidate identities are incomplete")
	}
	for index, candidate := range evidence.LocalBackupCandidateIdentities {
		if candidate.Path != evidence.LocalBackupCandidates[index] || !validBackupCandidateIdentity(*evidence.LocalBackupRoot, report.CutoverAt, candidate) {
			return errors.New("committed private cutover backup candidate identity is invalid")
		}
	}
	manifestDigest, err := backupManifestSHA256(evidence.LocalBackupRoot, evidence.LocalBackupCandidateIdentities)
	if err != nil || !hmac.Equal([]byte(report.LocalBackupManifestSHA256), []byte(manifestDigest)) {
		return errors.New("committed private cutover backup manifest digest is invalid")
	}
	return nil
}

func reportKeySHA256(key []byte) string {
	digest := sha256.Sum256(key)
	return hex.EncodeToString(digest[:])
}

func backupCandidatePaths(candidates []backupCandidateIdentity) []string {
	paths := make([]string, len(candidates))
	for index, candidate := range candidates {
		paths[index] = candidate.Path
	}
	return paths
}

func backupManifestSHA256(root *backupRootIdentity, candidates []backupCandidateIdentity) (string, error) {
	encoded, err := json.Marshal(backupManifest{Root: root, Candidates: candidates})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func validBackupCandidateIdentity(root backupRootIdentity, _ time.Time, candidate backupCandidateIdentity) bool {
	if !filepath.IsAbs(candidate.Path) || filepath.IsAbs(candidate.RelativePath) ||
		candidate.RelativePath == "" || candidate.RelativePath == "." || filepath.Clean(candidate.RelativePath) != candidate.RelativePath {
		return false
	}
	if candidate.RelativePath == ".." || strings.HasPrefix(candidate.RelativePath, ".."+string(os.PathSeparator)) ||
		filepath.Join(root.Path, candidate.RelativePath) != candidate.Path {
		return false
	}
	if candidate.Device == 0 || candidate.Inode == 0 || candidate.Size < 0 || candidate.LinkCount != 1 ||
		!os.FileMode(candidate.Mode).IsRegular() {
		return false
	}
	if len(candidate.ContentSHA256) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(candidate.ContentSHA256)
	return err == nil && candidate.ContentSHA256 == strings.ToLower(candidate.ContentSHA256)
}

func sortedUniqueAbsolutePaths(paths []string) bool {
	for index, path := range paths {
		if !filepath.IsAbs(path) || (index > 0 && paths[index-1] >= path) {
			return false
		}
	}
	return true
}

func sortedUniqueSubset(subset, set []string) bool {
	if !sort.StringsAreSorted(subset) {
		return false
	}
	setIndex := 0
	for index, value := range subset {
		if index > 0 && subset[index-1] == value {
			return false
		}
		for setIndex < len(set) && set[setIndex] < value {
			setIndex++
		}
		if setIndex == len(set) || set[setIndex] != value {
			return false
		}
		setIndex++
	}
	return true
}

func sortedSetsDisjoint(left, right []string) bool {
	leftIndex, rightIndex := 0, 0
	for leftIndex < len(left) && rightIndex < len(right) {
		switch {
		case left[leftIndex] == right[rightIndex]:
			return false
		case left[leftIndex] < right[rightIndex]:
			leftIndex++
		default:
			rightIndex++
		}
	}
	return true
}

func finalizeCommittedCutover(ctx context.Context, commandConn *sql.Conn, reportKey []byte, options CutoverOptions, evidence cutoverEvidence) (MigrationReport, error) {
	if err := validateBackupRootChoice(evidence.LocalBackupRoot, options); err != nil {
		return MigrationReport{}, err
	}
	updatedEvidence, err := purgeBackupCandidatesDurably(ctx, commandConn, evidence)
	if err != nil {
		return MigrationReport{}, fmt.Errorf("purge legacy local backups: %w", err)
	}
	evidence = updatedEvidence
	if len(evidence.LocalBackupDeletionIntents) != 0 {
		return MigrationReport{}, errors.New("purge legacy local backups: deletion journal is not empty after finalization")
	}
	report := evidence.Report

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
	if err := recordReportDigest(ctx, commandConn, report.ReportSHA256); err != nil {
		return MigrationReport{}, err
	}
	return report, nil
}

func recordCutoverEvidence(ctx context.Context, db contextExecer, evidence []byte) error {
	result, err := db.ExecContext(ctx, `UPDATE exapi_private_state SET cutover_evidence = $1 WHERE id = true`, string(evidence))
	if err != nil {
		return fmt.Errorf("record private cutover evidence: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("verify private cutover evidence persistence: %w", err)
	}
	if updated != 1 {
		return fmt.Errorf("record private cutover evidence: expected one private state row, updated %d", updated)
	}
	return nil
}

func persistCutoverEvidence(ctx context.Context, db contextExecer, evidence cutoverEvidence) error {
	encoded, err := json.Marshal(evidence)
	if err != nil {
		return fmt.Errorf("encode private cutover evidence: %w", err)
	}
	return recordCutoverEvidence(ctx, db, encoded)
}

func withCommandLock(ctx context.Context, db *sql.DB, operation func(*sql.Conn) (MigrationReport, error)) (result MigrationReport, retErr error) {
	commandConn, releaseCommandLock, err := acquireCommandLock(ctx, db)
	if err != nil {
		return MigrationReport{}, err
	}
	// Keep separate invocations serialized through the post-commit report
	// install and digest update. The transaction-scoped lock below is released
	// too early to protect those filesystem/database operations by itself.
	defer func() {
		if err := releaseCommandLock(); err != nil {
			result = MigrationReport{}
			retErr = errors.Join(retErr, err)
		}
	}()
	return operation(commandConn)
}

func acquireCommandLock(ctx context.Context, db *sql.DB) (*sql.Conn, func() error, error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("acquire cutover command lock connection: %w", err)
	}
	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock(hashtext($1))`, commandAdvisoryLockKey); err != nil {
		discardSQLConn(conn)
		return nil, nil, fmt.Errorf("acquire cutover command lock: %w", err)
	}
	return conn, func() error {
		unlockCtx, cancel := context.WithTimeout(context.Background(), commandLockReleaseLimit)
		defer cancel()
		var unlocked bool
		if err := conn.QueryRowContext(unlockCtx, `SELECT pg_advisory_unlock(hashtext($1))`, commandAdvisoryLockKey).Scan(&unlocked); err != nil {
			discardSQLConn(conn)
			return fmt.Errorf("release cutover command lock: %w", err)
		}
		if !unlocked {
			discardSQLConn(conn)
			return errors.New("release cutover command lock: PostgreSQL reported that this session did not hold the lock")
		}
		if err := conn.Close(); err != nil {
			return fmt.Errorf("release cutover command lock connection: %w", err)
		}
		return nil
	}, nil
}

func discardSQLConn(conn *sql.Conn) {
	// Returning driver.ErrBadConn from Raw tells database/sql not to put this
	// physical PostgreSQL session back into the pool. That is essential when an
	// advisory-lock operation has an ambiguous outcome.
	_ = conn.Raw(func(any) error { return driver.ErrBadConn })
	_ = conn.Close()
}

func validateBackupOptions(options CutoverOptions) error {
	if err := validateBackupChoice(options); err != nil {
		return err
	}
	directory := strings.TrimSpace(options.LocalBackupDir)
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

func validateBackupChoice(options CutoverOptions) error {
	directory := strings.TrimSpace(options.LocalBackupDir)
	if directory != "" && options.AssertNoLocalBackups {
		return errors.New("--local-backup-dir and --no-local-backups are mutually exclusive")
	}
	if directory == "" && !options.AssertNoLocalBackups {
		return errors.New("an explicit --local-backup-dir is required; use --no-local-backups only after verifying no legacy local backup directory exists")
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
	cleanPath := filepath.Clean(path)
	if destinationInfo, err := os.Stat(cleanPath); err == nil {
		if destinationInfo.IsDir() {
			return fmt.Errorf("validate migration report path: %s is a directory", cleanPath)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("validate migration report path: %w", err)
	}

	directory := filepath.Dir(cleanPath)
	info, err := os.Stat(directory)
	if err != nil {
		return fmt.Errorf("validate migration report directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("validate migration report directory: %s is not a directory", directory)
	}
	// Prove the final report can be staged with its required protections before
	// the destructive transaction begins. Stat/access-bit checks alone are not
	// sufficient for read-only mounts, ACLs, quotas, or exhausted filesystems.
	probe, err := os.CreateTemp(directory, "."+filepath.Base(cleanPath)+".preflight-*")
	if err != nil {
		return fmt.Errorf("probe migration report destination: %w", err)
	}
	probePath := probe.Name()
	defer func() {
		_ = probe.Close()
		_ = os.Remove(probePath)
	}()
	if err := probe.Chmod(0o600); err != nil {
		return fmt.Errorf("protect migration report preflight: %w", err)
	}
	if err := probe.Sync(); err != nil {
		return fmt.Errorf("sync migration report preflight: %w", err)
	}
	if err := probe.Close(); err != nil {
		return fmt.Errorf("close migration report preflight: %w", err)
	}
	if err := os.Remove(probePath); err != nil {
		return fmt.Errorf("remove migration report preflight: %w", err)
	}
	directoryHandle, err := os.Open(directory)
	if err != nil {
		return fmt.Errorf("open migration report directory preflight: %w", err)
	}
	defer func() { _ = directoryHandle.Close() }()
	if err := directoryHandle.Sync(); err != nil {
		return fmt.Errorf("sync migration report directory preflight: %w", err)
	}
	return nil
}

func validateReportOutsideLocalBackupRoot(options CutoverOptions) error {
	reportPath := strings.TrimSpace(options.ReportPath)
	backupDirectory := strings.TrimSpace(options.LocalBackupDir)
	if reportPath == "" || backupDirectory == "" {
		return nil
	}
	root, err := filepath.EvalSymlinks(backupDirectory)
	if err != nil {
		return fmt.Errorf("resolve legacy local backup directory for report isolation: %w", err)
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("canonicalize legacy local backup directory for report isolation: %w", err)
	}
	reportDirectory, err := filepath.EvalSymlinks(filepath.Dir(filepath.Clean(reportPath)))
	if err != nil {
		return fmt.Errorf("resolve migration report directory for backup isolation: %w", err)
	}
	reportDirectory, err = filepath.Abs(reportDirectory)
	if err != nil {
		return fmt.Errorf("canonicalize migration report directory for backup isolation: %w", err)
	}
	canonicalReportPath := filepath.Join(reportDirectory, filepath.Base(filepath.Clean(reportPath)))
	relative, err := filepath.Rel(root, canonicalReportPath)
	if err != nil {
		return fmt.Errorf("compare migration report and legacy local backup paths: %w", err)
	}
	if relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator))) {
		return errors.New("migration report path must be outside the legacy local backup root")
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

func assertNoBatchImageRows(ctx context.Context, tx *sql.Tx) error {
	var count int64
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM batch_image_jobs`).Scan(&count); err != nil {
		return fmt.Errorf("inspect batch image jobs: %w", err)
	}
	if count != 0 {
		return fmt.Errorf("batch image cleanup prerequisite failed: %d job(s) remain; cancel provider jobs and delete provider-managed inputs/outputs before cutover", count)
	}
	return nil
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
	defer func() { _ = directoryHandle.Close() }()
	if err := directoryHandle.Sync(); err != nil {
		return fmt.Errorf("sync migration report directory: %w", err)
	}
	return nil
}

// WriteDurableFile atomically installs a protected 0600 file and fsyncs its
// containing directory before returning.
func WriteDurableFile(path string, contents []byte) error {
	return writeDurableReport(path, nil, contents)
}

func purgeSettingRows(ctx context.Context, tx *sql.Tx, table string) ([]string, error) {
	rows, err := tx.QueryContext(ctx, `DELETE FROM `+quoteIdentifier(table)+` WHERE key = ANY($1) OR EXISTS (SELECT 1 FROM unnest($2::text[]) AS prefix WHERE key LIKE prefix || '%') RETURNING key`, pq.Array(SaaSSettingKeys), pq.Array(SaaSSettingPrefixes))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
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
// operator is the only retained user. Batch jobs are required to be empty at
// cutover, but legitimate operator-owned jobs may be created afterward and do
// not invalidate the durable private-state proof.
func VerifyRuntimeState(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return errors.New("database is required")
	}
	var stateVersion int
	var operatorID int64
	var cutoverAt time.Time
	var reportDigest string
	var evidenceJSON []byte
	var sqlStateValid bool
	err := db.QueryRowContext(ctx, `
		SELECT state.private_schema_version,
		       state.operator_id,
		       state.cutover_at,
		       state.report_sha256,
		       state.cutover_evidence,
		       (SELECT COUNT(*) FROM users) = 1
		       AND (SELECT COUNT(*) FROM users WHERE role = 'admin' AND status = 'active' AND deleted_at IS NULL) = 1
		       AND EXISTS (
				SELECT 1 FROM users operator
				WHERE operator.id = state.operator_id
				  AND operator.role = 'admin'
				  AND operator.status = 'active'
				  AND operator.deleted_at IS NULL
		       )
		FROM exapi_private_state state
		WHERE state.id = true
	`).Scan(&stateVersion, &operatorID, &cutoverAt, &reportDigest, &evidenceJSON, &sqlStateValid)
	if errors.Is(err, sql.ErrNoRows) {
		return errors.New("private runtime state is incomplete; run migrate-private-only offline and verify its signed report")
	}
	if err != nil {
		return fmt.Errorf("query private runtime state: %w", err)
	}
	var evidence cutoverEvidence
	if err := json.Unmarshal(evidenceJSON, &evidence); err != nil {
		return fmt.Errorf("decode private runtime evidence: %w", err)
	}
	if err := validateCommittedEvidence(evidence, operatorID, ExpectedConfirmation(operatorID)); err != nil {
		return fmt.Errorf("validate private runtime evidence: %w", err)
	}
	unsigned, err := unsignedReportPayload(evidence.Report)
	if err != nil {
		return fmt.Errorf("encode private runtime evidence: %w", err)
	}
	digest := sha256.Sum256(unsigned)
	expectedDigest := hex.EncodeToString(digest[:])
	if !sqlStateValid || stateVersion != privateSchemaVersion || evidence.Report.SchemaVersion != stateVersion ||
		evidence.Report.OperatorID != operatorID || !evidence.Report.CutoverAt.UTC().Equal(cutoverAt.UTC()) ||
		!hmac.Equal([]byte(reportDigest), []byte(expectedDigest)) {
		return errors.New("private runtime state is incomplete; run migrate-private-only offline and verify its signed report")
	}
	return nil
}

type contextExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func recordReportDigest(ctx context.Context, db contextExecer, digest string) error {
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

func snapshotPreCutoverBackups(directory string, cutoverAt time.Time) (backupRootIdentity, []backupCandidateIdentity, error) {
	directory = strings.TrimSpace(directory)
	if directory == "" {
		return backupRootIdentity{}, nil, errors.New("legacy local backup directory is required")
	}
	root, err := identifyBackupRoot(directory)
	if err != nil {
		return backupRootIdentity{}, nil, err
	}
	candidates, err := secureSnapshotBackupTree(root, cutoverAt)
	if err != nil {
		return backupRootIdentity{}, nil, err
	}
	return root, candidates, nil
}

func identifyBackupRoot(directory string) (backupRootIdentity, error) {
	resolved, err := filepath.EvalSymlinks(strings.TrimSpace(directory))
	if err != nil {
		return backupRootIdentity{}, err
	}
	absolute, err := filepath.Abs(resolved)
	if err != nil {
		return backupRootIdentity{}, err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return backupRootIdentity{}, err
	}
	if !info.IsDir() {
		return backupRootIdentity{}, fmt.Errorf("%s is not a directory", absolute)
	}
	device, inode, err := fileIdentity(info)
	if err != nil {
		return backupRootIdentity{}, err
	}
	return backupRootIdentity{Path: absolute, Device: device, Inode: inode}, nil
}

func fileIdentity(info os.FileInfo) (uint64, uint64, error) {
	value := reflect.ValueOf(info.Sys())
	if value.Kind() == reflect.Pointer {
		value = value.Elem()
	}
	if !value.IsValid() || value.Kind() != reflect.Struct {
		return 0, 0, errors.New("filesystem does not expose stable root identity")
	}
	deviceField := value.FieldByName("Dev")
	inodeField := value.FieldByName("Ino")
	if !deviceField.IsValid() || !inodeField.IsValid() || !deviceField.CanUint() || !inodeField.CanUint() {
		return 0, 0, errors.New("filesystem does not expose device/inode identity")
	}
	device, inode := deviceField.Uint(), inodeField.Uint()
	if device == 0 || inode == 0 {
		return 0, 0, errors.New("filesystem returned an empty device/inode identity")
	}
	return device, inode, nil
}

func validateBackupCandidateLinkCount(path string, info os.FileInfo) error {
	linkCount, err := backupCandidateLinkCount(path, info)
	if err != nil {
		return err
	}
	if linkCount != 1 {
		return fmt.Errorf("legacy local backup candidate must not be hardlinked: %s has %d links", path, linkCount)
	}
	return nil
}

func backupCandidateLinkCount(path string, info os.FileInfo) (uint64, error) {
	value := reflect.ValueOf(info.Sys())
	if value.Kind() == reflect.Pointer {
		value = value.Elem()
	}
	if !value.IsValid() || value.Kind() != reflect.Struct {
		return 0, fmt.Errorf("inspect legacy local backup hardlink count for %s: filesystem does not expose stable link count", path)
	}
	linkCountField := value.FieldByName("Nlink")
	if !linkCountField.IsValid() {
		return 0, fmt.Errorf("inspect legacy local backup hardlink count for %s: filesystem does not expose stable link count", path)
	}

	var linkCount uint64
	switch linkCountField.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		linkCount = linkCountField.Uint()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if linkCountField.Int() <= 0 {
			return 0, fmt.Errorf("inspect legacy local backup hardlink count for %s: filesystem returned invalid link count", path)
		}
		linkCount = uint64(linkCountField.Int())
	default:
		return 0, fmt.Errorf("inspect legacy local backup hardlink count for %s: filesystem does not expose stable link count", path)
	}
	if linkCount == 0 {
		return 0, fmt.Errorf("inspect legacy local backup hardlink count for %s: filesystem returned invalid link count", path)
	}
	return linkCount, nil
}

func validateBackupRootChoice(expected *backupRootIdentity, options CutoverOptions) error {
	directory := strings.TrimSpace(options.LocalBackupDir)
	if expected == nil {
		if directory != "" {
			return errors.New("committed cutover asserted no local backups; refusing a different backup mode")
		}
		return nil
	}
	if directory == "" {
		return errors.New("committed cutover requires its original local backup root")
	}
	actual, err := identifyBackupRoot(directory)
	if err != nil {
		return fmt.Errorf("revalidate legacy local backup root: %w", err)
	}
	if actual != *expected {
		return fmt.Errorf("legacy local backup root identity changed: expected %s device=%d inode=%d", expected.Path, expected.Device, expected.Inode)
	}
	return nil
}

func purgeBackupCandidatesDurably(ctx context.Context, db contextExecer, evidence cutoverEvidence) (cutoverEvidence, error) {
	if evidence.LocalBackupRoot == nil {
		if len(evidence.LocalBackupCandidates) != 0 {
			return cutoverEvidence{}, errors.New("committed backup root evidence is missing")
		}
		return evidence, nil
	}
	legacyPathOnlyEvidence := len(evidence.LocalBackupCandidateIdentities) == 0
	for index, path := range evidence.LocalBackupCandidates {
		relative, err := filepath.Rel(evidence.LocalBackupRoot.Path, path)
		if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
			return cutoverEvidence{}, fmt.Errorf("backup candidate is outside its committed root: %s", path)
		}
		wasPurged := sortedContains(evidence.Report.PurgedLocalBackups, path)
		if wasPurged {
			exists, existsErr := secureBackupPathExists(*evidence.LocalBackupRoot, relative)
			if existsErr != nil {
				return cutoverEvidence{}, existsErr
			}
			if exists {
				return cutoverEvidence{}, fmt.Errorf("previously purged backup candidate reappeared: %s", path)
			}
			continue
		}
		if legacyPathOnlyEvidence {
			return cutoverEvidence{}, fmt.Errorf("backup candidate %s has no committed identity; refusing legacy path-only deletion", path)
		}
		candidate := evidence.LocalBackupCandidateIdentities[index]
		hasIntent := sortedContains(evidence.LocalBackupDeletionIntents, path)
		if !hasIntent {
			if err := secureValidateBackupCandidate(*evidence.LocalBackupRoot, candidate); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return cutoverEvidence{}, fmt.Errorf("backup candidate disappeared without committed deletion intent: %s", path)
				}
				return cutoverEvidence{}, err
			}
			evidence.LocalBackupDeletionIntents = sortedInsert(evidence.LocalBackupDeletionIntents, path)
			if err := persistCutoverEvidence(ctx, db, evidence); err != nil {
				return cutoverEvidence{}, fmt.Errorf("record backup deletion intent for %s: %w", path, err)
			}
		}

		// The committed intent is a write-ahead journal entry. If unlink completed
		// but the completion update did not, absence is now explained and can be
		// promoted to committed purge evidence on retry.
		if err := secureRemoveBackupCandidate(*evidence.LocalBackupRoot, candidate); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return cutoverEvidence{}, err
			}
			if err := secureConfirmBackupCandidateAbsent(*evidence.LocalBackupRoot, candidate.RelativePath); err != nil {
				return cutoverEvidence{}, err
			}
		}
		evidence.LocalBackupDeletionIntents = sortedRemove(evidence.LocalBackupDeletionIntents, path)
		evidence.Report.PurgedLocalBackups = sortedInsert(evidence.Report.PurgedLocalBackups, path)
		if err := persistCutoverEvidence(ctx, db, evidence); err != nil {
			return cutoverEvidence{}, fmt.Errorf("record completed backup deletion for %s: %w", path, err)
		}
	}
	// A committed manifest is a deletion allowlist, not proof that no other
	// backups exist. Re-scan the securely reopened root before signing so files
	// added after the snapshot (and files omitted by any legacy logic) cannot be
	// silently legitimized by a ready report.
	if err := secureAssertBackupTreeHasNoFiles(*evidence.LocalBackupRoot); err != nil {
		return cutoverEvidence{}, err
	}
	return evidence, nil
}

func sortedContains(values []string, value string) bool {
	_, found := slices.BinarySearch(values, value)
	return found
}

func sortedInsert(values []string, value string) []string {
	index, found := slices.BinarySearch(values, value)
	if found {
		return values
	}
	values = append(values, "")
	copy(values[index+1:], values[index:])
	values[index] = value
	return values
}

func sortedRemove(values []string, value string) []string {
	index, found := slices.BinarySearch(values, value)
	if !found {
		return values
	}
	return append(values[:index], values[index+1:]...)
}
