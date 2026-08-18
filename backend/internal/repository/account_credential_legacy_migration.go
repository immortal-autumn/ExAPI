package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

//nolint:unused // Used by integration-tag migration tests.
type legacyAccountCredentialMigrationStats struct {
	AccountsMigrated int64
}

// inspectLegacyAccountCredentialsInTx validates every active credential row and
// reports how many plaintext rows require migration without taking write locks
// or mutating data. Protected envelopes are authenticated as part of preflight.
//
//nolint:unused // Used by integration-tag migration tests.
func inspectLegacyAccountCredentialsInTx(ctx context.Context, tx *sql.Tx, protector *accountCredentialProtector) (stats legacyAccountCredentialMigrationStats, err error) {
	if tx == nil {
		return stats, errors.New("account credential migration transaction is required")
	}
	if protector == nil || protector.keyring == nil {
		return stats, errors.New("data-encryption keyring is required for account credential migration")
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT id, credentials::text
		FROM accounts
		WHERE deleted_at IS NULL
		ORDER BY id`)
	if err != nil {
		return stats, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id int64
		var raw []byte
		if err := rows.Scan(&id, &raw); err != nil {
			return stats, err
		}
		var stored map[string]any
		if err := json.Unmarshal(raw, &stored); err != nil {
			return stats, fmt.Errorf("decode account %d credentials: %w", id, err)
		}
		_, rewrite, err := protector.openLegacy(id, stored)
		if err != nil {
			return stats, fmt.Errorf("open account %d credentials: %w", id, err)
		}
		if rewrite {
			stats.AccountsMigrated++
		}
	}
	if err := rows.Err(); err != nil {
		return stats, err
	}
	return stats, nil
}

// migrateLegacyAccountCredentialsInTx rewrites plaintext account credential
// JSON into row-bound AEAD envelopes. A savepoint makes the invocation atomic
// while preserving the caller-owned transaction for disposable verification.
//
//nolint:unused // Used by integration-tag migration tests.
func migrateLegacyAccountCredentialsInTx(ctx context.Context, tx *sql.Tx, protector *accountCredentialProtector) (stats legacyAccountCredentialMigrationStats, err error) {
	if tx == nil {
		return stats, errors.New("account credential migration transaction is required")
	}
	if protector == nil || protector.keyring == nil {
		return stats, errors.New("data-encryption keyring is required for account credential migration")
	}
	const savepoint = "migrate_legacy_account_credentials"
	if _, err = tx.ExecContext(ctx, "SAVEPOINT "+savepoint); err != nil {
		return stats, err
	}
	defer func() {
		if err != nil {
			_, _ = tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT "+savepoint)
		}
		_, _ = tx.ExecContext(ctx, "RELEASE SAVEPOINT "+savepoint)
	}()

	rows, err := tx.QueryContext(ctx, `
		SELECT id, credentials::text
		FROM accounts
		WHERE deleted_at IS NULL
		ORDER BY id
		FOR UPDATE`)
	if err != nil {
		return stats, err
	}
	type row struct {
		id  int64
		raw []byte
	}
	var candidates []row
	for rows.Next() {
		var item row
		if err = rows.Scan(&item.id, &item.raw); err != nil {
			_ = rows.Close()
			return stats, err
		}
		candidates = append(candidates, item)
	}
	if err = rows.Close(); err != nil {
		return stats, err
	}
	if err = rows.Err(); err != nil {
		return stats, err
	}

	for _, item := range candidates {
		var stored map[string]any
		if err = json.Unmarshal(item.raw, &stored); err != nil {
			return stats, fmt.Errorf("decode account %d credentials: %w", item.id, err)
		}
		plaintext, rewrite, openErr := protector.openLegacy(item.id, stored)
		if openErr != nil {
			return stats, fmt.Errorf("open account %d credentials: %w", item.id, openErr)
		}
		if !rewrite {
			continue
		}
		sealed, sealErr := protector.seal(item.id, plaintext)
		if sealErr != nil {
			return stats, fmt.Errorf("seal account %d credentials: %w", item.id, sealErr)
		}
		encoded, marshalErr := json.Marshal(sealed)
		if marshalErr != nil {
			return stats, marshalErr
		}
		result, updateErr := tx.ExecContext(ctx, `
			UPDATE accounts
			SET credentials = $1::jsonb, updated_at = NOW()
			WHERE id = $2 AND credentials = $3::jsonb`, encoded, item.id, item.raw)
		if updateErr != nil {
			return stats, updateErr
		}
		affected, rowsErr := result.RowsAffected()
		if rowsErr != nil {
			return stats, rowsErr
		}
		if affected != 1 {
			return stats, fmt.Errorf("account %d credentials changed during migration", item.id)
		}
		stats.AccountsMigrated++
	}
	return stats, nil
}
