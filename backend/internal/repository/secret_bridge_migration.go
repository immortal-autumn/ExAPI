package repository

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"sort"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// SecretBridgeMigrationStats is emitted after every idempotent run. Remaining
// counts make the same command usable as a promotion/cutover gate.
type SecretBridgeMigrationStats struct {
	ProxiesMigrated         int64
	PaymentConfigsMigrated  int64
	SettingsMigrated        int64
	AdminDigestsMigrated    int64
	LegacyProxiesRemaining  int64
	LegacyPaymentsRemaining int64
	LegacySettingsRemaining int64
}

// CutoverSecretBridge irreversibly removes bridge plaintext after a successful
// migration verification. Operational callers must take a database snapshot
// and preserve matching pre-cutover keyroots before invoking it.
func CutoverSecretBridge(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return errors.New("database is required")
	}
	keyrings, err := loadSecretBridgeKeyrings()
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Verification, the completeness gate, and plaintext removal must observe
	// one snapshot. At serializable isolation a concurrent change that would
	// invalidate a verified row aborts the transaction instead of
	// allowing an unverified value to be cleared.
	if err := verifySecretBridgeEnvelopes(ctx, tx, keyrings); err != nil {
		return err
	}
	var stats SecretBridgeMigrationStats
	if err := inspectSecretBridgeRemaining(ctx, tx, &stats); err != nil {
		return err
	}
	if stats.LegacyProxiesRemaining != 0 || stats.LegacyPaymentsRemaining != 0 || stats.LegacySettingsRemaining != 0 {
		return fmt.Errorf("ciphertext-only cutover blocked: legacy values remain without protected counterparts")
	}
	if _, err := tx.ExecContext(ctx, `UPDATE proxies SET password=NULL, updated_at=CURRENT_TIMESTAMP WHERE password_encrypted IS NOT NULL AND password IS NOT NULL`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE payment_provider_instances SET config='', updated_at=CURRENT_TIMESTAMP WHERE config_encrypted IS NOT NULL AND config <> ''`); err != nil {
		return err
	}
	for key, class := range sensitiveSettingRegistry {
		if class != settingClassDataEnvelope && class != settingClassDigest {
			continue
		}
		result, err := tx.ExecContext(ctx, `DELETE FROM settings WHERE key=$1 AND EXISTS (SELECT 1 FROM protected_settings p WHERE p.key=settings.key)`, key)
		if err != nil {
			return err
		}
		_, _ = result.RowsAffected()
	}
	return tx.Commit()
}

// MigrateSecretBridge verifies all candidate plaintext and existing envelopes.
// When execute is true it migrates in bounded, resumable batches; legacy values
// intentionally remain until the separate ciphertext-only cutover.
func MigrateSecretBridge(ctx context.Context, db *sql.DB, batchSize int, execute bool) (SecretBridgeMigrationStats, error) {
	var stats SecretBridgeMigrationStats
	if db == nil {
		return stats, errors.New("database is required")
	}
	if batchSize < 1 || batchSize > 5000 {
		return stats, errors.New("batch size must be between 1 and 5000")
	}
	keyrings, err := loadSecretBridgeKeyrings()
	if err != nil {
		return stats, err
	}

	if execute {
		stats.ProxiesMigrated, err = migrateSecretColumn(ctx, db, batchSize,
			`SELECT id, password FROM proxies WHERE password IS NOT NULL AND password <> '' AND password_encrypted IS NULL ORDER BY id LIMIT $1 FOR UPDATE SKIP LOCKED`,
			`UPDATE proxies SET password_encrypted=$1, updated_at=NOW() WHERE id=$2 AND password_encrypted IS NULL`,
			func(_ int64, value string) (string, error) {
				return keyrings.DataEncryption.Encrypt(proxyPasswordPurpose, []byte(value))
			})
		if err != nil {
			return stats, fmt.Errorf("migrate proxy passwords: %w", err)
		}
		stats.PaymentConfigsMigrated, err = migrateSecretColumn(ctx, db, batchSize,
			`SELECT id, config FROM payment_provider_instances WHERE config <> '' AND config_encrypted IS NULL ORDER BY id LIMIT $1 FOR UPDATE SKIP LOCKED`,
			`UPDATE payment_provider_instances SET config_encrypted=$1, updated_at=NOW() WHERE id=$2 AND config_encrypted IS NULL`,
			func(_ int64, value string) (string, error) {
				return keyrings.DataEncryption.Encrypt(paymentProviderConfigPurpose, []byte(value))
			})
		if err != nil {
			return stats, fmt.Errorf("migrate payment provider configs: %w", err)
		}

		keys := make([]string, 0, len(sensitiveSettingRegistry))
		for key, class := range sensitiveSettingRegistry {
			if class == settingClassDataEnvelope || class == settingClassDigest {
				keys = append(keys, key)
			}
		}
		sort.Strings(keys)
		for _, key := range keys {
			var value string
			err := db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key=$1 AND NOT EXISTS (SELECT 1 FROM protected_settings WHERE key=$1)`, key).Scan(&value)
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			if err != nil {
				return stats, err
			}
			var envelope string
			if sensitiveSettingRegistry[key] == settingClassDigest {
				envelope, err = keyrings.GatewayKeyDigest.Digest(adminAPIKeyDigestPurpose, []byte(value))
			} else {
				envelope, err = keyrings.DataEncryption.Encrypt("settings."+key, []byte(value))
			}
			if err != nil {
				return stats, fmt.Errorf("protect setting %q: %w", key, err)
			}
			result, err := db.ExecContext(ctx, `INSERT INTO protected_settings (key,envelope,updated_at) VALUES ($1,$2,NOW()) ON CONFLICT (key) DO NOTHING`, key, envelope)
			if err != nil {
				return stats, err
			}
			affected, _ := result.RowsAffected()
			if sensitiveSettingRegistry[key] == settingClassDigest {
				stats.AdminDigestsMigrated += affected
			} else {
				stats.SettingsMigrated += affected
			}
		}
	}

	if err := verifySecretBridgeEnvelopes(ctx, db, keyrings); err != nil {
		return stats, err
	}
	if err := inspectSecretBridgeRemaining(ctx, db, &stats); err != nil {
		return stats, err
	}
	return stats, nil
}

