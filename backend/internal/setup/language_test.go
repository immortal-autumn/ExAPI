package setup

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetupUserFacingMessagesAreChinese(t *testing.T) {
	paths := []string{"handler.go", "cli.go"}
	forbidden := []string{
		"Setup is not allowed: system is already installed",
		"password must be at least 8 characters",
		"password must be at most 128 characters",
		"Connection successful",
		"ExAPI Installation Wizard",
		"Proceed with installation?",
	}

	for _, path := range paths {
		content, err := os.ReadFile(path)
		require.NoError(t, err)
		for _, text := range forbidden {
			require.Falsef(t, strings.Contains(string(content), text), "%s still contains user-facing English: %s", path, text)
		}
	}
}
