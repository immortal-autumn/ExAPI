package config

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func encodedKey(fill byte) string {
	return base64.RawStdEncoding.EncodeToString(bytes.Repeat([]byte{fill}, 32))
}

func encodedKeyMap(t *testing.T, values map[string]string) string {
	t.Helper()
	raw, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func clearExternalKeyringEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		EnvDataEncryptionActiveKeyID, EnvDataEncryptionKeysJSON,
		EnvGatewayKeyDigestActiveKeyID, EnvGatewayKeyDigestKeysJSON,
		EnvBackupEncryptionActiveKeyID, EnvBackupEncryptionKeysJSON,
	} {
		t.Setenv(name, "")
	}
}

func TestLoadExternalKeyringsAllowsAllDomainsUnset(t *testing.T) {
	clearExternalKeyringEnv(t)

	keyrings, err := LoadExternalKeyringsFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if keyrings.DataEncryption != nil || keyrings.GatewayKeyDigest != nil || keyrings.BackupEncryption != nil {
		t.Fatal("unset external keyrings should remain nil")
	}
}

func TestLoadExternalKeyringsBuildsIndependentDomains(t *testing.T) {
	clearExternalKeyringEnv(t)
	t.Setenv(EnvDataEncryptionActiveKeyID, "data-2026-01")
	t.Setenv(EnvDataEncryptionKeysJSON, encodedKeyMap(t, map[string]string{"data-2025-01": encodedKey(1), "data-2026-01": encodedKey(2)}))
	t.Setenv(EnvGatewayKeyDigestActiveKeyID, "digest-2026-01")
	t.Setenv(EnvGatewayKeyDigestKeysJSON, encodedKeyMap(t, map[string]string{"digest-2026-01": encodedKey(3)}))
	t.Setenv(EnvBackupEncryptionActiveKeyID, "backup-2026-01")
	t.Setenv(EnvBackupEncryptionKeysJSON, encodedKeyMap(t, map[string]string{"backup-2026-01": encodedKey(4)}))

	keyrings, err := LoadExternalKeyringsFromEnv()
	if err != nil {
		t.Fatal(err)
	}

	envelope, err := keyrings.DataEncryption.Encrypt("account.credentials", []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(envelope, "enc:v1:data-2026-01:") {
		t.Fatalf("data envelope uses wrong active key: %q", envelope)
	}
	digest, err := keyrings.GatewayKeyDigest.Digest("gateway.api_key", []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(digest, "hmac:v1:digest-2026-01:") {
		t.Fatalf("gateway digest uses wrong active key: %q", digest)
	}
	backupEnvelope, err := keyrings.BackupEncryption.Encrypt("backup.stream", []byte("dump"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(backupEnvelope, "enc:v1:backup-2026-01:") {
		t.Fatalf("backup envelope uses wrong active key: %q", backupEnvelope)
	}
}

func TestLoadExternalKeyringsRejectsPartialOrMalformedDomain(t *testing.T) {
	tests := []struct {
		name   string
		active string
		keys   string
	}{
		{name: "active_without_keys", active: "data-1"},
		{name: "keys_without_active", keys: encodedKeyMap(t, map[string]string{"data-1": encodedKey(1)})},
		{name: "malformed_json", active: "data-1", keys: "not-json"},
		{name: "invalid_base64", active: "data-1", keys: encodedKeyMap(t, map[string]string{"data-1": "***"})},
		{name: "wrong_length", active: "data-1", keys: encodedKeyMap(t, map[string]string{"data-1": base64.RawStdEncoding.EncodeToString([]byte("short"))})},
		{name: "missing_active_entry", active: "data-2", keys: encodedKeyMap(t, map[string]string{"data-1": encodedKey(1)})},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clearExternalKeyringEnv(t)
			t.Setenv(EnvDataEncryptionActiveKeyID, tc.active)
			t.Setenv(EnvDataEncryptionKeysJSON, tc.keys)
			if _, err := LoadExternalKeyringsFromEnv(); err == nil {
				t.Fatal("invalid external keyring configuration accepted")
			}
		})
	}
}

func TestLoadExternalKeyringsRejectsCrossDomainKeyReuse(t *testing.T) {
	clearExternalKeyringEnv(t)
	shared := encodedKey(7)
	t.Setenv(EnvDataEncryptionActiveKeyID, "data-1")
	t.Setenv(EnvDataEncryptionKeysJSON, encodedKeyMap(t, map[string]string{"data-1": shared}))
	t.Setenv(EnvGatewayKeyDigestActiveKeyID, "digest-1")
	t.Setenv(EnvGatewayKeyDigestKeysJSON, encodedKeyMap(t, map[string]string{"digest-1": shared}))

	if _, err := LoadExternalKeyringsFromEnv(); err == nil {
		t.Fatal("cross-domain key reuse was accepted")
	}
}
