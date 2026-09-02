package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func scheduledPlanRows() *sqlmock.Rows {
	now := time.Date(2026, time.September, 2, 12, 0, 0, 0, time.UTC)
	lease := now.Add(10 * time.Minute)
	return sqlmock.NewRows([]string{
		"id", "account_id", "model_id", "cron_expression", "enabled", "max_results", "auto_recover",
		"last_run_at", "next_run_at", "created_at", "updated_at",
	}).AddRow(7, 42, "gpt-5", "* * * * *", true, 50, false, nil, lease, now, now)
}

func TestScheduledTestPlanRepositoryClaimDueAtomicallyLeasesPlans(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	now := time.Date(2026, time.September, 2, 12, 0, 0, 0, time.UTC)
	lease := now.Add(10 * time.Minute)
	mock.ExpectQuery(`WITH due AS \(`).
		WithArgs(now, lease, 10).
		WillReturnRows(scheduledPlanRows())

	repo := NewScheduledTestPlanRepository(db)
	plans, err := repo.ClaimDue(context.Background(), now, lease, 10)

	require.NoError(t, err)
	require.Len(t, plans, 1)
	require.Equal(t, int64(7), plans[0].ID)
	require.Equal(t, lease, plans[0].NextRunAt.UTC())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestScheduledTestPlanRepositoryCompleteClaimedRunRequiresLeaseOwnership(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	now := time.Date(2026, time.September, 2, 12, 0, 0, 0, time.UTC)
	lease := now.Add(10 * time.Minute)
	next := now.Add(time.Hour)
	mock.ExpectExec("UPDATE scheduled_test_plans").
		WithArgs(int64(7), now, next, lease).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("UPDATE scheduled_test_plans").
		WithArgs(int64(7), now, next, lease).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := NewScheduledTestPlanRepository(db)
	completed, err := repo.CompleteClaimedRun(context.Background(), 7, lease, now, next)
	require.NoError(t, err)
	require.False(t, completed)
	completed, err = repo.CompleteClaimedRun(context.Background(), 7, lease, now, next)
	require.NoError(t, err)
	require.True(t, completed)
	require.NoError(t, mock.ExpectationsWereMet())
}

// Keep the service import in this package-level test fixture tied to the
// repository contract; this catches accidental model drift at compile time.
var _ service.ScheduledTestPlanRunnerRepository = (*scheduledTestPlanRepository)(nil)
