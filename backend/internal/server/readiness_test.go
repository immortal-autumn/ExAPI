package server

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/privatecutover"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestProbeReadiness(t *testing.T) {
	t.Run("ready", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		mock.ExpectPing()
		mock.ExpectQuery("SELECT EXISTS").
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
		expectPrivateRuntimeState(t, mock, true)

		redisServer := miniredis.RunT(t)
		redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
		t.Cleanup(func() { require.NoError(t, redisClient.Close()) })

		require.NoError(t, probeReadiness(context.Background(), db, redisClient))
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("database_unavailable", func(t *testing.T) {
		require.EqualError(t, probeReadiness(context.Background(), nil, nil), "database unavailable")
	})

	t.Run("database_ping_fails", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		mock.ExpectPing().WillReturnError(errors.New("postgres unavailable"))

		require.ErrorContains(t, probeReadiness(context.Background(), db, nil), "database ping")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("migration_table_missing", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		mock.ExpectPing()
		mock.ExpectQuery("SELECT EXISTS").WillReturnError(errors.New("relation does not exist"))

		require.ErrorContains(t, probeReadiness(context.Background(), db, nil), "schema migrations check")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("no_migration_applied", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		mock.ExpectPing()
		mock.ExpectQuery("SELECT EXISTS").
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

		require.EqualError(t, probeReadiness(context.Background(), db, nil), "schema migrations unavailable")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("redis_unavailable", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		mock.ExpectPing()
		mock.ExpectQuery("SELECT EXISTS").
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
		expectPrivateRuntimeState(t, mock, true)

		require.EqualError(t, probeReadiness(context.Background(), db, nil), "redis unavailable")
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("private_cutover_incomplete", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
		require.NoError(t, err)
		t.Cleanup(func() { _ = db.Close() })
		mock.ExpectPing()
		mock.ExpectQuery("SELECT EXISTS").
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
		expectPrivateRuntimeState(t, mock, false)

		require.ErrorContains(t, probeReadiness(context.Background(), db, nil), "private runtime state is incomplete")
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func expectPrivateRuntimeState(t *testing.T, mock sqlmock.Sqlmock, valid bool) {
	t.Helper()
	const schemaVersion = 2
	cutoverAt := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	report := privatecutover.MigrationReport{
		SchemaVersion: schemaVersion,
		OperatorID:    42,
		CutoverAt:     cutoverAt,
		DroppedTables: append([]string(nil), privatecutover.SaaSTables...),
		Confirmation:  privatecutover.ExpectedConfirmation(42),
		BatchCleanupEvidence: privatecutover.BatchCleanupEvidence{
			SchemaVersion:     privatecutover.BatchCleanupEvidenceSchemaVersion,
			Verified:          true,
			VerifiedAt:        cutoverAt,
			EvidenceURI:       "s3://audit.example/batch-cleanup.json",
			EvidenceVersionID: "version-1",
			EvidenceSHA256:    strings.Repeat("a", 64),
		},
	}
	signed, err := privatecutover.SignReport(report, []byte(strings.Repeat("k", 32)))
	require.NoError(t, err)
	var signedReport privatecutover.MigrationReport
	require.NoError(t, json.Unmarshal(signed, &signedReport))
	evidence, err := json.Marshal(struct {
		Report          privatecutover.MigrationReport `json:"report"`
		ReportKeySHA256 string                         `json:"report_key_sha256"`
	}{
		Report:          report,
		ReportKeySHA256: strings.Repeat("0", 64),
	})
	require.NoError(t, err)

	mock.ExpectQuery(`(?s)SELECT state\.private_schema_version.*FROM exapi_private_state`).
		WillReturnRows(sqlmock.NewRows([]string{
			"private_schema_version",
			"operator_id",
			"cutover_at",
			"report_sha256",
			"cutover_evidence",
			"valid",
		}).AddRow(schemaVersion, int64(42), cutoverAt, signedReport.ReportSHA256, evidence, valid))
}
