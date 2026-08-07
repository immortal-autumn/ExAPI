package service

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

func (s *BackupService) uploadEncryptedDump(ctx context.Context, objectStore BackupObjectStore, objectKey string) (int64, error) {
	if s.backupCipher == nil {
		return 0, fmt.Errorf("backup encryption keyring is not configured")
	}
	dumpReader, err := s.dumper.Dump(ctx)
	if err != nil {
		return 0, fmt.Errorf("pg_dump: %w", err)
	}
	staged, stagedSize, err := stageEncryptedBackup(s.backupCipher, dumpReader)
	if err != nil {
		return 0, err
	}
	defer cleanupStagedFile(staged)

	// PutObject success is the transport acknowledgement. The interface's byte
	// count is not an independent integrity signal (the S3 adapter necessarily
	// derives it from Content-Length), so durable integrity is provided by the
	// authenticated encrypted stream during restore.
	if _, err := objectStore.Upload(ctx, objectKey, staged, stagedSize, "application/octet-stream"); err != nil {
		return 0, fmt.Errorf("S3 upload: %w", err)
	}
	return stagedSize, nil
}

func (s *BackupService) restoreEncryptedDump(ctx context.Context, objectStore BackupObjectStore, objectKey string) error {
	if s.backupCipher == nil {
		return fmt.Errorf("backup encryption keyring is not configured")
	}
	body, err := objectStore.Download(ctx, objectKey)
	if err != nil {
		return fmt.Errorf("S3 download: %w", err)
	}
	staged, decryptErr := stageDecryptedBackup(s.backupCipher, body)
	closeErr := body.Close()
	if decryptErr != nil {
		return decryptErr
	}
	if closeErr != nil {
		cleanupStagedFile(staged)
		return fmt.Errorf("close S3 download: %w", closeErr)
	}
	defer cleanupStagedFile(staged)

	gzipReader, err := gzip.NewReader(staged)
	if err != nil {
		return fmt.Errorf("open authenticated gzip backup: %w", err)
	}
	restoreErr := s.dumper.Restore(ctx, gzipReader)
	gzipCloseErr := gzipReader.Close()
	if restoreErr != nil {
		return fmt.Errorf("pg restore: %w", restoreErr)
	}
	if gzipCloseErr != nil && gzipCloseErr != io.EOF {
		return fmt.Errorf("close gzip backup: %w", gzipCloseErr)
	}
	return nil
}

func (s *BackupService) restoreBackupRecord(ctx context.Context, objectStore BackupObjectStore, record *BackupRecord) error {
	if record == nil {
		return ErrBackupFormatUnsupported
	}
	switch record.Format {
	case backupFormatEncryptedV1:
		return s.restoreEncryptedDump(ctx, objectStore, record.S3Key)
	case "":
		if !isRecognizedLegacyBackup(record) {
			return ErrBackupFormatUnsupported
		}
		if !config.LegacyPlaintextBackupRestoreEnabled() {
			return ErrLegacyBackupRestoreDisabled
		}
		return s.restoreLegacyGzipDump(ctx, objectStore, record.S3Key)
	default:
		return ErrBackupFormatUnsupported
	}
}

func isRecognizedLegacyBackup(record *BackupRecord) bool {
	if record == nil || record.Format != "" {
		return false
	}
	return strings.HasSuffix(record.FileName, ".sql.gz") &&
		!strings.HasSuffix(record.FileName, ".sql.gz.enc") &&
		strings.HasSuffix(record.S3Key, ".sql.gz") &&
		!strings.HasSuffix(record.S3Key, ".sql.gz.enc")
}

// restoreLegacyGzipDump is an explicit compatibility path for records created
// before encrypted backup format metadata existed. It validates and fully
// expands gzip into a mode-0600 staging file before invoking psql.
func (s *BackupService) restoreLegacyGzipDump(ctx context.Context, objectStore BackupObjectStore, objectKey string) error {
	body, err := objectStore.Download(ctx, objectKey)
	if err != nil {
		return fmt.Errorf("S3 download legacy backup: %w", err)
	}
	staged, stageErr := stageLegacySQLBackup(body)
	closeErr := body.Close()
	if stageErr != nil {
		return stageErr
	}
	if closeErr != nil {
		cleanupStagedFile(staged)
		return fmt.Errorf("close legacy S3 download: %w", closeErr)
	}
	defer cleanupStagedFile(staged)
	if err := s.dumper.Restore(ctx, staged); err != nil {
		return fmt.Errorf("pg restore legacy backup: %w", err)
	}
	return nil
}

func stageLegacySQLBackup(compressed io.Reader) (_ *os.File, retErr error) {
	return stageLegacySQLBackupWithLimit(compressed, defaultBackupStagingMaxBytes)
}

func stageLegacySQLBackupWithLimit(compressed io.Reader, maxBytes int64) (_ *os.File, retErr error) {
	if compressed == nil {
		return nil, fmt.Errorf("legacy backup reader is nil")
	}
	staged, err := createSecureBackupStagingFile("sub2api-legacy-restore-*.sql")
	if err != nil {
		return nil, fmt.Errorf("create legacy restore staging file: %w", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			cleanupStagedFile(staged)
		}
	}()
	gzipReader, err := gzip.NewReader(compressed)
	if err != nil {
		return nil, fmt.Errorf("open legacy gzip backup: %w", err)
	}
	if maxBytes <= 0 {
		return nil, fmt.Errorf("invalid backup staging size limit")
	}
	limited := &limitedStagingWriter{writer: staged, remaining: maxBytes}
	_, copyErr := io.Copy(limited, gzipReader)
	closeErr := gzipReader.Close()
	if copyErr != nil {
		return nil, fmt.Errorf("validate legacy gzip backup: %w", copyErr)
	}
	if closeErr != nil && closeErr != io.EOF {
		return nil, fmt.Errorf("close legacy gzip backup: %w", closeErr)
	}
	if err := staged.Sync(); err != nil {
		return nil, fmt.Errorf("sync legacy restore staging file: %w", err)
	}
	if _, err := staged.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("rewind legacy restore staging file: %w", err)
	}
	cleanup = false
	return staged, nil
}
