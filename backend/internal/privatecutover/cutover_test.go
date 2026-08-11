package privatecutover

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

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
