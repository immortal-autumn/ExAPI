package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/privatecutover"
	"github.com/Wei-Shaw/sub2api/internal/repository"
)

type verificationEvidence struct {
	Verified         bool      `json:"verified"`
	VerifiedAt       time.Time `json:"verified_at"`
	SchemaVersion    int       `json:"private_schema_version"`
	OperatorID       int64     `json:"operator_id"`
	CutoverAt        time.Time `json:"cutover_at"`
	ReportSHA256     string    `json:"report_sha256"`
	ReportFileSHA256 string    `json:"report_file_sha256"`
	ReportKeySHA256  string    `json:"report_key_sha256"`
	DatabaseMatched  bool      `json:"database_matched"`
	HMACVerified     bool      `json:"hmac_verified"`
}

// A migration report is a compact signed JSON document. Bound the amount of
// data this verification utility will buffer so a hostile or accidentally
// replaced report cannot cause unbounded memory growth.
const maxMigrationReportBytes int64 = 4 << 20

func main() {
	reportPath := flag.String("report-file", os.Getenv("EXAPI_PRIVATE_MIGRATION_REPORT"), "required signed migration report file")
	evidencePath := flag.String("evidence-file", os.Getenv("EXAPI_PRIVATE_MIGRATION_EVIDENCE"), "required durable 0600 verification evidence file")
	flag.Parse()
	if strings.TrimSpace(*reportPath) == "" || strings.TrimSpace(*evidencePath) == "" {
		log.Fatal("--report-file and --evidence-file are required")
	}
	canonicalReportPath, canonicalEvidencePath, err := validateReportEvidencePaths(*reportPath, *evidencePath)
	if err != nil {
		log.Fatal(err)
	}

	signed, err := readMigrationReport(canonicalReportPath)
	if err != nil {
		log.Fatalf("read migration report: %v", err)
	}
	reportKey, err := privatecutover.ParseReportKey(os.Getenv("EXAPI_MIGRATION_REPORT_KEY"))
	if err != nil {
		log.Fatal(err)
	}
	cfg, err := config.LoadForBootstrap()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	client, db, err := repository.InitEnt(cfg)
	if err != nil {
		log.Fatalf("initialize database: %v", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			log.Printf("close database client: %v", err)
		}
	}()

	report, err := privatecutover.VerifyMigrationReport(context.Background(), db, signed, reportKey)
	if err != nil {
		log.Fatalf("verify private cutover report: %v", err)
	}
	fileDigest := sha256.Sum256(signed)
	// This fingerprints the decoded signing-key bytes used for HMAC and for
	// the durable database evidence. It is intentionally not the digest of
	// the printable key file, which is tracked separately by orchestration.
	evidence := verificationEvidence{
		Verified:         true,
		VerifiedAt:       time.Now().UTC(),
		SchemaVersion:    report.SchemaVersion,
		OperatorID:       report.OperatorID,
		CutoverAt:        report.CutoverAt,
		ReportSHA256:     report.ReportSHA256,
		ReportFileSHA256: hex.EncodeToString(fileDigest[:]),
		ReportKeySHA256:  decodedReportKeySHA256(reportKey),
		DatabaseMatched:  true,
		HMACVerified:     true,
	}
	encoded, err := json.Marshal(evidence)
	if err != nil {
		log.Fatalf("encode verification evidence: %v", err)
	}
	if err := privatecutover.WriteDurableFile(canonicalEvidencePath, encoded); err != nil {
		log.Fatalf("write verification evidence: %v", err)
	}
	fmt.Printf("private cutover report verified: operator=%d report_sha256=%s\n", report.OperatorID, report.ReportSHA256)
}

func readMigrationReport(path string) ([]byte, error) {
	reportFile, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = reportFile.Close() }()

	initial, err := reportFile.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat migration report: %w", err)
	}
	if !initial.Mode().IsRegular() || initial.Mode().Perm() != 0o600 {
		return nil, fmt.Errorf("migration report must be a regular 0600 file (mode=%s)", initial.Mode())
	}
	if initial.Size() > maxMigrationReportBytes {
		return nil, fmt.Errorf("migration report exceeds %d-byte limit", maxMigrationReportBytes)
	}

	// The path was canonicalized and inspected before opening. Re-check it
	// against the descriptor so a same-UID replacement cannot make the utility
	// verify a different file than the one it inspected.
	if err := ensureReportPathIdentity(path, initial); err != nil {
		return nil, err
	}
	signed, err := io.ReadAll(io.LimitReader(reportFile, maxMigrationReportBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(signed)) > maxMigrationReportBytes {
		return nil, fmt.Errorf("migration report exceeds %d-byte limit", maxMigrationReportBytes)
	}

	final, err := reportFile.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat migration report after read: %w", err)
	}
	if !os.SameFile(initial, final) || initial.Size() != final.Size() || !initial.ModTime().Equal(final.ModTime()) {
		return nil, errors.New("migration report changed while being read")
	}
	if int64(len(signed)) != final.Size() {
		return nil, errors.New("migration report size changed while being read")
	}
	if err := ensureReportPathIdentity(path, final); err != nil {
		return nil, err
	}
	return signed, nil
}

func ensureReportPathIdentity(path string, expected os.FileInfo) error {
	current, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("stat migration report path: %w", err)
	}
	if current.Mode()&os.ModeSymlink != 0 || !current.Mode().IsRegular() || current.Mode().Perm() != 0o600 {
		return errors.New("migration report path changed to a non-regular or symlink file")
	}
	if !os.SameFile(expected, current) {
		return errors.New("migration report path changed while being read")
	}
	return nil
}

func decodedReportKeySHA256(key []byte) string {
	digest := sha256.Sum256(key)
	return hex.EncodeToString(digest[:])
}

func canonicalFinalPath(path string) (string, error) {
	absolute, err := filepath.Abs(filepath.Clean(strings.TrimSpace(path)))
	if err != nil {
		return "", err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(absolute))
	if err != nil {
		return "", err
	}
	return filepath.Join(parent, filepath.Base(absolute)), nil
}

func validateReportEvidencePaths(reportPath, evidencePath string) (string, string, error) {
	report, err := canonicalFinalPath(reportPath)
	if err != nil {
		return "", "", fmt.Errorf("canonicalize migration report path: %w", err)
	}
	evidence, err := canonicalFinalPath(evidencePath)
	if err != nil {
		return "", "", fmt.Errorf("canonicalize verification evidence path: %w", err)
	}
	if report == evidence {
		return "", "", errors.New("report and evidence files must be different paths")
	}
	reportInfo, err := os.Lstat(report)
	if err != nil {
		return "", "", fmt.Errorf("inspect migration report path: %w", err)
	}
	if reportInfo.Mode()&os.ModeSymlink != 0 {
		return "", "", errors.New("migration report path must not be a symlink")
	}
	if evidenceInfo, evidenceErr := os.Lstat(evidence); evidenceErr == nil {
		if evidenceInfo.Mode()&os.ModeSymlink != 0 {
			return "", "", errors.New("verification evidence path must not be a symlink")
		}
		if os.SameFile(reportInfo, evidenceInfo) {
			return "", "", errors.New("report and evidence files must not alias the same file")
		}
	} else if !errors.Is(evidenceErr, os.ErrNotExist) {
		return "", "", fmt.Errorf("inspect verification evidence path: %w", evidenceErr)
	}
	return report, evidence, nil
}
