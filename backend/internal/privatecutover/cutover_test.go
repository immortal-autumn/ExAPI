package privatecutover

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestVerifyRuntimeStateFailsClosedAndAcceptsCompleteState(t *testing.T) {
	t.Run("nil database", func(t *testing.T) {
		require.ErrorContains(t, VerifyRuntimeState(context.Background(), nil), "database is required")
	})

	for _, tc := range []struct {
		name      string
		valid     bool
		queryErr  error
		wantError string
	}{
		{name: "complete", valid: true},
		{name: "incomplete", valid: false, wantError: "private runtime state is incomplete"},
		{name: "query failure", queryErr: errors.New("state table unavailable"), wantError: "query private runtime state"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })

			row := mock.ExpectQuery(`(?s)SELECT EXISTS .*COUNT\(\*\) FROM users\) = 1`).WithArgs(privateSchemaVersion)
			if tc.queryErr != nil {
				row.WillReturnError(tc.queryErr)
			} else {
				row.WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(tc.valid))
			}

			err = VerifyRuntimeState(context.Background(), db)
			if tc.wantError == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.wantError)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestExpectedAndParseConfirmation(t *testing.T) {
	expected := ExpectedConfirmation(42)
	require.Equal(t, "DROP-SAAS-DATA-KEEP-USER-42", expected)
	operatorID, err := ParseConfirmation(expected)
	require.NoError(t, err)
	require.Equal(t, int64(42), operatorID)
}

func TestRecordReportDigestPersistsSignedReportHash(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	digest := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	mock.ExpectExec(`UPDATE exapi_private_state SET report_sha256 = \$1 WHERE id = true`).
		WithArgs(digest).
		WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, recordReportDigest(context.Background(), db, digest))
	require.NoError(t, mock.ExpectationsWereMet())
	require.Error(t, recordReportDigest(context.Background(), db, ""))
}

