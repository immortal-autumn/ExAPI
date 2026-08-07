package service

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/security/secretcrypto"
	"github.com/stretchr/testify/require"
)

func backupTestKey(fill byte) []byte { return bytes.Repeat([]byte{fill}, 32) }

func backupTestKeyring(t *testing.T) *secretcrypto.Keyring {
	t.Helper()
	ring, err := secretcrypto.NewKeyring("backup-1", map[string][]byte{"backup-1": backupTestKey(1)})
	if err != nil {
		t.Fatal(err)
	}
	return ring
}

// useSecureBackupTestTempDir prefers the project-local test directory. The
// workspace may live on a permissionless removable filesystem, so this narrow
// security test falls back to a memory-backed Linux temp directory when the
// local filesystem cannot represent mode 0600.
func useSecureBackupTestTempDir(t *testing.T) {
	t.Helper()
	candidates := []string{t.TempDir(), "/dev/shm", "/tmp"}
	for _, base := range candidates {
		info, err := os.Stat(base)
		if err != nil || !info.IsDir() {
			continue
		}
		dir, err := os.MkdirTemp(base, "exapi-backup-test-*")
		if err != nil {
			continue
		}
		probe, err := os.CreateTemp(dir, "permission-probe-*")
		if err == nil {
			err = probe.Chmod(0o600)
		}
		if err == nil {
			var probeInfo os.FileInfo
			probeInfo, err = probe.Stat()
			if err == nil && probeInfo.Mode().Perm() != 0o600 {
				err = ErrInsecureBackupStagingPermissions
			}
		}
		if probe != nil {
			_ = probe.Close()
			_ = os.Remove(probe.Name())
		}
		if err != nil {
			_ = os.RemoveAll(dir)
			continue
		}
		t.Cleanup(func() { _ = os.RemoveAll(dir) })
		t.Setenv("TMPDIR", dir)
		return
	}
	t.Skip("no temporary filesystem can enforce mode 0600")
}

func TestStageDecryptedBackupSupportsRetainedKeyAfterRotation(t *testing.T) {
	useSecureBackupTestTempDir(t)
	oldRing, err := secretcrypto.NewKeyring("backup-old", map[string][]byte{"backup-old": backupTestKey(1)})
	require.NoError(t, err)
	rotatedRing, err := secretcrypto.NewKeyring("backup-new", map[string][]byte{
		"backup-old": backupTestKey(1),
		"backup-new": backupTestKey(2),
	})
	require.NoError(t, err)
	plaintext := bytes.Repeat([]byte("SELECT 1;\n"), 128)

	oldBackup, _, err := stageEncryptedBackup(oldRing, io.NopCloser(bytes.NewReader(plaintext)))
	require.NoError(t, err)
	defer cleanupStagedFile(oldBackup)
	restored, err := stageDecryptedBackup(rotatedRing, oldBackup)
	require.NoError(t, err)
	defer cleanupStagedFile(restored)
	gz, err := gzip.NewReader(restored)
	require.NoError(t, err)
	got, err := io.ReadAll(gz)
	require.NoError(t, err)
	require.NoError(t, gz.Close())
	require.Equal(t, plaintext, got)

	newBackup, _, err := stageEncryptedBackup(rotatedRing, io.NopCloser(bytes.NewReader(plaintext)))
	require.NoError(t, err)
	defer cleanupStagedFile(newBackup)
	header := make([]byte, len("S2BKENC1")+1+len("backup-new"))
	_, err = io.ReadFull(newBackup, header)
	require.NoError(t, err)
	require.Contains(t, string(header), "backup-new")
}

