package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLegacyPlaintextBackupRestoreEnabled(t *testing.T) {
	for _, value := range []string{"1", "true", "TRUE", " yes ", "on"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv(legacyPlaintextBackupRestoreEnv, value)
			require.True(t, LegacyPlaintextBackupRestoreEnabled())
		})
	}
	for _, value := range []string{"", "0", "false", "no", "off", "unexpected"} {
		t.Run("disabled-"+value, func(t *testing.T) {
			t.Setenv(legacyPlaintextBackupRestoreEnv, value)
			require.False(t, LegacyPlaintextBackupRestoreEnabled())
		})
	}
}
