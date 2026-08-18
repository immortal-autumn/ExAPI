package repository

import (
	"context"
	"database/sql"
	"fmt"
)

//nolint:unused // Used by integration-tag migration tests.
type legacyGatewayKeyMigrationStats struct {
	APIKeysMigrated       int64
	DeletedAuditsMigrated int64
}

// inspectLegacyGatewayKeyMaterialInTx validates legacy rows and reports the
// number that require migration without locking or mutating them.
//
//nolint:unused // Used by integration-tag migration tests.
func inspectLegacyGatewayKeyMaterialInTx(ctx context.Context, tx *sql.Tx, digester *gatewayAPIKeyDigester) (legacyGatewayKeyMigrationStats, error) {
	var stats legacyGatewayKeyMigrationStats
	if tx == nil {
		return stats, fmt.Errorf("nil migration transaction")
	}
	if digester == nil {
		return stats, errGatewayAPIKeyDigestRequired
	}
	rows, err := tx.QueryContext(ctx, `SELECT key FROM api_keys WHERE key_digest IS NULL ORDER BY id`)
	if err != nil {
		return stats, err
	}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			_ = rows.Close()
			return stats, err
		}
		if _, err := digester.Digest(raw); err != nil {
			_ = rows.Close()
			return stats, fmt.Errorf("validate legacy API key: %w", err)
		}
		stats.APIKeysMigrated++
	}
	if err := rows.Close(); err != nil {
		return stats, err
	}
	if err := rows.Err(); err != nil {
		return stats, err
	}
	auditRows, err := tx.QueryContext(ctx, `SELECT key FROM deleted_api_key_audits WHERE key_digest IS NULL ORDER BY id`)
	if err != nil {
		return stats, err
	}
	for auditRows.Next() {
		var raw string
		if err := auditRows.Scan(&raw); err != nil {
			_ = auditRows.Close()
			return stats, err
		}
		if _, err := digester.Digest(raw); err != nil {
			_ = auditRows.Close()
			return stats, fmt.Errorf("validate legacy deleted-key audit: %w", err)
		}
		stats.DeletedAuditsMigrated++
	}
	if err := auditRows.Close(); err != nil {
		return stats, err
	}
	if err := auditRows.Err(); err != nil {
		return stats, err
	}
	return stats, nil
}

// migrateLegacyGatewayKeyMaterial protects an existing transaction with a
// savepoint. Any malformed row or conflict rolls back all rewrites made by this
// invocation while leaving the caller able to inspect or continue the outer
// disposable transaction.
//
//nolint:unused // Used by integration-tag migration tests.
func migrateLegacyGatewayKeyMaterial(ctx context.Context, tx *sql.Tx, digester *gatewayAPIKeyDigester) error {
	if tx == nil {
		return fmt.Errorf("nil migration transaction")
	}
	const savepoint = "migrate_legacy_gateway_keys"
	if _, err := tx.ExecContext(ctx, "SAVEPOINT "+savepoint); err != nil {
		return err
	}
	if _, err := migrateLegacyGatewayKeyMaterialInTx(ctx, tx, digester); err != nil {
		if _, rollbackErr := tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT "+savepoint); rollbackErr != nil {
			return fmt.Errorf("%w; rollback migration savepoint: %v", err, rollbackErr)
		}
		_, _ = tx.ExecContext(ctx, "RELEASE SAVEPOINT "+savepoint)
		return err
	}
	_, err := tx.ExecContext(ctx, "RELEASE SAVEPOINT "+savepoint)
	return err
}

//nolint:unused // Used by integration-tag migration tests.
func migrateLegacyGatewayKeyMaterialInTx(ctx context.Context, tx *sql.Tx, digester *gatewayAPIKeyDigester) (legacyGatewayKeyMigrationStats, error) {
	var stats legacyGatewayKeyMigrationStats
	if tx == nil {
		return stats, fmt.Errorf("nil migration transaction")
	}
	if digester == nil {
		return stats, errGatewayAPIKeyDigestRequired
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT id, key
		FROM api_keys
		WHERE key_digest IS NULL
		ORDER BY id
		FOR UPDATE`)
	if err != nil {
		return stats, err
	}
	type legacyRow struct {
		id  int64
		raw string
	}
	var apiKeys []legacyRow
	for rows.Next() {
		var row legacyRow
		if err := rows.Scan(&row.id, &row.raw); err != nil {
			_ = rows.Close()
			return stats, err
		}
		apiKeys = append(apiKeys, row)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return stats, fmt.Errorf("iterate legacy API keys: %w", err)
	}
	if err := rows.Close(); err != nil {
		return stats, err
	}

	for _, row := range apiKeys {
		digest, err := digester.Digest(row.raw)
		if err != nil {
			return stats, fmt.Errorf("digest legacy API key id %d: %w", row.id, err)
		}
		result, err := tx.ExecContext(ctx, `
			UPDATE api_keys
			SET key = $1, key_digest = $2, key_prefix = $3, updated_at = NOW()
			WHERE id = $4 AND key_digest IS NULL`,
			gatewayAPIKeyStoragePlaceholder(digest), digest, gatewayAPIKeyDisplayPrefix(row.raw), row.id)
		if err != nil {
			return stats, fmt.Errorf("rewrite legacy API key id %d: %w", row.id, err)
		}
		affected, err := result.RowsAffected()
		if err != nil || affected != 1 {
			if err != nil {
				return stats, err
			}
			return stats, fmt.Errorf("rewrite legacy API key id %d affected %d rows", row.id, affected)
		}
		stats.APIKeysMigrated++
	}

	auditRows, err := tx.QueryContext(ctx, `
		SELECT id, key
		FROM deleted_api_key_audits
		WHERE key_digest IS NULL
		ORDER BY id
		FOR UPDATE`)
	if err != nil {
		return stats, err
	}
	var audits []legacyRow
	for auditRows.Next() {
		var row legacyRow
		if err := auditRows.Scan(&row.id, &row.raw); err != nil {
			_ = auditRows.Close()
			return stats, err
		}
		audits = append(audits, row)
	}
	if err := auditRows.Err(); err != nil {
		_ = auditRows.Close()
		return stats, fmt.Errorf("iterate legacy deleted-key audits: %w", err)
	}
	if err := auditRows.Close(); err != nil {
		return stats, err
	}

	for _, row := range audits {
		digest, err := digester.Digest(row.raw)
		if err != nil {
			return stats, fmt.Errorf("digest legacy deleted-key audit id %d: %w", row.id, err)
		}
		result, err := tx.ExecContext(ctx, `
			UPDATE deleted_api_key_audits
			SET key = $1, key_digest = $2
			WHERE id = $3 AND key_digest IS NULL`,
			gatewayAPIKeyStoragePlaceholder(digest), digest, row.id)
		if err != nil {
			return stats, fmt.Errorf("rewrite legacy deleted-key audit id %d: %w", row.id, err)
		}
		affected, err := result.RowsAffected()
		if err != nil || affected != 1 {
			if err != nil {
				return stats, err
			}
			return stats, fmt.Errorf("rewrite legacy deleted-key audit id %d affected %d rows", row.id, affected)
		}
		stats.DeletedAuditsMigrated++
	}
	return stats, nil
}