func TestStageEncryptedBackupRoundTripAndPermissions(t *testing.T) {
	useSecureBackupTestTempDir(t)
	plaintext := bytes.Repeat([]byte("CREATE TABLE secure_data;\n"), 8000)

	staged, size, err := stageEncryptedBackup(backupTestKeyring(t), io.NopCloser(bytes.NewReader(plaintext)))
	if err != nil {
		t.Fatal(err)
	}
	path := staged.Name()
	defer func() {
		_ = staged.Close()
		_ = os.Remove(path)
	}()
	if size <= 0 {
		t.Fatalf("staged size = %d", size)
	}
	info, err := staged.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("temporary backup mode = %o, want 600", info.Mode().Perm())
	}

	header := make([]byte, 8)
	if _, err := io.ReadFull(staged, header); err != nil {
		t.Fatal(err)
	}
	if string(header) != "S2BKENC1" {
		t.Fatalf("unexpected encrypted header %q", header)
	}
	if _, err := staged.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}

	decrypted, err := stageDecryptedBackup(backupTestKeyring(t), staged)
	if err != nil {
		t.Fatal(err)
	}
	decryptedPath := decrypted.Name()
	defer func() {
		_ = decrypted.Close()
		_ = os.Remove(decryptedPath)
	}()
	gz, err := gzip.NewReader(decrypted)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := io.ReadAll(gz)
	if err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(restored, plaintext) {
		t.Fatal("staged backup round trip changed dump")
	}
}

func TestStageDecryptedBackupRejectsTamperingBeforeReturningFile(t *testing.T) {
	useSecureBackupTestTempDir(t)
	tmpDir := os.Getenv("TMPDIR")
	ring := backupTestKeyring(t)
	staged, _, err := stageEncryptedBackup(ring, io.NopCloser(bytes.NewReader([]byte("secret dump"))))
	if err != nil {
		t.Fatal(err)
	}
	path := staged.Name()
	data, err := io.ReadAll(staged)
	if err != nil {
		t.Fatal(err)
	}
	_ = staged.Close()
	_ = os.Remove(path)
	data[len(data)-1] ^= 0x80

	if file, err := stageDecryptedBackup(ring, bytes.NewReader(data)); err == nil {
		if file != nil {
			_ = file.Close()
			_ = os.Remove(file.Name())
		}
		t.Fatal("tampered backup was accepted")
	}
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if matched, _ := filepath.Match("sub2api-restore-*.sql.gz", entry.Name()); matched {
			t.Fatalf("failed decryption leaked temporary file %q", entry.Name())
		}
	}
}

func TestStageDecryptedBackupRejectsOversizedOutput(t *testing.T) {
	useSecureBackupTestTempDir(t)
	ring := backupTestKeyring(t)
	var encrypted bytes.Buffer
	require.NoError(t, ring.EncryptStream(backupStreamPurpose, &encrypted, bytes.NewReader(bytes.Repeat([]byte("x"), 65))))

	file, err := stageDecryptedBackupWithLimit(ring, bytes.NewReader(encrypted.Bytes()), 64)
	if file != nil {
		cleanupStagedFile(file)
	}
	require.ErrorIs(t, err, ErrBackupStagingTooLarge)
}

func TestStageLegacySQLBackupRejectsOversizedOutput(t *testing.T) {
	useSecureBackupTestTempDir(t)
	var compressed bytes.Buffer
	gz := gzip.NewWriter(&compressed)
	_, err := gz.Write(bytes.Repeat([]byte("x"), 65))
	require.NoError(t, err)
	require.NoError(t, gz.Close())

	file, err := stageLegacySQLBackupWithLimit(bytes.NewReader(compressed.Bytes()), 64)
	if file != nil {
		cleanupStagedFile(file)
	}
	require.ErrorIs(t, err, ErrBackupStagingTooLarge)
}

func TestBackupStagingFailsClosedWithoutCipher(t *testing.T) {
	if file, _, err := stageEncryptedBackup(nil, io.NopCloser(bytes.NewReader([]byte("dump")))); err == nil {
		if file != nil {
			_ = file.Close()
			_ = os.Remove(file.Name())
		}
		t.Fatal("backup staging succeeded without encryption keyring")
	}
	if file, err := stageDecryptedBackup(nil, bytes.NewReader(nil)); err == nil {
		if file != nil {
			_ = file.Close()
			_ = os.Remove(file.Name())
		}
		t.Fatal("restore staging succeeded without encryption keyring")
	}
}
