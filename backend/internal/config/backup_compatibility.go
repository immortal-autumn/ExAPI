package config

import (
	"os"
	"strings"
)

const legacyPlaintextBackupRestoreEnv = "SUB2API_ALLOW_LEGACY_PLAINTEXT_BACKUP_RESTORE"

// LegacyPlaintextBackupRestoreEnabled reports whether an operator has explicitly
// enabled restoration of pre-encryption .sql.gz backup records. It never affects
// records marked as encrypted and therefore cannot act as an authentication
// fallback.
func LegacyPlaintextBackupRestoreEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(legacyPlaintextBackupRestoreEnv))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