func loadSecretBridgeKeyrings() (*config.ExternalKeyrings, error) {
	keyrings, err := config.LoadExternalKeyringsFromEnv()
	if err != nil {
		return nil, err
	}
	if keyrings.DataEncryption == nil || keyrings.GatewayKeyDigest == nil {
		return nil, errors.New("data-encryption and gateway-key digest keyrings are required")
	}
	return keyrings, nil
}

type secretSealFunc func(id int64, plaintext string) (string, error)

func migrateSecretColumn(ctx context.Context, db *sql.DB, batchSize int, selectSQL, updateSQL string, seal secretSealFunc) (int64, error) {
	var migrated int64
	for {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return migrated, err
		}
		rows, err := tx.QueryContext(ctx, selectSQL, batchSize)
		if err != nil {
			_ = tx.Rollback()
			return migrated, err
		}
		type candidate struct {
			id    int64
			value string
		}
		batch := make([]candidate, 0, batchSize)
		for rows.Next() {
			var item candidate
			if err := rows.Scan(&item.id, &item.value); err != nil {
				_ = rows.Close()
				_ = tx.Rollback()
				return migrated, err
			}
			batch = append(batch, item)
		}
		if err := rows.Close(); err != nil {
			_ = tx.Rollback()
			return migrated, err
		}
		if len(batch) == 0 {
			_ = tx.Rollback()
			return migrated, nil
		}
		for _, item := range batch {
			envelope, err := seal(item.id, item.value)
			if err != nil {
				_ = tx.Rollback()
				return migrated, err
			}
			result, err := tx.ExecContext(ctx, updateSQL, envelope, item.id)
			if err != nil {
				_ = tx.Rollback()
				return migrated, err
			}
			affected, _ := result.RowsAffected()
			migrated += affected
		}
		if err := tx.Commit(); err != nil {
			return migrated, err
		}
	}
}

