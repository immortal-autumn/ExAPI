package repository

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	dbent "github.com/Wei-Shaw/sub2api/ent"
)

// withLockedCredentialMatch runs a credential-dependent mutation while holding
// a row lock. Account credentials are encrypted with randomized nonces, so a
// plaintext JSON document cannot be compared safely or meaningfully in SQL.
// The row is instead locked, decrypted, compared in process, and mutated in the
// same database transaction. A successful callback is committed together with
// any durable outbox write it performs.
func (r *accountRepository) withLockedCredentialMatch(
	ctx context.Context,
	id int64,
	expectedCredentials map[string]any,
	mutate func(exec sqlExecutor) (bool, error),
) (bool, error) {
	if r == nil || r.client == nil || r.sql == nil {
		return false, errors.New("account repository transaction dependencies are not configured")
	}
	if r.protector == nil {
		return false, errors.New("data-encryption keyring is required for account credential comparison")
	}

	tx, err := r.client.Tx(ctx)
	if err != nil && !errors.Is(err, dbent.ErrTxStarted) {
		return false, err
	}
	exec := r.sql
	if tx != nil {
		defer func() { _ = tx.Rollback() }()
		exec = tx.Client()
	}

	storedCredentials, found, err := loadLockedAccountCredentials(ctx, exec, id)
	if err != nil || !found {
		return false, err
	}
	currentCredentials, _, err := r.protector.openLegacy(id, storedCredentials)
	if err != nil {
		return false, err
	}
	equal, err := normalizedCredentialMapsEqual(currentCredentials, expectedCredentials)
	if err != nil || !equal {
		return false, err
	}

	applied, err := mutate(exec)
	if err != nil || !applied {
		return false, err
	}
	if tx != nil {
		if err := tx.Commit(); err != nil {
			return false, err
		}
	}
	return true, nil
}

func loadLockedAccountCredentials(ctx context.Context, exec sqlExecutor, id int64) (map[string]any, bool, error) {
	rows, err := exec.QueryContext(ctx, `
		SELECT credentials
		FROM accounts
		WHERE id = $1 AND deleted_at IS NULL
		FOR UPDATE
	`, id)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, false, err
		}
		return nil, false, nil
	}
	var encoded []byte
	if err := rows.Scan(&encoded); err != nil {
		return nil, false, err
	}
	if rows.Next() {
		return nil, false, fmt.Errorf("multiple account rows found for id %d", id)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	var stored map[string]any
	if len(encoded) == 0 {
		stored = map[string]any{}
	} else if err := json.Unmarshal(encoded, &stored); err != nil {
		return nil, false, errors.New("invalid stored account credential JSON")
	}
	if stored == nil {
		stored = map[string]any{}
	}
	return stored, true, nil
}

func normalizedCredentialMapsEqual(left, right map[string]any) (bool, error) {
	leftJSON, err := json.Marshal(normalizeJSONMap(left))
	if err != nil {
		return false, err
	}
	rightJSON, err := json.Marshal(normalizeJSONMap(right))
	if err != nil {
		return false, err
	}
	return bytes.Equal(leftJSON, rightJSON), nil
}

func credentialSnapshotJSON(raw string) (map[string]any, error) {
	var credentials map[string]any
	if err := json.Unmarshal([]byte(raw), &credentials); err != nil {
		return nil, fmt.Errorf("decode credential mutation snapshot: %w", err)
	}
	if credentials == nil {
		credentials = map[string]any{}
	}
	return credentials, nil
}

func rowsAffected(result sql.Result) (bool, error) {
	count, err := result.RowsAffected()
	return count > 0, err
}
