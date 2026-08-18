package privatecutover

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func validBatchCleanupEvidence(at time.Time) BatchCleanupEvidence {
	return BatchCleanupEvidence{
		SchemaVersion:     BatchCleanupEvidenceSchemaVersion,
		Verified:          true,
		VerifiedAt:        at.UTC(),
		EvidenceURI:       "s3://exapi-audit/batch-cleanup/manifest.json",
		EvidenceVersionID: "immutable-version-1",
		EvidenceSHA256:    strings.Repeat("a", 64),
	}
}

func TestReadBatchCleanupEvidence(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "batch-cleanup.json")
	raw, err := json.Marshal(validBatchCleanupEvidence(time.Now()))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, raw, 0o600))

	evidence, err := ReadBatchCleanupEvidence(path)
	require.NoError(t, err)
	require.True(t, evidence.Verified)

	require.NoError(t, os.Chmod(path, 0o644))
	_, err = ReadBatchCleanupEvidence(path)
	require.ErrorContains(t, err, "mode 0600")
}

func TestReadBatchCleanupEvidenceRejectsAliasesAndUnknownFields(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "target.json")
	require.NoError(t, os.WriteFile(target, []byte(`{"schema_version":1}`), 0o600))
	symlink := filepath.Join(directory, "alias.json")
	require.NoError(t, os.Symlink(target, symlink))
	_, err := ReadBatchCleanupEvidence(symlink)
	require.ErrorContains(t, err, "non-symlink")

	unknown := filepath.Join(directory, "unknown.json")
	raw, err := json.Marshal(validBatchCleanupEvidence(time.Now()))
	require.NoError(t, err)
	raw = append(raw[:len(raw)-1], []byte(`,"unexpected":true}`)...)
	require.NoError(t, os.WriteFile(unknown, raw, 0o600))
	_, err = ReadBatchCleanupEvidence(unknown)
	require.ErrorContains(t, err, "unknown field")
}

func TestValidateBatchCleanupEvidenceRequiresFreshImmutableZeroAttestation(t *testing.T) {
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	require.NoError(t, validateBatchCleanupEvidence(validBatchCleanupEvidence(now), now))

	stale := validBatchCleanupEvidence(now.Add(-16 * time.Minute))
	require.ErrorContains(t, validateBatchCleanupEvidence(stale, now), "stale")

	remaining := validBatchCleanupEvidence(now)
	remaining.ProviderJobsRemaining = 1
	require.ErrorContains(t, validateBatchCleanupEvidence(remaining, now), "zero SQL rows")

	mutable := validBatchCleanupEvidence(now)
	mutable.EvidenceVersionID = ""
	require.ErrorContains(t, validateBatchCleanupEvidence(mutable, now), "version_id")
}