type secretBridgeQuerier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func verifySecretBridgeEnvelopes(ctx context.Context, db secretBridgeQuerier, keyrings *config.ExternalKeyrings) error {
	rows, err := db.QueryContext(ctx, `SELECT id,password,password_encrypted FROM proxies WHERE password_encrypted IS NOT NULL`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id int64
		var legacy sql.NullString
		var envelope string
		if err := rows.Scan(&id, &legacy, &envelope); err != nil {
			_ = rows.Close()
			return err
		}
		plaintext, err := keyrings.DataEncryption.Decrypt(proxyPasswordPurpose, envelope)
		if err != nil {
			_ = rows.Close()
			return fmt.Errorf("verify proxy %d password: %w", id, err)
		}
		if legacy.Valid && subtle.ConstantTimeCompare([]byte(legacy.String), plaintext) != 1 {
			_ = rows.Close()
			return fmt.Errorf("verify proxy %d password: legacy value does not match protected envelope", id)
		}
	}
	if err := closeSecretBridgeRows(rows); err != nil {
		return err
	}

	rows, err = db.QueryContext(ctx, `SELECT id,config,config_encrypted FROM payment_provider_instances WHERE config_encrypted IS NOT NULL`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id int64
		var legacy sql.NullString
		var envelope string
		if err := rows.Scan(&id, &legacy, &envelope); err != nil {
			_ = rows.Close()
			return err
		}
		plaintext, err := keyrings.DataEncryption.Decrypt(paymentProviderConfigPurpose, envelope)
		if err != nil {
			_ = rows.Close()
			return fmt.Errorf("verify payment provider instance %d config: %w", id, err)
		}
		// Empty config is the compatibility sentinel written alongside new
		// envelopes, not a second copy of the plaintext.
		if legacy.Valid && legacy.String != "" && subtle.ConstantTimeCompare([]byte(legacy.String), plaintext) != 1 {
			_ = rows.Close()
			return fmt.Errorf("verify payment provider instance %d config: legacy value does not match protected envelope", id)
		}
	}
	if err := closeSecretBridgeRows(rows); err != nil {
		return err
	}

	rows, err = db.QueryContext(ctx, `SELECT p.key,p.envelope,s.value FROM protected_settings p LEFT JOIN settings s ON s.key=p.key`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var key, envelope string
		var legacy sql.NullString
		if err := rows.Scan(&key, &envelope, &legacy); err != nil {
			_ = rows.Close()
			return err
		}
		switch sensitiveSettingRegistry[key] {
		case settingClassDataEnvelope:
			plaintext, err := keyrings.DataEncryption.Decrypt("settings."+key, envelope)
			if err != nil {
				_ = rows.Close()
				return fmt.Errorf("verify setting %q: %w", key, err)
			}
			if legacy.Valid && subtle.ConstantTimeCompare([]byte(legacy.String), plaintext) != 1 {
				_ = rows.Close()
				return fmt.Errorf("verify setting %q: legacy value does not match protected envelope", key)
			}
		case settingClassDigest:
			if !keyrings.GatewayKeyDigest.IsDigest(envelope) {
				_ = rows.Close()
				return fmt.Errorf("verify setting %q: invalid digest", key)
			}
			if legacy.Valid && !keyrings.GatewayKeyDigest.VerifyDigest(adminAPIKeyDigestPurpose, []byte(legacy.String), envelope) {
				_ = rows.Close()
				return fmt.Errorf("verify setting %q: legacy value does not match protected digest", key)
			}
		default:
			_ = rows.Close()
			return fmt.Errorf("protected setting %q is not registered", key)
		}
	}
	return closeSecretBridgeRows(rows)
}

func inspectSecretBridgeRemaining(ctx context.Context, db secretBridgeQuerier, stats *SecretBridgeMigrationStats) error {
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM proxies WHERE password IS NOT NULL AND password <> '' AND password_encrypted IS NULL`).Scan(&stats.LegacyProxiesRemaining); err != nil {
		return err
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM payment_provider_instances WHERE config <> '' AND config_encrypted IS NULL`).Scan(&stats.LegacyPaymentsRemaining); err != nil {
		return err
	}
	rows, err := db.QueryContext(ctx, `SELECT s.key FROM settings s WHERE NOT EXISTS (SELECT 1 FROM protected_settings p WHERE p.key=s.key)`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			_ = rows.Close()
			return err
		}
		class := sensitiveSettingRegistry[key]
		if class == settingClassDataEnvelope || class == settingClassDigest {
			stats.LegacySettingsRemaining++
		}
	}
	return closeSecretBridgeRows(rows)
}

func closeSecretBridgeRows(rows *sql.Rows) error {
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	return rows.Close()
}
