//go:build !linux

package privatecutover

import (
	"errors"
	"time"
)

var errSecureBackupPurgeUnsupported = errors.New("secure descriptor-relative backup purge is supported only on Linux")

func secureSnapshotBackupTree(backupRootIdentity, time.Time) ([]backupCandidateIdentity, error) {
	return nil, errSecureBackupPurgeUnsupported
}

func secureValidateBackupRoot(backupRootIdentity) error {
	return errSecureBackupPurgeUnsupported
}

func secureAssertBackupTreeHasNoFiles(backupRootIdentity) error {
	return errSecureBackupPurgeUnsupported
}

func secureBackupPathExists(backupRootIdentity, string) (bool, error) {
	return false, errSecureBackupPurgeUnsupported
}

func secureValidateBackupCandidate(backupRootIdentity, backupCandidateIdentity) error {
	return errSecureBackupPurgeUnsupported
}

func secureConfirmBackupCandidateAbsent(backupRootIdentity, string) error {
	return errSecureBackupPurgeUnsupported
}

func secureRemoveBackupCandidate(backupRootIdentity, backupCandidateIdentity) error {
	return errSecureBackupPurgeUnsupported
}
