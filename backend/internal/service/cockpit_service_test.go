package service

import (
	"context"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestCockpitServiceReturnsBoundedAuthoritativeSummary(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery(`SELECT platform,`).WillReturnRows(sqlmock.NewRows([]string{
		"platform", "total", "active", "inactive", "error", "dispatch_eligible",
	}).AddRow("gemini", int64(3), int64(2), int64(0), int64(1), int64(1)).
		AddRow("openai", int64(7), int64(5), int64(1), int64(1), int64(3)))
	mock.ExpectQuery(`WITH windows AS`).WithArgs(cockpitWarningLimit).WillReturnRows(sqlmock.NewRows([]string{
		"id", "name", "platform", "scope", "used", "limit", "percent", "severity", "warning_total",
	}).AddRow(int64(8), "critical", "openai", "weekly", 95.0, 100.0, 95.0, "critical", int64(9)).
		AddRow(int64(4), "warning", "gemini", "daily", 7.5, 10.0, 75.0, "warning", int64(9)))

	summary, err := NewCockpitService(db).GetSummary(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(10), summary.Accounts.Total)
	require.Equal(t, int64(7), summary.Accounts.Active)
	require.Equal(t, int64(1), summary.Accounts.Inactive)
	require.Equal(t, int64(2), summary.Accounts.Error)
	require.Equal(t, int64(4), summary.Accounts.DispatchEligible)
	require.Equal(t, int64(9), summary.Accounts.QuotaWarningTotal)
	require.Len(t, summary.Platforms, 2)
	require.Len(t, summary.QuotaWarnings, 2)
	require.Equal(t, "critical", summary.QuotaWarnings[0].Severity)
	require.Equal(t, "weekly", summary.QuotaWarnings[0].Scope)
	require.False(t, summary.GeneratedAt.IsZero())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCockpitQueriesEncodeSchedulingAndWarningContract(t *testing.T) {
	counts := cockpitCountsQuery()
	require.Contains(t, counts, "status='active' AND schedulable")
	require.Contains(t, counts, "NOT auto_pause_on_expired OR expires_at IS NULL OR expires_at > NOW()")
	require.Contains(t, counts, "rate_limit_reset_at IS NULL OR rate_limit_reset_at <= NOW()")
	require.Contains(t, counts, "temp_unschedulable_until IS NULL OR temp_unschedulable_until <= NOW()")
	require.Contains(t, counts, cockpitQuotaAccountTypes)
	require.Contains(t, counts, cockpitDailyWindowActive)
	require.Contains(t, counts, cockpitWeeklyWindowActive)
	warnings := cockpitWarningsQuery()
	require.Contains(t, warnings, ">= 0.70")
	require.Contains(t, warnings, "percent >= 90")
	require.Contains(t, warnings, "ROW_NUMBER() OVER (PARTITION BY id")
	require.Contains(t, warnings, "ORDER BY percent DESC, id ASC")
	require.Contains(t, warnings, "LIMIT $1")
	require.Equal(t, 3, strings.Count(warnings, cockpitQuotaAccountTypes))
	require.Contains(t, warnings, cockpitDailyWindowActive)
	require.Contains(t, warnings, cockpitWeeklyWindowActive)
}
