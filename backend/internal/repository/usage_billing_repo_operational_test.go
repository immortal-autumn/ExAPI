package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestApplyUsageBillingEffectsAllowsDeletedTargetsOnlyForOperationalUsage(t *testing.T) {
	t.Run("operational usage updates retained tombstones", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		mock.ExpectBegin()
		tx, err := db.BeginTx(context.Background(), nil)
		require.NoError(t, err)

		mock.ExpectQuery(`UPDATE api_keys`).
			WithArgs(0.75, int64(42), service.StatusAPIKeyActive, service.StatusAPIKeyQuotaExhausted, true).
			WillReturnRows(sqlmock.NewRows([]string{"exhausted"}).AddRow(false))
		mock.ExpectExec(`UPDATE api_keys SET`).
			WithArgs(0.75, int64(42), true).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery(`UPDATE accounts SET extra`).
			WithArgs(1.5, int64(84), true).
			WillReturnRows(sqlmock.NewRows([]string{
				"total_used", "total_limit", "daily_used", "daily_limit", "weekly_used", "weekly_limit",
			}).AddRow(1.5, 100.0, 0.0, 0.0, 0.0, 0.0))

		cmd := &service.UsageBillingCommand{
			APIKeyID:            42,
			AccountID:           84,
			AccountType:         service.AccountTypeAPIKey,
			BillingType:         service.BillingTypeOperational,
			APIKeyQuotaCost:     0.75,
			APIKeyRateLimitCost: 0.75,
			AccountQuotaCost:    1.5,
		}
		err = (&usageBillingRepository{}).applyUsageBillingEffects(
			context.Background(), tx, cmd, &service.UsageBillingApplyResult{},
		)
		require.NoError(t, err)
		mock.ExpectRollback()
		require.NoError(t, tx.Rollback())
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("normal usage remains fail closed", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		mock.ExpectBegin()
		tx, err := db.BeginTx(context.Background(), nil)
		require.NoError(t, err)

		mock.ExpectQuery(`UPDATE api_keys`).
			WithArgs(0.25, int64(42), service.StatusAPIKeyActive, service.StatusAPIKeyQuotaExhausted, false).
			WillReturnRows(sqlmock.NewRows([]string{"exhausted"}))

		err = (&usageBillingRepository{}).applyUsageBillingEffects(
			context.Background(),
			tx,
			&service.UsageBillingCommand{APIKeyID: 42, APIKeyQuotaCost: 0.25},
			&service.UsageBillingApplyResult{},
		)
		require.ErrorIs(t, err, service.ErrAPIKeyNotFound)
		mock.ExpectRollback()
		require.NoError(t, tx.Rollback())
		require.NoError(t, mock.ExpectationsWereMet())
	})
}
