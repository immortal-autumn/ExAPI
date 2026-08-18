package privatecutover

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	BatchCleanupEvidenceSchemaVersion = 1
	maxBatchCleanupEvidenceBytes      = 1 << 20
	batchCleanupEvidenceMaxAge        = 15 * time.Minute
	batchCleanupEvidenceFutureSkew    = 5 * time.Minute
)

// BatchCleanupEvidence is the compact, immutable provider-cleanup
// attestation embedded in the signed cutover report. The detailed manifest is
// retained at EvidenceURI; its exact object version and digest make this
// summary independently auditable without placing provider data in the report.
type BatchCleanupEvidence struct {
	SchemaVersion            int       `json:"schema_version"`
	Verified                 bool      `json:"verified"`
	VerifiedAt               time.Time `json:"verified_at"`
	SQLRowsRemaining         int64     `json:"sql_rows_remaining"`
	ProviderJobsRemaining    int64     `json:"provider_jobs_remaining"`
	ProviderInputsRemaining  int64     `json:"provider_inputs_remaining"`
	ProviderOutputsRemaining int64     `json:"provider_outputs_remaining"`
	EvidenceURI              string    `json:"evidence_uri"`
	EvidenceVersionID        string    `json:"evidence_version_id"`
	EvidenceSHA256           string    `json:"evidence_sha256"`
}

// ReadBatchCleanupEvidence reads one protected regular file through a stable
// descriptor and rejects aliases, trailing JSON, and oversized attestations.
func ReadBatchCleanupEvidence(path string) (BatchCleanupEvidence, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return BatchCleanupEvidence{}, errors.New("batch cleanup evidence file is required")
	}
	if !filepath.IsAbs(path) {
		return BatchCleanupEvidence{}, errors.New("batch cleanup evidence file must be an absolute path")
	}
	before, err := os.Lstat(path)
	if err != nil {
		return BatchCleanupEvidence{}, fmt.Errorf("inspect batch cleanup evidence: %w", err)
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return BatchCleanupEvidence{}, errors.New("batch cleanup evidence must be a regular non-symlink file")
	}
	if before.Mode().Perm() != 0o600 {
		return BatchCleanupEvidence{}, fmt.Errorf("batch cleanup evidence must use mode 0600 (mode=%04o)", before.Mode().Perm())
	}
	file, err := os.Open(path)
	if err != nil {
		return BatchCleanupEvidence{}, fmt.Errorf("open batch cleanup evidence: %w", err)
	}
	defer func() { _ = file.Close() }()
	after, err := file.Stat()
	if err != nil {
		return BatchCleanupEvidence{}, fmt.Errorf("stat batch cleanup evidence: %w", err)
	}
	if !os.SameFile(before, after) {
		return BatchCleanupEvidence{}, errors.New("batch cleanup evidence changed while it was opened")
	}
	raw, err := io.ReadAll(io.LimitReader(file, maxBatchCleanupEvidenceBytes+1))
	if err != nil {
		return BatchCleanupEvidence{}, fmt.Errorf("read batch cleanup evidence: %w", err)
	}
	if len(raw) > maxBatchCleanupEvidenceBytes {
		return BatchCleanupEvidence{}, fmt.Errorf("batch cleanup evidence exceeds %d bytes", maxBatchCleanupEvidenceBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var evidence BatchCleanupEvidence
	if err := decoder.Decode(&evidence); err != nil {
		return BatchCleanupEvidence{}, fmt.Errorf("decode batch cleanup evidence: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return BatchCleanupEvidence{}, errors.New("decode batch cleanup evidence: trailing JSON content")
	}
	if err := validateBatchCleanupEvidence(evidence, time.Time{}); err != nil {
		return BatchCleanupEvidence{}, err
	}
	return evidence, nil
}

func validateBatchCleanupEvidence(evidence BatchCleanupEvidence, reference time.Time) error {
	if evidence.SchemaVersion != BatchCleanupEvidenceSchemaVersion || !evidence.Verified {
		return errors.New("batch cleanup evidence must be verified schema version 1")
	}
	if evidence.VerifiedAt.IsZero() {
		return errors.New("batch cleanup evidence verified_at is required")
	}
	if evidence.SQLRowsRemaining != 0 || evidence.ProviderJobsRemaining != 0 ||
		evidence.ProviderInputsRemaining != 0 || evidence.ProviderOutputsRemaining != 0 {
		return errors.New("batch cleanup evidence must attest zero SQL rows, provider jobs, inputs, and outputs")
	}
	parsedURI, err := url.Parse(strings.TrimSpace(evidence.EvidenceURI))
	if err != nil || parsedURI.Scheme != "s3" || parsedURI.Host == "" || strings.Trim(parsedURI.Path, "/") == "" {
		return errors.New("batch cleanup evidence_uri must be an off-host s3:// object URI")
	}
	if strings.TrimSpace(evidence.EvidenceVersionID) == "" {
		return errors.New("batch cleanup evidence_version_id is required")
	}
	if len(evidence.EvidenceSHA256) != 64 || evidence.EvidenceSHA256 != strings.ToLower(evidence.EvidenceSHA256) {
		return errors.New("batch cleanup evidence_sha256 must be a lowercase SHA-256 digest")
	}
	if decoded, err := hex.DecodeString(evidence.EvidenceSHA256); err != nil || len(decoded) != 32 {
		return errors.New("batch cleanup evidence_sha256 must be a lowercase SHA-256 digest")
	}
	if !reference.IsZero() && (evidence.VerifiedAt.Before(reference.Add(-batchCleanupEvidenceMaxAge)) ||
		evidence.VerifiedAt.After(reference.Add(batchCleanupEvidenceFutureSkew))) {
		return errors.New("batch cleanup evidence is stale or from the future")
	}
	return nil
}
