package repository

import (
	"bytes"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/security/secretcrypto"
	"github.com/stretchr/testify/require"
)

func testSecuritySecretKeyring(t *testing.T, active string, keys map[string]byte) *secretcrypto.Keyring {
	t.Helper()
	raw := make(map[string][]byte, len(keys))
	for id, fill := range keys {
		raw[id] = bytes.Repeat([]byte{fill}, 32)
	}
	ring, err := secretcrypto.NewKeyring(active, raw)
	require.NoError(t, err)
	return ring
}

func TestSecuritySecretProtectorRoundTripAndPurposeBinding(t *testing.T) {
	protector, err := newSecuritySecretProtector(testSecuritySecretKeyring(t, "k1", map[string]byte{"k1": 0x11}))
	require.NoError(t, err)

	stored, err := protector.seal("jwt_secret", "jwt-value-at-least-32-bytes-long")
	require.NoError(t, err)
	require.NotContains(t, stored, "jwt-value")

	plaintext, rewrap, err := protector.open("jwt_secret", stored)
	require.NoError(t, err)
	require.False(t, rewrap)
	require.Equal(t, "jwt-value-at-least-32-bytes-long", plaintext)

	_, _, err = protector.open("different_secret", stored)
	require.Error(t, err)
}

func TestSecuritySecretProtectorLegacyPlaintextRequiresRewrite(t *testing.T) {
	protector, err := newSecuritySecretProtector(testSecuritySecretKeyring(t, "k1", map[string]byte{"k1": 0x22}))
	require.NoError(t, err)

	plaintext, rewrap, err := protector.open("jwt_secret", "legacy-jwt-secret-at-least-32-bytes")
	require.NoError(t, err)
	require.True(t, rewrap)
	require.Equal(t, "legacy-jwt-secret-at-least-32-bytes", plaintext)
}

func TestSecuritySecretProtectorRejectsMalformedEnvelopeLookingValue(t *testing.T) {
	protector, err := newSecuritySecretProtector(testSecuritySecretKeyring(t, "k1", map[string]byte{"k1": 0x33}))
	require.NoError(t, err)

	_, _, err = protector.open("jwt_secret", "enc:not-a-valid-envelope")
	require.Error(t, err)
}

func TestSecuritySecretProtectorReportsOldKeyForRewrap(t *testing.T) {
	oldProtector, err := newSecuritySecretProtector(testSecuritySecretKeyring(t, "old", map[string]byte{"old": 0x44}))
	require.NoError(t, err)
	stored, err := oldProtector.seal("jwt_secret", "rotating-jwt-secret-at-least-32-bytes")
	require.NoError(t, err)

	rotated, err := newSecuritySecretProtector(testSecuritySecretKeyring(t, "new", map[string]byte{
		"old": 0x44,
		"new": 0x55,
	}))
	require.NoError(t, err)

	plaintext, rewrap, err := rotated.open("jwt_secret", stored)
	require.NoError(t, err)
	require.True(t, rewrap)
	require.Equal(t, "rotating-jwt-secret-at-least-32-bytes", plaintext)
}

func TestSecuritySecretProtectorRejectsNonCanonicalLogicalKey(t *testing.T) {
	protector, err := newSecuritySecretProtector(testSecuritySecretKeyring(t, "data-v1", map[string]byte{"data-v1": 0x11}))
	require.NoError(t, err)

	_, err = protector.seal(" jwt_secret", "0123456789abcdef0123456789abcdef")
	require.Error(t, err)
	_, _, err = protector.open("jwt_secret ", "legacy-secret")
	require.Error(t, err)
}

func TestSecuritySecretProtectorPurposeCannotSwapRows(t *testing.T) {
	protector, err := newSecuritySecretProtector(testSecuritySecretKeyring(t, "data-v1", map[string]byte{"data-v1": 0x11}))
	require.NoError(t, err)

	envelope, err := protector.seal("jwt_secret", "0123456789abcdef0123456789abcdef")
	require.NoError(t, err)
	_, _, err = protector.open("totp_encryption_key", envelope)
	require.Error(t, err)
}

func TestSecuritySecretProtectorRejectsWhitespaceWrappedEnvelope(t *testing.T) {
	protector, err := newSecuritySecretProtector(testSecuritySecretKeyring(t, "data-v1", map[string]byte{"data-v1": 0x11}))
	require.NoError(t, err)

	for _, stored := range []string{" enc:not-valid", "enc:not-valid ", "\tenc:not-valid\n"} {
		_, _, err := protector.open("jwt_secret", stored)
		require.Error(t, err)
	}
}

func TestSecuritySecretProtectorFailsClosedWithoutKeyring(t *testing.T) {
	_, err := newSecuritySecretProtector(nil)
	require.Error(t, err)
}
