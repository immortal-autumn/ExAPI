package privatecutover

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestVerifyMigrationReportChecksSignatureAndDurableState(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	key := []byte(strings.Repeat("k", 32))
	cutoverAt := time.Date(2026, 8, 16, 12, 0, 0, 123456000, time.UTC)
	baseReport := MigrationReport{
		SchemaVersion:        privateSchemaVersion,
		OperatorID:           42,
		CutoverAt:            cutoverAt,
		DroppedTables:        append([]string(nil), SaaSTables...),
		Confirmation:         ExpectedConfirmation(42),
		PurgedSettings:       []string{},
		BatchCleanupEvidence: validBatchCleanupEvidence(cutoverAt),
	}
	signed, err := SignReport(baseReport, key)
	require.NoError(t, err)
	var signedReport MigrationReport
	require.NoError(t, json.Unmarshal(signed, &signedReport))
	evidenceJSON, err := json.Marshal(cutoverEvidence{Report: baseReport, ReportKeySHA256: reportKeySHA256(key)})
	require.NoError(t, err)

	mock.ExpectQuery(`SELECT private_schema_version, operator_id, cutover_at, report_sha256, cutover_evidence FROM exapi_private_state WHERE id = true`).
		WillReturnRows(sqlmock.NewRows([]string{"private_schema_version", "operator_id", "cutover_at", "report_sha256", "cutover_evidence"}).
			AddRow(privateSchemaVersion, int64(42), cutoverAt, signedReport.ReportSHA256, evidenceJSON))
	mock.ExpectQuery(`(?s)SELECT state.private_schema_version.*FROM exapi_private_state`).
		WillReturnRows(sqlmock.NewRows([]string{"private_schema_version", "operator_id", "cutover_at", "report_sha256", "cutover_evidence", "valid"}).
			AddRow(privateSchemaVersion, int64(42), cutoverAt, signedReport.ReportSHA256, evidenceJSON, true))

	verified, err := VerifyMigrationReport(context.Background(), db, signed, key)
	require.NoError(t, err)
	require.Equal(t, int64(42), verified.OperatorID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestVerifyMigrationReportRejectsTamperingAndUnknownFields(t *testing.T) {
	key := []byte(strings.Repeat("k", 32))
	cutoverAt := time.Now().UTC().Truncate(time.Microsecond)
	signed, err := SignReport(MigrationReport{
		SchemaVersion:        privateSchemaVersion,
		OperatorID:           42,
		CutoverAt:            cutoverAt,
		DroppedTables:        append([]string(nil), SaaSTables...),
		Confirmation:         ExpectedConfirmation(42),
		BatchCleanupEvidence: validBatchCleanupEvidence(cutoverAt),
	}, key)
	require.NoError(t, err)

	db, _, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	tampered := bytes.Replace(signed, []byte(`"operator_id":42`), []byte(`"operator_id":43`), 1)
	_, err = VerifyMigrationReport(context.Background(), db, tampered, key)
	require.ErrorContains(t, err, "confirmation is invalid")

	unknown := append(append([]byte(nil), signed[:len(signed)-1]...), []byte(`,"unknown":true}`)...)
	_, err = VerifyMigrationReport(context.Background(), db, unknown, key)
	require.ErrorContains(t, err, "unknown field")
}

func TestVerifyMigrationReportRejectsEvidenceMismatch(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	key := []byte(strings.Repeat("k", 32))
	cutoverAt := time.Date(2026, 8, 16, 12, 0, 0, 123456000, time.UTC)
	report := MigrationReport{
		SchemaVersion:        privateSchemaVersion,
		OperatorID:           42,
		CutoverAt:            cutoverAt,
		DeletedUsers:         7,
		DroppedTables:        append([]string(nil), SaaSTables...),
		Confirmation:         ExpectedConfirmation(42),
		BatchCleanupEvidence: validBatchCleanupEvidence(cutoverAt),
	}
	signed, err := SignReport(report, key)
	require.NoError(t, err)
	var signedReport MigrationReport
	require.NoError(t, json.Unmarshal(signed, &signedReport))
	report.DeletedUsers = 0
	evidenceJSON, err := json.Marshal(cutoverEvidence{Report: report, ReportKeySHA256: reportKeySHA256(key)})
	require.NoError(t, err)

	mock.ExpectQuery(`SELECT private_schema_version, operator_id, cutover_at, report_sha256, cutover_evidence FROM exapi_private_state WHERE id = true`).
		WillReturnRows(sqlmock.NewRows([]string{"private_schema_version", "operator_id", "cutover_at", "report_sha256", "cutover_evidence"}).
			AddRow(privateSchemaVersion, int64(42), cutoverAt, signedReport.ReportSHA256, evidenceJSON))

	_, err = VerifyMigrationReport(context.Background(), db, signed, key)
	require.ErrorContains(t, err, "preserved cutover evidence")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestVerifyMigrationReportRejectsInvalidBackupEvidence(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	key := []byte(strings.Repeat("k", 32))
	cutoverAt := time.Date(2026, 8, 16, 12, 0, 0, 123456000, time.UTC)
	report := MigrationReport{
		SchemaVersion:        privateSchemaVersion,
		OperatorID:           42,
		CutoverAt:            cutoverAt,
		DroppedTables:        append([]string(nil), SaaSTables...),
		PurgedLocalBackups:   []string{"/backups/not-a-candidate.sql"},
		Confirmation:         ExpectedConfirmation(42),
		BatchCleanupEvidence: validBatchCleanupEvidence(cutoverAt),
	}
	signed, err := SignReport(report, key)
	require.NoError(t, err)
	var signedReport MigrationReport
	require.NoError(t, json.Unmarshal(signed, &signedReport))
	evidenceJSON, err := json.Marshal(cutoverEvidence{Report: report, ReportKeySHA256: reportKeySHA256(key)})
	require.NoError(t, err)

	mock.ExpectQuery(`SELECT private_schema_version, operator_id, cutover_at, report_sha256, cutover_evidence FROM exapi_private_state WHERE id = true`).
		WillReturnRows(sqlmock.NewRows([]string{"private_schema_version", "operator_id", "cutover_at", "report_sha256", "cutover_evidence"}).
			AddRow(privateSchemaVersion, int64(42), cutoverAt, signedReport.ReportSHA256, evidenceJSON))

	_, err = VerifyMigrationReport(context.Background(), db, signed, key)
	require.ErrorContains(t, err, "backup evidence is invalid")
	require.NoError(t, mock.ExpectationsWereMet())
}
