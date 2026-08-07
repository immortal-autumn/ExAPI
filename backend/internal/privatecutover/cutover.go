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
	"strconv"
	"strings"
	"time"
)

const (
	ConfirmationPrefix   = "DROP-SAAS-DATA-KEEP-USER-"
	privateSchemaVersion = 1
	advisoryLockKey      = "exapi:migrate-private-only:v1"
)

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
	"pending_auth_sessions",
	"auth_identity_channels",
	"auth_identities",
	"passkey_credentials",
	"passkey_user_handles",
}

type MigrationReport struct {
	SchemaVersion    int       `json:"private_schema_version"`
	OperatorID       int64     `json:"operator_id"`
	CutoverAt        time.Time `json:"cutover_at"`
	DeletedUsers     int64     `json:"deleted_users"`
	DroppedTables    []string  `json:"dropped_tables"`
	PurgedBackups    []string  `json:"purged_backups"`
	Confirmation     string    `json:"confirmation"`
	ReportSHA256     string    `json:"report_sha256"`
	ReportHMACSHA256 string    `json:"report_hmac_sha256"`
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
// pre-cutover backup files. The caller must supply the exact confirmation and
// a 32-byte-or-longer report key. No caller in the online server invokes it.
func Run(ctx context.Context, db *sql.DB, confirmation string, reportKey []byte, backupDir string, now func() time.Time, output io.Writer) (MigrationReport, error) {
	if db == nil {
		return MigrationReport{}, errors.New("database is required")
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

	for _, table := range SaaSTables {
		if _, err := tx.ExecContext(ctx, `DROP TABLE IF EXISTS `+quoteIdentifier(table)+` CASCADE`); err != nil {
			return MigrationReport{}, fmt.Errorf("drop SaaS table %s: %w", table, err)
		}
	}
	var deletedUsers int64
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE id <> $1`, operatorID).Scan(&deletedUsers); err != nil {
		return MigrationReport{}, fmt.Errorf("count customer users: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM users WHERE id <> $1`, operatorID); err != nil {
		return MigrationReport{}, fmt.Errorf("delete customer users: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET role = 'admin', status = 'active', deleted_at = NULL WHERE id = $1`, operatorID); err != nil {
		return MigrationReport{}, fmt.Errorf("retain operator: %w", err)
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

	purged, err := purgePreCutoverBackups(backupDir, cutoverAt)
	if err != nil {
		return MigrationReport{}, err
	}
	report := MigrationReport{
		SchemaVersion: privateSchemaVersion,
		OperatorID:    operatorID,
		CutoverAt:     cutoverAt,
		DeletedUsers:  deletedUsers,
		DroppedTables: append([]string(nil), SaaSTables...),
		PurgedBackups: purged,
		Confirmation:  confirmation,
	}
	signed, err := SignReport(report, reportKey)
	if err != nil {
		return MigrationReport{}, err
	}
	if err := json.Unmarshal(signed, &report); err != nil {
		return MigrationReport{}, err
	}
	if output != nil {
		if _, err := output.Write(signed); err != nil {
			return MigrationReport{}, err
		}
		_, _ = output.Write([]byte("\n"))
	}
	return report, nil
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
