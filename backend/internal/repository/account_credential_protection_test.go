package repository

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func mustAccountCredentialProtectorForTest(t *testing.T) *accountCredentialProtector {
	t.Helper()
	protector, err := newAccountCredentialProtector(testSecuritySecretKeyring(t, "account-data", map[string]byte{"account-data": 0x61}))
	require.NoError(t, err)
	return protector
}

func TestAccountCredentialProtectorRoundTripAndPurposeBinding(t *testing.T) {
	protector := mustAccountCredentialProtectorForTest(t)
	credentials := map[string]any{
		"access_token":  "test-access-token",
		"refresh_token": "test-refresh-token",
		"expires_at":    float64(12345),
		"nested": map[string]any{
			"api_key": "test-nested-key",
		},
	}

	stored, err := protector.seal(41, credentials)
	require.NoError(t, err)
	require.NotContains(t, stored, "test-access-token")
	require.NotContains(t, stored, "test-refresh-token")
	require.NotContains(t, stored, "test-nested-key")

	opened, rewrap, err := protector.open(41, stored)
	require.NoError(t, err)
	require.False(t, rewrap)
	require.Equal(t, credentials, opened)

	_, _, err = protector.open(42, stored)
	require.Error(t, err, "credential envelope must be bound to the account row")
}

func TestAccountCredentialProtectorLegacyPlaintextRequiresExplicitRewrite(t *testing.T) {
	protector := mustAccountCredentialProtectorForTest(t)
	legacy := map[string]any{"api_key": "test-legacy-account-key"}

	opened, rewrite, err := protector.openLegacy(9, legacy)
	require.NoError(t, err)
	require.True(t, rewrite)
	require.Equal(t, legacy, opened)
}

func TestAccountCredentialProtectorReportsRetainedKeyForRewrap(t *testing.T) {
	old, err := newAccountCredentialProtector(testSecuritySecretKeyring(t, "old", map[string]byte{"old": 0x62}))
	require.NoError(t, err)
	stored, err := old.seal(77, map[string]any{"access_token": "test-old-token"})
	require.NoError(t, err)

	rotated, err := newAccountCredentialProtector(testSecuritySecretKeyring(t, "new", map[string]byte{"old": 0x62, "new": 0x63}))
	require.NoError(t, err)
	opened, rewrap, err := rotated.open(77, stored)
	require.NoError(t, err)
	require.True(t, rewrap)
	require.Equal(t, "test-old-token", opened["access_token"])

	fresh, err := rotated.seal(77, opened)
	require.NoError(t, err)
	_, rewrap, err = rotated.open(77, fresh)
	require.NoError(t, err)
	require.False(t, rewrap)
}

func TestAccountCredentialProtectorRejectsMalformedEnvelope(t *testing.T) {
	protector := mustAccountCredentialProtectorForTest(t)
	_, _, err := protector.open(7, map[string]any{
		accountCredentialEnvelopeField: "enc:not-valid",
	})
	require.Error(t, err)
}