func TestRecordReportDigestRejectsMissingStateRow(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	digest := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	mock.ExpectExec(`UPDATE exapi_private_state SET report_sha256 = \$1 WHERE id = true`).
		WithArgs(digest).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err = recordReportDigest(context.Background(), db, digest)
	require.ErrorContains(t, err, "expected one private state row")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCutoverCommandLockSpansCallerWorkAndReleases(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectExec(`SELECT pg_advisory_lock\(hashtext\(\$1\)\)`).
		WithArgs(commandAdvisoryLockKey).
		WillDelayFor(25 * time.Millisecond).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE report_and_digest_work`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT pg_advisory_unlock\(hashtext\(\$1\)\)`).
		WithArgs(commandAdvisoryLockKey).
		WillReturnRows(sqlmock.NewRows([]string{"unlocked"}).AddRow(true))

	workCalled := false
	startedAt := time.Now()
	var sampledAt time.Time
	report, err := withCommandLock(context.Background(), db, func(conn *sql.Conn) (MigrationReport, error) {
		workCalled = true
		sampledAt = time.Now()
		_, err := conn.ExecContext(context.Background(), `UPDATE report_and_digest_work`)
		return MigrationReport{OperatorID: 42}, err
	})
	require.NoError(t, err)
	require.True(t, workCalled)
	require.GreaterOrEqual(t, sampledAt.Sub(startedAt), 20*time.Millisecond, "locked work must sample time after acquisition")
	require.Equal(t, int64(42), report.OperatorID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCutoverCommandLockDiscardsSessionWhenUnlockIsUncertain(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectExec(`SELECT pg_advisory_lock\(hashtext\(\$1\)\)`).
		WithArgs(commandAdvisoryLockKey).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT pg_advisory_unlock\(hashtext\(\$1\)\)`).
		WithArgs(commandAdvisoryLockKey).
		WillReturnError(errors.New("connection lost"))

	report, err := withCommandLock(context.Background(), db, func(*sql.Conn) (MigrationReport, error) {
		return MigrationReport{OperatorID: 42}, nil
	})
	require.Empty(t, report)
	require.ErrorContains(t, err, "release cutover command lock")
	require.Zero(t, db.Stats().OpenConnections, "uncertain lock session must not return to the pool")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCutoverCommandLockFailsClosed(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectExec(`SELECT pg_advisory_lock\(hashtext\(\$1\)\)`).
		WithArgs(commandAdvisoryLockKey).
		WillReturnError(errors.New("lock unavailable"))

	workCalled := false
	_, err = withCommandLock(context.Background(), db, func(*sql.Conn) (MigrationReport, error) {
		workCalled = true
		return MigrationReport{}, nil
	})
	require.False(t, workCalled)
	require.ErrorContains(t, err, "acquire cutover command lock")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPrivateStateUpsertClearsStaleReportDigest(t *testing.T) {
	sql := strings.Join(strings.Fields(privateStateUpsertSQL), " ")
	require.Contains(t, sql, "ON CONFLICT (id) DO UPDATE")
	require.Contains(t, sql, "report_sha256 = ''")
}

func TestWriteDurableReportInstallsProtectedFile(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "private-cutover-report.json")
	require.NoError(t, os.WriteFile(path, []byte("stale"), 0o644))

	require.NoError(t, writeDurableReport(path, nil, []byte(`{"ok":true}`)))
	contents, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, `{"ok":true}`, string(contents))
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	matches, err := filepath.Glob(filepath.Join(directory, ".private-cutover-report.json.tmp-*"))
	require.NoError(t, err)
	require.Empty(t, matches)
}

func TestWriteDurableReportCompatibilityOutput(t *testing.T) {
	var output bytes.Buffer
	require.NoError(t, writeDurableReport("", &output, []byte("report")))
	require.Equal(t, "report", output.String())
	require.ErrorContains(t, writeDurableReport("", nil, []byte("report")), "output is required")
}

func TestValidateReportOptionsProbesDestinationBeforeCutover(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "private-cutover-report.json")
	require.NoError(t, validateReportOptions(CutoverOptions{ReportPath: path}))

	matches, err := filepath.Glob(filepath.Join(directory, ".private-cutover-report.json.preflight-*"))
	require.NoError(t, err)
	require.Empty(t, matches)

	require.ErrorContains(t,
		validateReportOptions(CutoverOptions{ReportPath: directory}),
		"is a directory",
	)
}

func TestValidateReportOptionsRejectsUnwritableDestination(t *testing.T) {
	directory := t.TempDir()
	require.NoError(t, os.Chmod(directory, 0o500))
	t.Cleanup(func() { _ = os.Chmod(directory, 0o700) })

	err := validateReportOptions(CutoverOptions{
		ReportPath: filepath.Join(directory, "private-cutover-report.json"),
	})
	if err == nil {
		t.Skip("current user can write through directory permissions")
	}
	require.ErrorContains(t, err, "probe migration report destination")
}

func TestValidateBackupOptionsRequiresExactlyOneOperatorChoice(t *testing.T) {
	directory := t.TempDir()

	require.NoError(t, validateBackupOptions(CutoverOptions{BackupDir: directory}))
	require.NoError(t, validateBackupOptions(CutoverOptions{AssertNoManagedBackups: true}))
	require.ErrorContains(t, validateBackupOptions(CutoverOptions{}), "explicit --backup-dir is required")
	require.ErrorContains(t, validateBackupOptions(CutoverOptions{
		BackupDir:              directory,
		AssertNoManagedBackups: true,
	}), "mutually exclusive")
	require.ErrorContains(t, validateBackupOptions(CutoverOptions{
		BackupDir: filepath.Join(directory, "missing"),
	}), "validate backup directory")
}

func TestParseConfirmationRejectsNearMisses(t *testing.T) {
	for _, value := range []string{
		"",
		"DROP-SAAS-DATA-KEEP-USER-",
		"DROP-SAAS-DATA-KEEP-USER-0",
		"DROP-SAAS-DATA-KEEP-USER-1-extra",
		"KEEP-USER-1",
	} {
		_, err := ParseConfirmation(value)
		require.Error(t, err, value)
	}
}

func TestSignReportRequiresKeyAndIncludesIntegrityFields(t *testing.T) {
	_, err := SignReport(MigrationReport{OperatorID: 42}, []byte("short"))
	require.Error(t, err)

	signed, err := SignReport(MigrationReport{SchemaVersion: 1, OperatorID: 42}, []byte("01234567890123456789012345678901"))
	require.NoError(t, err)
	require.Contains(t, string(signed), `"report_sha256"`)
	require.Contains(t, string(signed), `"report_hmac_sha256"`)
}
