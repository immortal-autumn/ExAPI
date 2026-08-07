package service

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

const (
	backupStreamPurpose          = "backup.postgres.sql.gz"
	defaultBackupStagingMaxBytes = int64(20 << 30) // 20 GiB fail-safe cap
)

var (
	ErrBackupStagingTooLarge            = errors.New("backup staging size limit exceeded")
	ErrInsecureBackupStagingPermissions = errors.New("backup staging filesystem cannot enforce mode 0600")
)

// createSecureBackupStagingFile verifies the filesystem contract before any
// sensitive bytes are written. Some removable/FUSE filesystems accept chmod
// while continuing to expose every file as mode 0777; backup staging must fail
// closed on those filesystems.
func createSecureBackupStagingFile(pattern string) (*os.File, error) {
	file, err := os.CreateTemp("", pattern)
	if err != nil {
		return nil, err
	}
	cleanup := true
	defer func() {
		if cleanup {
			cleanupStagedFile(file)
		}
	}()
	if err := file.Chmod(0o600); err != nil {
		return nil, fmt.Errorf("set backup staging permissions: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("verify backup staging permissions: %w", err)
	}
	if info.Mode().Perm() != 0o600 {
		return nil, fmt.Errorf("%w: got mode %04o", ErrInsecureBackupStagingPermissions, info.Mode().Perm())
	}
	cleanup = false
	return file, nil
}

type limitedStagingWriter struct {
	writer    io.Writer
	remaining int64
}

func (w *limitedStagingWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > w.remaining {
		if w.remaining > 0 {
			n, err := w.writer.Write(p[:w.remaining])
			w.remaining -= int64(n)
			if err != nil {
				return n, err
			}
		}
		return 0, ErrBackupStagingTooLarge
	}
	n, err := w.writer.Write(p)
	w.remaining -= int64(n)
	return n, err
}

// BackupStreamCipher is implemented by the external backup-encryption keyring.
// It is intentionally separate from the encryptor used for settings/TOTP data.
type BackupStreamCipher interface {
	EncryptStream(purpose string, dst io.Writer, src io.Reader) error
	DecryptStream(purpose string, dst io.Writer, src io.Reader) error
}

// stageEncryptedBackup compresses a dump into a mode-0600 authenticated
// temporary file. The returned file is rewound and owned by the caller.
func stageEncryptedBackup(cipher BackupStreamCipher, dump io.ReadCloser) (_ *os.File, size int64, retErr error) {
	if cipher == nil {
		if dump != nil {
			_ = dump.Close()
		}
		return nil, 0, fmt.Errorf("backup encryption keyring is not configured")
	}
	if dump == nil {
		return nil, 0, fmt.Errorf("database dump reader is nil")
	}

	staged, err := createSecureBackupStagingFile("sub2api-backup-*.sql.gz.enc")
	if err != nil {
		_ = dump.Close()
		return nil, 0, fmt.Errorf("create encrypted backup staging file: %w", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			cleanupStagedFile(staged)
		}
	}()

	compressedReader, compressedWriter := io.Pipe()
	producerDone := make(chan error, 1)
	go func() {
		var producerErr error
		defer func() {
			if recovered := recover(); recovered != nil {
				producerErr = fmt.Errorf("backup compression panic: %v", recovered)
			}
			if closeErr := dump.Close(); closeErr != nil && producerErr == nil {
				producerErr = closeErr
			}
			if producerErr != nil {
				_ = compressedWriter.CloseWithError(producerErr)
			} else {
				_ = compressedWriter.Close()
			}
			producerDone <- producerErr
		}()

		gzipWriter := gzip.NewWriter(compressedWriter)
		if _, err := io.Copy(gzipWriter, dump); err != nil {
			producerErr = err
		}
		if closeErr := gzipWriter.Close(); closeErr != nil && producerErr == nil {
			producerErr = closeErr
		}
	}()

	encryptErr := cipher.EncryptStream(backupStreamPurpose, staged, compressedReader)
	if encryptErr != nil {
		_ = compressedReader.CloseWithError(encryptErr)
	} else {
		_ = compressedReader.Close()
	}
	producerErr := <-producerDone
	if encryptErr != nil {
		return nil, 0, fmt.Errorf("encrypt backup stream: %w", encryptErr)
	}
	if producerErr != nil {
		return nil, 0, fmt.Errorf("compress database dump: %w", producerErr)
	}
	if err := staged.Sync(); err != nil {
		return nil, 0, fmt.Errorf("sync encrypted backup staging file: %w", err)
	}
	info, err := staged.Stat()
	if err != nil {
		return nil, 0, fmt.Errorf("stat encrypted backup staging file: %w", err)
	}
	if _, err := staged.Seek(0, io.SeekStart); err != nil {
		return nil, 0, fmt.Errorf("rewind encrypted backup staging file: %w", err)
	}

	cleanup = false
	return staged, info.Size(), nil
}

// stageDecryptedBackup fully authenticates an encrypted object into a mode-0600
// temporary gzip file before returning it. Restore code must not invoke psql
// until this function has succeeded.
func stageDecryptedBackup(cipher BackupStreamCipher, encrypted io.Reader) (_ *os.File, retErr error) {
	return stageDecryptedBackupWithLimit(cipher, encrypted, defaultBackupStagingMaxBytes)
}

func stageDecryptedBackupWithLimit(cipher BackupStreamCipher, encrypted io.Reader, maxBytes int64) (_ *os.File, retErr error) {
	if cipher == nil {
		return nil, fmt.Errorf("backup encryption keyring is not configured")
	}
	if encrypted == nil {
		return nil, fmt.Errorf("encrypted backup reader is nil")
	}

	staged, err := createSecureBackupStagingFile("sub2api-restore-*.sql.gz")
	if err != nil {
		return nil, fmt.Errorf("create restore staging file: %w", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			cleanupStagedFile(staged)
		}
	}()

	if maxBytes <= 0 {
		return nil, fmt.Errorf("invalid backup staging size limit")
	}
	limited := &limitedStagingWriter{writer: staged, remaining: maxBytes}
	if err := cipher.DecryptStream(backupStreamPurpose, limited, encrypted); err != nil {
		return nil, fmt.Errorf("authenticate and decrypt backup: %w", err)
	}
	if err := staged.Sync(); err != nil {
		return nil, fmt.Errorf("sync restore staging file: %w", err)
	}
	if _, err := staged.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("rewind restore staging file: %w", err)
	}

	cleanup = false
	return staged, nil
}

func cleanupStagedFile(file *os.File) {
	if file == nil {
		return
	}
	name := file.Name()
	if err := file.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
		logger.LegacyPrintf("service.backup", "[Backup] close staging file failed: %v", err)
	}
	if err := os.Remove(name); err != nil && !errors.Is(err, os.ErrNotExist) {
		logger.LegacyPrintf("service.backup", "[Backup] remove staging file failed: %v", err)
	}
}
