package privatecutover

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

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
