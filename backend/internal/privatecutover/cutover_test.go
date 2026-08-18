package privatecutover

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

type exactStringArgument string

func (expected exactStringArgument) Match(value driver.Value) bool {
	actual, ok := value.(string)
	return ok && actual == string(expected)
}

type fileInfoWithoutLinkCount struct {
	os.FileInfo
}

func (fileInfoWithoutLinkCount) Sys() any {
	return struct{}{}
}

type cutoverEvidenceRecorder struct {
	calls     int
	failAt    int
	persisted []cutoverEvidence
}

func (recorder *cutoverEvidenceRecorder) ExecContext(_ context.Context, _ string, args ...any) (sql.Result, error) {
	recorder.calls++
	if recorder.calls == recorder.failAt {
		return nil, errors.New("simulated evidence persistence failure")
	}
	if len(args) != 1 {
		return nil, fmt.Errorf("unexpected evidence argument count: %d", len(args))
	}
	raw, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("unexpected evidence argument type: %T", args[0])
	}
	var evidence cutoverEvidence
	if err := json.Unmarshal([]byte(raw), &evidence); err != nil {
		return nil, err
	}
	recorder.persisted = append(recorder.persisted, evidence)
	return sqlmock.NewResult(0, 1), nil
}

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

			cutoverAt := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
			report := MigrationReport{
				SchemaVersion:        privateSchemaVersion,
				OperatorID:           42,
				CutoverAt:            cutoverAt,
				DroppedTables:        append([]string(nil), SaaSTables...),
				Confirmation:         ExpectedConfirmation(42),
				BatchCleanupEvidence: validBatchCleanupEvidence(cutoverAt),
			}
			unsigned, encodeErr := unsignedReportPayload(report)
			require.NoError(t, encodeErr)
			digest := sha256.Sum256(unsigned)
			evidenceJSON, encodeErr := json.Marshal(cutoverEvidence{Report: report, ReportKeySHA256: reportKeySHA256([]byte(strings.Repeat("k", 32)))})
			require.NoError(t, encodeErr)
			row := mock.ExpectQuery(`(?s)SELECT state.private_schema_version.*FROM exapi_private_state`)
			if tc.queryErr != nil {
				row.WillReturnError(tc.queryErr)
			} else {
				row.WillReturnRows(sqlmock.NewRows([]string{"private_schema_version", "operator_id", "cutover_at", "report_sha256", "cutover_evidence", "valid"}).
					AddRow(privateSchemaVersion, int64(42), cutoverAt, hex.EncodeToString(digest[:]), evidenceJSON, tc.valid))
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

func TestAssertNoBatchImageRowsFailsClosed(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM batch_image_jobs`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	mock.ExpectRollback()

	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	err = assertNoBatchImageRows(context.Background(), tx)
	require.ErrorContains(t, err, "2 job(s) remain")
	require.NoError(t, tx.Rollback())
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

func TestPrivateStateUpsertPreservesCommittedEvidence(t *testing.T) {
	sql := strings.Join(strings.Fields(privateStateUpsertSQL), " ")
	require.Contains(t, sql, "cutover_evidence")
	require.Contains(t, sql, "ON CONFLICT (id) DO NOTHING")
}

func TestInsertCutoverEvidencePersistsOriginalReportAsJSONBString(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	cutoverAt := time.Date(2026, 8, 16, 12, 0, 0, 123456000, time.UTC)
	evidence := cutoverEvidence{
		Report: MigrationReport{
			SchemaVersion:           privateSchemaVersion,
			OperatorID:              42,
			CutoverAt:               cutoverAt,
			DeletedUsers:            7,
			DroppedTables:           append([]string(nil), SaaSTables...),
			ManagedBackupsPreserved: 3,
			PurgedSettings:          []string{"payment_enabled"},
			PurgedProtected:         []string{"admin_api_key"},
			Confirmation:            ExpectedConfirmation(42),
		},
		ReportKeySHA256:       reportKeySHA256([]byte(strings.Repeat("k", 32))),
		LocalBackupCandidates: []string{"/backups/pre-cutover.sql"},
		LocalBackupRoot:       &backupRootIdentity{Path: "/backups", Device: 1, Inode: 1},
	}
	evidenceJSON, err := json.Marshal(evidence)
	require.NoError(t, err)
	mock.ExpectExec(`INSERT INTO exapi_private_state`).
		WithArgs(privateSchemaVersion, int64(42), cutoverAt, exactStringArgument(evidenceJSON)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, insertCutoverEvidence(context.Background(), db, evidence))
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

func TestValidateReportOutsideLocalBackupRootRejectsNestedDestination(t *testing.T) {
	root := t.TempDir()
	backupDirectory := filepath.Join(root, "backups")
	require.NoError(t, os.Mkdir(backupDirectory, 0o700))
	require.ErrorContains(t, validateReportOutsideLocalBackupRoot(CutoverOptions{
		LocalBackupDir: backupDirectory,
		ReportPath:     filepath.Join(backupDirectory, "private-cutover-report.json"),
	}), "outside the legacy local backup root")
	require.NoError(t, validateReportOutsideLocalBackupRoot(CutoverOptions{
		LocalBackupDir: backupDirectory,
		ReportPath:     filepath.Join(root, "private-cutover-report.json"),
	}))
}

func TestValidateBackupOptionsRequiresExactlyOneOperatorChoice(t *testing.T) {
	directory := t.TempDir()

	require.NoError(t, validateBackupOptions(CutoverOptions{LocalBackupDir: directory}))
	require.NoError(t, validateBackupOptions(CutoverOptions{AssertNoLocalBackups: true}))
	require.ErrorContains(t, validateBackupOptions(CutoverOptions{}), "explicit --local-backup-dir is required")
	require.ErrorContains(t, validateBackupOptions(CutoverOptions{
		LocalBackupDir:       directory,
		AssertNoLocalBackups: true,
	}), "mutually exclusive")
	require.ErrorContains(t, validateBackupOptions(CutoverOptions{
		LocalBackupDir: filepath.Join(directory, "missing"),
	}), "validate backup directory")
	require.NoError(t, validateBackupChoice(CutoverOptions{
		LocalBackupDir: filepath.Join(directory, "missing"),
	}), "a committed retry must not require the original backup mount")
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

func TestSnapshotPreCutoverBackupsCanonicalizesSymlinkedRoot(t *testing.T) {
	directory := t.TempDir()
	candidate := filepath.Join(directory, "pre-cutover.sql")
	require.NoError(t, os.WriteFile(candidate, []byte("backup"), 0o600))
	cutoverAt := time.Now().UTC().Add(time.Minute)

	link := filepath.Join(t.TempDir(), "backups")
	require.NoError(t, os.Symlink(directory, link))
	root, candidates, err := snapshotPreCutoverBackups(link, cutoverAt)
	require.NoError(t, err)
	require.Equal(t, directory, root.Path)
	require.Len(t, candidates, 1)
	require.Equal(t, candidate, candidates[0].Path)
	require.Equal(t, "pre-cutover.sql", candidates[0].RelativePath)
	require.Equal(t, int64(len("backup")), candidates[0].Size)
	require.Equal(t, uint64(1), candidates[0].LinkCount)
	digest := sha256.Sum256([]byte("backup"))
	require.Equal(t, hex.EncodeToString(digest[:]), candidates[0].ContentSHA256)
}

func TestSnapshotPreCutoverBackupsRejectsSymlinkedCandidate(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(t.TempDir(), "outside.sql")
	require.NoError(t, os.WriteFile(target, []byte("outside"), 0o600))
	require.NoError(t, os.Symlink(target, filepath.Join(directory, "backup.sql")))

	_, _, err := snapshotPreCutoverBackups(directory, time.Now().UTC().Add(time.Hour))
	require.ErrorContains(t, err, "contains a symlink")
}

func TestSnapshotPreCutoverBackupsRejectsHardlinkedCandidate(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(t.TempDir(), "outside.sql")
	require.NoError(t, os.WriteFile(target, []byte("outside"), 0o600))
	require.NoError(t, os.Link(target, filepath.Join(directory, "backup.sql")))

	_, _, err := snapshotPreCutoverBackups(directory, time.Now().UTC().Add(time.Hour))
	require.ErrorContains(t, err, "must not be hardlinked")
}

func TestValidateBackupCandidateLinkCountFailsClosedWithoutStableLinkCount(t *testing.T) {
	path := filepath.Join(t.TempDir(), "backup.sql")
	require.NoError(t, os.WriteFile(path, []byte("backup"), 0o600))
	info, err := os.Stat(path)
	require.NoError(t, err)

	err = validateBackupCandidateLinkCount(path, fileInfoWithoutLinkCount{FileInfo: info})
	require.ErrorContains(t, err, "does not expose stable link count")
}

func TestBackupManifestDigestBindsCandidateIdentity(t *testing.T) {
	directory := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(directory, "backup.sql"), []byte("backup"), 0o600))
	root, candidates, err := snapshotPreCutoverBackups(directory, time.Now().UTC().Add(time.Hour))
	require.NoError(t, err)
	digest, err := backupManifestSHA256(&root, candidates)
	require.NoError(t, err)

	tampered := append([]backupCandidateIdentity(nil), candidates...)
	tampered[0].ContentSHA256 = strings.Repeat("0", sha256.Size*2)
	tamperedDigest, err := backupManifestSHA256(&root, tampered)
	require.NoError(t, err)
	require.NotEqual(t, digest, tamperedDigest)
}

func TestSnapshotPreCutoverBackupsIncludesEqualAndFutureMTimeFiles(t *testing.T) {
	directory := t.TempDir()
	cutoverAt := time.Now().UTC().Truncate(time.Microsecond)
	equalPath := filepath.Join(directory, "equal.sql")
	futurePath := filepath.Join(directory, "future.sql")
	require.NoError(t, os.WriteFile(equalPath, []byte("equal"), 0o600))
	require.NoError(t, os.WriteFile(futurePath, []byte("future"), 0o600))
	require.NoError(t, os.Chtimes(equalPath, cutoverAt, cutoverAt))
	require.NoError(t, os.Chtimes(futurePath, cutoverAt.Add(time.Hour), cutoverAt.Add(time.Hour)))

	_, candidates, err := snapshotPreCutoverBackups(directory, cutoverAt)
	require.NoError(t, err)
	require.Equal(t, []string{equalPath, futurePath}, backupCandidatePaths(candidates))
}

func TestValidateCommittedEvidenceRejectsCandidateIdentityTampering(t *testing.T) {
	directory := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(directory, "backup.sql"), []byte("backup"), 0o600))
	evidence := snapshotBackupEvidenceForTest(t, directory)
	evidence.Report.SchemaVersion = privateSchemaVersion
	evidence.Report.OperatorID = 42
	evidence.Report.DroppedTables = append([]string(nil), SaaSTables...)
	evidence.Report.Confirmation = ExpectedConfirmation(42)
	evidence.Report.BatchCleanupEvidence = validBatchCleanupEvidence(evidence.Report.CutoverAt)
	evidence.ReportKeySHA256 = reportKeySHA256([]byte(strings.Repeat("k", 32)))
	require.NoError(t, validateCommittedEvidence(evidence, 42, ExpectedConfirmation(42)))

	evidence.LocalBackupCandidateIdentities[0].Size++
	require.ErrorContains(t, validateCommittedEvidence(evidence, 42, ExpectedConfirmation(42)), "manifest digest is invalid")
}

func TestValidateCommittedEvidencePreservesFinalizedV2PathOnlyCompatibility(t *testing.T) {
	path := "/backups/pre-cutover.sql"
	evidence := cutoverEvidence{
		Report: MigrationReport{
			SchemaVersion:        privateSchemaVersion,
			OperatorID:           42,
			CutoverAt:            time.Now().UTC(),
			DroppedTables:        append([]string(nil), SaaSTables...),
			PurgedLocalBackups:   []string{path},
			Confirmation:         ExpectedConfirmation(42),
			BatchCleanupEvidence: validBatchCleanupEvidence(time.Now().UTC()),
		},
		ReportKeySHA256:       reportKeySHA256([]byte(strings.Repeat("k", 32))),
		LocalBackupCandidates: []string{path},
		LocalBackupRoot:       &backupRootIdentity{Path: "/backups", Device: 1, Inode: 1},
	}
	require.NoError(t, validateCommittedEvidence(evidence, 42, ExpectedConfirmation(42)))

	evidence.Report.PurgedLocalBackups = nil
	require.ErrorContains(t, validateCommittedEvidence(evidence, 42, ExpectedConfirmation(42)), "lacks candidate identities")
}

func TestSignReportRequiresKeyAndIncludesIntegrityFields(t *testing.T) {
	_, err := SignReport(MigrationReport{OperatorID: 42}, []byte("short"))
	require.Error(t, err)

	signed, err := SignReport(MigrationReport{SchemaVersion: privateSchemaVersion, OperatorID: 42}, []byte("01234567890123456789012345678901"))
	require.NoError(t, err)
	require.Contains(t, string(signed), `"report_sha256"`)
	require.Contains(t, string(signed), `"report_hmac_sha256"`)
}

func TestRunWithOptionsLockedResumesFromCommittedEvidence(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	rootDirectory := t.TempDir()
	directory := filepath.Join(rootDirectory, "backups")
	require.NoError(t, os.Mkdir(directory, 0o700))
	reportPath := filepath.Join(rootDirectory, "private-cutover-report.json")
	candidate := filepath.Join(directory, "pre-cutover-backup.sql")
	require.NoError(t, os.WriteFile(candidate, []byte("backup"), 0o600))
	cutoverAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
	require.NoError(t, os.Chtimes(candidate, cutoverAt.Add(-time.Minute), cutoverAt.Add(-time.Minute)))
	rootIdentity, candidateIdentities, err := snapshotPreCutoverBackups(directory, cutoverAt)
	require.NoError(t, err)
	manifestDigest, err := backupManifestSHA256(&rootIdentity, candidateIdentities)
	require.NoError(t, err)

	key := []byte(strings.Repeat("k", 32))
	evidence := cutoverEvidence{
		Report: MigrationReport{
			SchemaVersion:             privateSchemaVersion,
			OperatorID:                42,
			CutoverAt:                 cutoverAt,
			DeletedUsers:              7,
			DroppedTables:             append([]string(nil), SaaSTables...),
			ManagedBackupsPreserved:   3,
			PurgedSettings:            []string{"payment_enabled"},
			PurgedProtected:           []string{"admin_api_key"},
			Confirmation:              ExpectedConfirmation(42),
			LocalBackupManifestSHA256: manifestDigest,
			BatchCleanupEvidence:      validBatchCleanupEvidence(cutoverAt),
		},
		ReportKeySHA256:                reportKeySHA256(key),
		LocalBackupCandidates:          []string{candidate},
		LocalBackupCandidateIdentities: candidateIdentities,
		LocalBackupRoot:                &rootIdentity,
	}
	evidenceJSON, err := json.Marshal(evidence)
	require.NoError(t, err)

	conn, err := db.Conn(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	mock.ExpectExec(`CREATE TABLE IF NOT EXISTS exapi_private_state`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`ALTER TABLE exapi_private_state`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT private_schema_version, operator_id, cutover_at, report_sha256, cutover_evidence FROM exapi_private_state WHERE id = true`).
		WillReturnRows(sqlmock.NewRows([]string{"private_schema_version", "operator_id", "cutover_at", "report_sha256", "cutover_evidence"}).
			AddRow(privateSchemaVersion, evidence.Report.OperatorID, evidence.Report.CutoverAt, "", evidenceJSON))
	mock.ExpectExec(`UPDATE exapi_private_state SET cutover_evidence = \$1 WHERE id = true`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE exapi_private_state SET cutover_evidence = \$1 WHERE id = true`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE exapi_private_state SET report_sha256 = \$1 WHERE id = true`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	report, err := runWithOptionsLocked(
		context.Background(), conn, evidence.Report.Confirmation, key,
		CutoverOptions{LocalBackupDir: directory, ReportPath: reportPath, BatchCleanupEvidence: validBatchCleanupEvidence(cutoverAt)},
		evidence.Report.OperatorID, cutoverAt, nil,
	)
	require.NoError(t, err)
	require.Equal(t, int64(7), report.DeletedUsers)
	require.Equal(t, 3, report.ManagedBackupsPreserved)
	require.Equal(t, []string{candidate}, report.PurgedLocalBackups)
	_, err = os.Stat(candidate)
	require.ErrorIs(t, err, os.ErrNotExist)
	require.NotEmpty(t, report.ReportSHA256)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRunWithOptionsLockedRejectsCommittedStateWithoutEvidence(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	conn, err := db.Conn(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	mock.ExpectExec(`CREATE TABLE IF NOT EXISTS exapi_private_state`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`ALTER TABLE exapi_private_state`).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT private_schema_version, operator_id, cutover_at, report_sha256, cutover_evidence FROM exapi_private_state WHERE id = true`).
		WillReturnRows(sqlmock.NewRows([]string{"private_schema_version", "operator_id", "cutover_at", "report_sha256", "cutover_evidence"}).
			AddRow(privateSchemaVersion, int64(42), time.Now().UTC(), "", []byte(`{}`)))

	_, err = runWithOptionsLocked(
		context.Background(), conn, ExpectedConfirmation(42), []byte(strings.Repeat("k", 32)),
		CutoverOptions{AssertNoLocalBackups: true, Output: &bytes.Buffer{}, BatchCleanupEvidence: validBatchCleanupEvidence(time.Now().UTC())},
		42, time.Now().UTC().Truncate(time.Microsecond), nil,
	)
	require.ErrorContains(t, err, "no preserved evidence")
	require.NoError(t, mock.ExpectationsWereMet(), "retry must stop before any customer recount or destructive SQL")
}

func TestPurgeBackupCandidatesRejectsReappearedPurgedPath(t *testing.T) {
	directory := t.TempDir()
	candidate := filepath.Join(directory, "pre-cutover.sql")
	require.NoError(t, os.WriteFile(candidate, []byte("backup"), 0o600))
	evidence := snapshotBackupEvidenceForTest(t, directory)
	evidence.Report.PurgedLocalBackups = []string{candidate}

	_, err := purgeBackupCandidatesDurably(context.Background(), &cutoverEvidenceRecorder{}, evidence)
	require.ErrorContains(t, err, "previously purged backup candidate reappeared")
}

func TestPurgeBackupCandidatesRejectsFileAddedAfterSnapshot(t *testing.T) {
	directory := t.TempDir()
	candidate := filepath.Join(directory, "pre-cutover.sql")
	require.NoError(t, os.WriteFile(candidate, []byte("backup"), 0o600))
	evidence := snapshotBackupEvidenceForTest(t, directory)
	latePath := filepath.Join(directory, "late.sql")
	require.NoError(t, os.WriteFile(latePath, []byte("late backup"), 0o600))

	_, err := purgeBackupCandidatesDurably(context.Background(), &cutoverEvidenceRecorder{}, evidence)
	require.ErrorContains(t, err, "uncommitted entry after purge")
	_, candidateErr := os.Stat(candidate)
	require.ErrorIs(t, candidateErr, os.ErrNotExist)
	contents, readErr := os.ReadFile(latePath)
	require.NoError(t, readErr)
	require.Equal(t, "late backup", string(contents), "an uncommitted file must never be deleted automatically")
}

func TestPurgeBackupCandidatesRejectsFileAddedAfterEmptySnapshot(t *testing.T) {
	directory := t.TempDir()
	cutoverAt := time.Now().UTC().Truncate(time.Microsecond)
	root, candidates, err := snapshotPreCutoverBackups(directory, cutoverAt)
	require.NoError(t, err)
	require.Empty(t, candidates)
	evidence := cutoverEvidence{
		Report:          MigrationReport{CutoverAt: cutoverAt},
		LocalBackupRoot: &root,
	}
	latePath := filepath.Join(directory, "late.sql")
	require.NoError(t, os.WriteFile(latePath, []byte("late backup"), 0o600))

	_, err = purgeBackupCandidatesDurably(context.Background(), &cutoverEvidenceRecorder{}, evidence)
	require.ErrorContains(t, err, "uncommitted entry after purge")
	_, statErr := os.Stat(latePath)
	require.NoError(t, statErr, "an uncommitted file must never be deleted automatically")
}

func TestPurgeBackupCandidatesRejectsUnexplainedMissingPath(t *testing.T) {
	directory := t.TempDir()
	candidate := filepath.Join(directory, "missing-pre-cutover.sql")
	require.NoError(t, os.WriteFile(candidate, []byte("backup"), 0o600))
	evidence := snapshotBackupEvidenceForTest(t, directory)
	require.NoError(t, os.Remove(candidate))

	_, err := purgeBackupCandidatesDurably(context.Background(), &cutoverEvidenceRecorder{}, evidence)
	require.ErrorContains(t, err, "disappeared without committed deletion intent")
}

func TestPurgeBackupCandidatesAcceptsMissingPathWithCommittedIntent(t *testing.T) {
	directory := t.TempDir()
	candidate := filepath.Join(directory, "committed-pre-cutover.sql")
	require.NoError(t, os.WriteFile(candidate, []byte("backup"), 0o600))
	evidence := snapshotBackupEvidenceForTest(t, directory)
	evidence.LocalBackupDeletionIntents = []string{candidate}
	require.NoError(t, os.Remove(candidate))
	recorder := &cutoverEvidenceRecorder{}

	updated, err := purgeBackupCandidatesDurably(context.Background(), recorder, evidence)
	require.NoError(t, err)
	require.Empty(t, updated.LocalBackupDeletionIntents)
	require.Equal(t, []string{candidate}, updated.Report.PurgedLocalBackups)
	require.Len(t, recorder.persisted, 1)
}

func TestPurgeBackupCandidatesResumesAfterCompletionEvidenceFailure(t *testing.T) {
	directory := t.TempDir()
	candidate := filepath.Join(directory, "pre-cutover.sql")
	require.NoError(t, os.WriteFile(candidate, []byte("backup"), 0o600))
	evidence := snapshotBackupEvidenceForTest(t, directory)
	firstAttempt := &cutoverEvidenceRecorder{failAt: 2}

	_, err := purgeBackupCandidatesDurably(context.Background(), firstAttempt, evidence)
	require.ErrorContains(t, err, "record completed backup deletion")
	require.Len(t, firstAttempt.persisted, 1, "deletion intent must be durable before unlink")
	require.Equal(t, []string{candidate}, firstAttempt.persisted[0].LocalBackupDeletionIntents)
	_, statErr := os.Stat(candidate)
	require.ErrorIs(t, statErr, os.ErrNotExist)

	retry := &cutoverEvidenceRecorder{}
	updated, err := purgeBackupCandidatesDurably(context.Background(), retry, firstAttempt.persisted[0])
	require.NoError(t, err)
	require.Empty(t, updated.LocalBackupDeletionIntents)
	require.Equal(t, []string{candidate}, updated.Report.PurgedLocalBackups)
	require.Len(t, retry.persisted, 1)
}

func TestPurgeBackupCandidatesDoesNotDeleteWhenIntentPersistenceFails(t *testing.T) {
	directory := t.TempDir()
	candidate := filepath.Join(directory, "pre-cutover.sql")
	require.NoError(t, os.WriteFile(candidate, []byte("backup"), 0o600))
	evidence := snapshotBackupEvidenceForTest(t, directory)

	_, err := purgeBackupCandidatesDurably(context.Background(), &cutoverEvidenceRecorder{failAt: 1}, evidence)
	require.ErrorContains(t, err, "record backup deletion intent")
	contents, readErr := os.ReadFile(candidate)
	require.NoError(t, readErr)
	require.Equal(t, "backup", string(contents))
}

func TestPurgeBackupCandidatesRejectsIntermediateSymlinkSwap(t *testing.T) {
	directory := t.TempDir()
	subdirectory := filepath.Join(directory, "nested")
	require.NoError(t, os.Mkdir(subdirectory, 0o700))
	candidate := filepath.Join(subdirectory, "pre-cutover.sql")
	require.NoError(t, os.WriteFile(candidate, []byte("outside must survive"), 0o600))
	evidence := snapshotBackupEvidenceForTest(t, directory)

	movedDirectory := filepath.Join(t.TempDir(), "moved")
	require.NoError(t, os.Rename(subdirectory, movedDirectory))
	require.NoError(t, os.Symlink(movedDirectory, subdirectory))

	_, err := purgeBackupCandidatesDurably(context.Background(), &cutoverEvidenceRecorder{}, evidence)
	require.Error(t, err)
	contents, readErr := os.ReadFile(filepath.Join(movedDirectory, "pre-cutover.sql"))
	require.NoError(t, readErr)
	require.Equal(t, "outside must survive", string(contents))
}

func TestPurgeBackupCandidatesRejectsSameMetadataContentChange(t *testing.T) {
	directory := t.TempDir()
	candidate := filepath.Join(directory, "pre-cutover.sql")
	require.NoError(t, os.WriteFile(candidate, []byte("original"), 0o600))
	evidence := snapshotBackupEvidenceForTest(t, directory)
	identity := evidence.LocalBackupCandidateIdentities[0]
	require.NoError(t, os.WriteFile(candidate, []byte("tampered"), 0o600))
	mtime := time.Unix(0, identity.MTimeUnixNano)
	require.NoError(t, os.Chtimes(candidate, mtime, mtime))

	_, err := purgeBackupCandidatesDurably(context.Background(), &cutoverEvidenceRecorder{}, evidence)
	require.ErrorContains(t, err, "content digest changed")
	contents, readErr := os.ReadFile(candidate)
	require.NoError(t, readErr)
	require.Equal(t, "tampered", string(contents))
}

func TestPurgeBackupCandidatesRejectsHardlinkReplacement(t *testing.T) {
	directory := t.TempDir()
	candidate := filepath.Join(directory, "pre-cutover.sql")
	require.NoError(t, os.WriteFile(candidate, []byte("backup"), 0o600))
	evidence := snapshotBackupEvidenceForTest(t, directory)
	require.NoError(t, os.Remove(candidate))
	target := filepath.Join(t.TempDir(), "outside.sql")
	require.NoError(t, os.WriteFile(target, []byte("outside"), 0o600))
	require.NoError(t, os.Link(target, candidate))

	_, err := purgeBackupCandidatesDurably(context.Background(), &cutoverEvidenceRecorder{}, evidence)
	require.Error(t, err)
	_, candidateErr := os.Stat(candidate)
	require.NoError(t, candidateErr, "unsafe candidate must not be removed")
	_, targetErr := os.Stat(target)
	require.NoError(t, targetErr, "outside hardlink target must remain")
}

func snapshotBackupEvidenceForTest(t *testing.T, directory string) cutoverEvidence {
	t.Helper()
	cutoverAt := time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond)
	root, candidates, err := snapshotPreCutoverBackups(directory, cutoverAt)
	require.NoError(t, err)
	require.NotEmpty(t, candidates)
	manifestDigest, err := backupManifestSHA256(&root, candidates)
	require.NoError(t, err)
	return cutoverEvidence{
		Report: MigrationReport{
			CutoverAt:                 cutoverAt,
			LocalBackupManifestSHA256: manifestDigest,
		},
		LocalBackupCandidates:          backupCandidatePaths(candidates),
		LocalBackupCandidateIdentities: candidates,
		LocalBackupRoot:                &root,
	}
}

func TestRunWithOptionsLockedUpgradesVerifiedSchemaV1State(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	conn, err := db.Conn(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	rootDirectory := t.TempDir()
	directory := filepath.Join(rootDirectory, "backups")
	require.NoError(t, os.Mkdir(directory, 0o700))
	reportPath := filepath.Join(rootDirectory, "private-cutover-report.json")
	purgedPath := filepath.Join(directory, "already-purged.sql")
	stateCutoverAt := time.Date(2026, 8, 18, 10, 0, 0, 123456000, time.UTC)
	upgradeAt := stateCutoverAt.Add(2 * time.Hour)
	key := []byte(strings.Repeat("k", 32))
	v1Report, signedV1 := signV1ReportForTest(t, migrationReportV1{
		SchemaVersion:           1,
		OperatorID:              42,
		CutoverAt:               stateCutoverAt.Add(789 * time.Nanosecond),
		DeletedUsers:            7,
		DroppedTables:           append([]string(nil), SaaSTables...),
		PurgedBackups:           []string{purgedPath},
		ManagedBackupsPreserved: 3,
		PurgedSettings:          []string{"payment_enabled"},
		PurgedProtected:         []string{"admin_api_key"},
		Confirmation:            ExpectedConfirmation(42),
	}, key)
	require.NoError(t, os.WriteFile(reportPath, signedV1, 0o600))

	mock.ExpectExec(`CREATE TABLE IF NOT EXISTS exapi_private_state`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`ALTER TABLE exapi_private_state`).WillReturnResult(sqlmock.NewResult(0, 0))
	expectPrivateStateRow(mock, 1, 42, stateCutoverAt, v1Report.ReportSHA256, []byte(`{}`), false)
	mock.ExpectBegin()
	mock.ExpectExec(`SELECT pg_advisory_xact_lock\(hashtext\(\$1\)\)`).
		WithArgs(advisoryLockKey).WillReturnResult(sqlmock.NewResult(0, 1))
	expectPrivateStateRow(mock, 1, 42, stateCutoverAt, v1Report.ReportSHA256, []byte(`{}`), true)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM batch_image_jobs`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec(`(?s)UPDATE exapi_private_state.*SET private_schema_version = \$1, report_sha256 = '', cutover_evidence = \$2`).
		WithArgs(privateSchemaVersion, sqlmock.AnyArg(), int64(42), stateCutoverAt, v1Report.ReportSHA256).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectExec(`UPDATE exapi_private_state SET report_sha256 = \$1 WHERE id = true`).
		WithArgs(sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))

	report, err := runWithOptionsLocked(context.Background(), conn, ExpectedConfirmation(42), key, CutoverOptions{
		LocalBackupDir:       directory,
		BatchCleanupEvidence: validBatchCleanupEvidence(upgradeAt),
		ReportPath:           reportPath,
	}, 42, upgradeAt, nil)
	require.NoError(t, err)
	require.Equal(t, privateSchemaVersion, report.SchemaVersion)
	require.Equal(t, 1, report.UpgradedFromSchemaVersion)
	require.NotNil(t, report.UpgradedAt)
	require.True(t, report.UpgradedAt.Equal(upgradeAt))
	require.True(t, report.CutoverAt.Equal(stateCutoverAt))
	require.Equal(t, []string{purgedPath}, report.PurgedLocalBackups)
	require.Equal(t, validBatchCleanupEvidence(upgradeAt), report.BatchCleanupEvidence)
	require.NoError(t, mock.ExpectationsWereMet())

	installed, err := os.ReadFile(reportPath)
	require.NoError(t, err)
	require.Contains(t, string(installed), `"upgraded_from_private_schema_version":1`)
	require.NotContains(t, string(installed), `"purged_backups"`)
}

func TestRunWithOptionsLockedRejectsTamperedSchemaV1Report(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	conn, err := db.Conn(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	directory := t.TempDir()
	reportPath := filepath.Join(directory, "private-cutover-report.json")
	cutoverAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
	key := []byte(strings.Repeat("k", 32))
	v1Report, signed := signV1ReportForTest(t, migrationReportV1{
		SchemaVersion: 1, OperatorID: 42, CutoverAt: cutoverAt, DeletedUsers: 7,
		DroppedTables: append([]string(nil), SaaSTables...), Confirmation: ExpectedConfirmation(42),
	}, key)
	tampered := bytes.Replace(signed, []byte(`"deleted_users":7`), []byte(`"deleted_users":8`), 1)
	require.NoError(t, os.WriteFile(reportPath, tampered, 0o600))
	mock.ExpectExec(`CREATE TABLE IF NOT EXISTS exapi_private_state`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`ALTER TABLE exapi_private_state`).WillReturnResult(sqlmock.NewResult(0, 0))
	expectPrivateStateRow(mock, 1, 42, cutoverAt, v1Report.ReportSHA256, []byte(`{}`), false)

	_, err = runWithOptionsLocked(context.Background(), conn, ExpectedConfirmation(42), key, CutoverOptions{
		AssertNoLocalBackups: true, BatchCleanupEvidence: validBatchCleanupEvidence(cutoverAt.Add(time.Hour)), ReportPath: reportPath,
	}, 42, cutoverAt.Add(time.Hour), nil)
	require.ErrorContains(t, err, "SHA-256 does not match")
	require.NoError(t, mock.ExpectationsWereMet(), "tampering must fail before the upgrade transaction")
}

func TestReadAndVerifyRetainedV1ReportRejectsDurableDigestMismatch(t *testing.T) {
	directory := t.TempDir()
	reportPath := filepath.Join(directory, "private-cutover-report.json")
	cutoverAt := time.Now().UTC().Truncate(time.Microsecond)
	key := []byte(strings.Repeat("k", 32))
	report, signed := signV1ReportForTest(t, migrationReportV1{
		SchemaVersion: 1, OperatorID: 42, CutoverAt: cutoverAt,
		DroppedTables: append([]string(nil), SaaSTables...), Confirmation: ExpectedConfirmation(42),
	}, key)
	require.NoError(t, os.WriteFile(reportPath, signed, 0o600))
	_, err := readAndVerifyRetainedV1Report(reportPath, key, privateCutoverState{
		SchemaVersion: 1, OperatorID: 42, CutoverAt: cutoverAt, ReportSHA256: strings.Repeat("0", sha256.Size*2),
	}, 42, ExpectedConfirmation(42))
	require.ErrorContains(t, err, "does not match durable private state")
	require.NotEmpty(t, report.ReportSHA256)
}

func TestRunWithOptionsLockedSchemaV1UpgradeRollsBackWhenBatchRowsRemain(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	conn, err := db.Conn(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	directory := t.TempDir()
	reportPath := filepath.Join(directory, "private-cutover-report.json")
	cutoverAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
	upgradeAt := cutoverAt.Add(time.Hour)
	key := []byte(strings.Repeat("k", 32))
	v1Report, signed := signV1ReportForTest(t, migrationReportV1{
		SchemaVersion: 1, OperatorID: 42, CutoverAt: cutoverAt,
		DroppedTables: append([]string(nil), SaaSTables...), Confirmation: ExpectedConfirmation(42),
	}, key)
	require.NoError(t, os.WriteFile(reportPath, signed, 0o600))
	mock.ExpectExec(`CREATE TABLE IF NOT EXISTS exapi_private_state`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`ALTER TABLE exapi_private_state`).WillReturnResult(sqlmock.NewResult(0, 0))
	expectPrivateStateRow(mock, 1, 42, cutoverAt, v1Report.ReportSHA256, []byte(`{}`), false)
	mock.ExpectBegin()
	mock.ExpectExec(`SELECT pg_advisory_xact_lock\(hashtext\(\$1\)\)`).
		WithArgs(advisoryLockKey).WillReturnResult(sqlmock.NewResult(0, 1))
	expectPrivateStateRow(mock, 1, 42, cutoverAt, v1Report.ReportSHA256, []byte(`{}`), true)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM batch_image_jobs`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectRollback()

	_, err = runWithOptionsLocked(context.Background(), conn, ExpectedConfirmation(42), key, CutoverOptions{
		AssertNoLocalBackups: true, BatchCleanupEvidence: validBatchCleanupEvidence(upgradeAt), ReportPath: reportPath,
	}, 42, upgradeAt, nil)
	require.ErrorContains(t, err, "1 job(s) remain")
	require.NoError(t, mock.ExpectationsWereMet())
	retained, err := os.ReadFile(reportPath)
	require.NoError(t, err)
	require.Equal(t, signed, retained)
}

func signV1ReportForTest(t *testing.T, report migrationReportV1, key []byte) (migrationReportV1, []byte) {
	t.Helper()
	unsigned, err := unsignedV1ReportPayload(report)
	require.NoError(t, err)
	digest := sha256.Sum256(unsigned)
	report.ReportSHA256 = hex.EncodeToString(digest[:])
	signer := hmac.New(sha256.New, key)
	_, _ = signer.Write(unsigned)
	report.ReportHMACSHA256 = hex.EncodeToString(signer.Sum(nil))
	signed, err := json.Marshal(report)
	require.NoError(t, err)
	return report, signed
}

func expectPrivateStateRow(mock sqlmock.Sqlmock, version int, operatorID int64, cutoverAt time.Time, digest string, evidence []byte, forUpdate bool) {
	query := `SELECT private_schema_version, operator_id, cutover_at, report_sha256, cutover_evidence FROM exapi_private_state WHERE id = true`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	mock.ExpectQuery(query).WillReturnRows(sqlmock.NewRows([]string{
		"private_schema_version", "operator_id", "cutover_at", "report_sha256", "cutover_evidence",
	}).AddRow(version, operatorID, cutoverAt, digest, evidence))
}

func TestParseReportKeyAcceptsProductionHexAndRejectsShortInput(t *testing.T) {
	key, err := ParseReportKey(strings.Repeat("ab", 32))
	require.NoError(t, err)
	require.Len(t, key, 32)
	require.ErrorContains(t, func() error {
		_, err := ParseReportKey("short")
		return err
	}(), "at least 32 bytes")
}
