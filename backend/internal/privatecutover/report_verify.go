package privatecutover

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"
)

func unsignedReportPayload(report MigrationReport) ([]byte, error) {
	report.ReportSHA256 = ""
	report.ReportHMACSHA256 = ""
	return json.Marshal(report)
}

// VerifyMigrationReport independently verifies the signed report payload and
// its durable database marker. It is safe to call only after the offline
// cutover command has returned successfully.
func VerifyMigrationReport(ctx context.Context, db *sql.DB, signed, key []byte) (MigrationReport, error) {
	if db == nil {
		return MigrationReport{}, errors.New("database is required")
	}
	if len(key) < 32 {
		return MigrationReport{}, errors.New("report verification key must be at least 32 bytes")
	}

	var report MigrationReport
	decoder := json.NewDecoder(bytes.NewReader(signed))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&report); err != nil {
		return MigrationReport{}, fmt.Errorf("decode migration report: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return MigrationReport{}, errors.New("decode migration report: trailing JSON content")
	}
	if report.SchemaVersion != privateSchemaVersion {
		return MigrationReport{}, fmt.Errorf("migration report schema version %d does not match %d", report.SchemaVersion, privateSchemaVersion)
	}
	if report.OperatorID <= 0 || report.Confirmation != ExpectedConfirmation(report.OperatorID) {
		return MigrationReport{}, errors.New("migration report operator confirmation is invalid")
	}
	if report.CutoverAt.IsZero() || !slices.Equal(report.DroppedTables, SaaSTables) {
		return MigrationReport{}, errors.New("migration report cutover metadata is incomplete")
	}

	unsigned, err := unsignedReportPayload(report)
	if err != nil {
		return MigrationReport{}, fmt.Errorf("encode unsigned migration report: %w", err)
	}
	digest := sha256.Sum256(unsigned)
	expectedDigest := hex.EncodeToString(digest[:])
	if !hmac.Equal([]byte(strings.ToLower(report.ReportSHA256)), []byte(expectedDigest)) {
		return MigrationReport{}, errors.New("migration report SHA-256 does not match its payload")
	}
	providedMAC, err := hex.DecodeString(report.ReportHMACSHA256)
	if err != nil || len(providedMAC) != sha256.Size {
		return MigrationReport{}, errors.New("migration report HMAC-SHA-256 is invalid")
	}
	signer := hmac.New(sha256.New, key)
	_, _ = signer.Write(unsigned)
	if !hmac.Equal(providedMAC, signer.Sum(nil)) {
		return MigrationReport{}, errors.New("migration report HMAC-SHA-256 does not match its payload")
	}

	var stateVersion int
	var stateOperatorID int64
	var stateCutoverAt time.Time
	var stateDigest string
	var stateEvidenceJSON []byte
	if err := db.QueryRowContext(ctx, `SELECT private_schema_version, operator_id, cutover_at, report_sha256, cutover_evidence FROM exapi_private_state WHERE id = true`).
		Scan(&stateVersion, &stateOperatorID, &stateCutoverAt, &stateDigest, &stateEvidenceJSON); err != nil {
		return MigrationReport{}, fmt.Errorf("query private migration report state: %w", err)
	}
	if stateVersion != report.SchemaVersion || stateOperatorID != report.OperatorID ||
		!stateCutoverAt.UTC().Equal(report.CutoverAt.UTC()) || stateDigest != report.ReportSHA256 {
		return MigrationReport{}, errors.New("migration report does not match durable private state")
	}
	var evidence cutoverEvidence
	if err := json.Unmarshal(stateEvidenceJSON, &evidence); err != nil || evidence.empty() {
		return MigrationReport{}, errors.New("migration report durable evidence is missing or invalid")
	}
	if err := validateCommittedEvidence(evidence, report.OperatorID, report.Confirmation); err != nil {
		return MigrationReport{}, fmt.Errorf("migration report durable evidence is invalid: %w", err)
	}
	if !hmac.Equal([]byte(evidence.ReportKeySHA256), []byte(reportKeySHA256(key))) {
		return MigrationReport{}, errors.New("migration report key does not match preserved cutover evidence")
	}
	stateReportPayload, err := unsignedReportPayload(evidence.Report)
	if err != nil {
		return MigrationReport{}, fmt.Errorf("encode durable migration evidence: %w", err)
	}
	if !bytes.Equal(stateReportPayload, unsigned) {
		return MigrationReport{}, errors.New("migration report does not match preserved cutover evidence")
	}
	if err := VerifyRuntimeState(ctx, db); err != nil {
		return MigrationReport{}, err
	}
	return report, nil
}
