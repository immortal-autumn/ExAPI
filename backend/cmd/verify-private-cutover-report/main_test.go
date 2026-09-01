package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecodedReportKeySHA256IsNotPrintableKeyFileDigest(t *testing.T) {
	printable := []byte(strings.Repeat("ab", 32) + "\n")
	decoded, err := hex.DecodeString(strings.TrimSpace(string(printable)))
	require.NoError(t, err)

	fileDigest := sha256.Sum256(printable)
	require.Equal(t, "9a2db2e23f1504cd056606553ac049c5e718e8f9ce9233876df1a7a1821af885", decodedReportKeySHA256(decoded))
	require.NotEqual(t, hex.EncodeToString(fileDigest[:]), decodedReportKeySHA256(decoded))
}

func TestCanonicalFinalPathResolvesSymlinkedParent(t *testing.T) {
	realDirectory := t.TempDir()
	aliasRoot := t.TempDir()
	aliasDirectory := filepath.Join(aliasRoot, "alias")
	require.NoError(t, os.Symlink(realDirectory, aliasDirectory))

	realPath, err := canonicalFinalPath(filepath.Join(realDirectory, "evidence.json"))
	require.NoError(t, err)
	aliasPath, err := canonicalFinalPath(filepath.Join(aliasDirectory, "evidence.json"))
	require.NoError(t, err)
	require.Equal(t, realPath, aliasPath)
}

func TestValidateReportEvidencePathsRejectsFinalAliases(t *testing.T) {
	directory := t.TempDir()
	report := filepath.Join(directory, "report.json")
	require.NoError(t, os.WriteFile(report, []byte("report"), 0o600))

	symlink := filepath.Join(directory, "evidence-symlink.json")
	require.NoError(t, os.Symlink(report, symlink))
	_, _, err := validateReportEvidencePaths(report, symlink)
	require.ErrorContains(t, err, "must not be a symlink")

	hardlink := filepath.Join(directory, "evidence-hardlink.json")
	require.NoError(t, os.Link(report, hardlink))
	_, _, err = validateReportEvidencePaths(report, hardlink)
	require.ErrorContains(t, err, "must not alias")
}

func TestReadMigrationReportRejectsOversizedFile(t *testing.T) {
	directory := t.TempDir()
	report := filepath.Join(directory, "report.json")
	contents := make([]byte, maxMigrationReportBytes+1)
	require.NoError(t, os.WriteFile(report, contents, 0o600))

	_, err := readMigrationReport(report)
	require.ErrorContains(t, err, "exceeds")
}

func TestReadMigrationReportAcceptsProtectedBoundedFile(t *testing.T) {
	directory := t.TempDir()
	report := filepath.Join(directory, "report.json")
	contents := []byte(`{"schema_version":1}`)
	require.NoError(t, os.WriteFile(report, contents, 0o600))

	got, err := readMigrationReport(report)
	require.NoError(t, err)
	require.Equal(t, contents, got)
}

func TestEnsureReportPathIdentityRejectsSymlink(t *testing.T) {
	directory := t.TempDir()
	report := filepath.Join(directory, "report.json")
	link := filepath.Join(directory, "report-link.json")
	contents := []byte(`{"schema_version":1}`)
	require.NoError(t, os.WriteFile(report, contents, 0o600))
	require.NoError(t, os.Symlink(report, link))
	info, err := os.Stat(report)
	require.NoError(t, err)

	require.ErrorContains(t, ensureReportPathIdentity(link, info), "non-regular or symlink")
}
