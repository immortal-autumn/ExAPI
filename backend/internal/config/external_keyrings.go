package config

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/security/secretcrypto"
)

const (
	EnvDataEncryptionActiveKeyID = "SUB2API_DATA_ENCRYPTION_ACTIVE_KEY_ID"
	EnvDataEncryptionKeysJSON    = "SUB2API_DATA_ENCRYPTION_KEYS_JSON"

	EnvGatewayKeyDigestActiveKeyID = "SUB2API_GATEWAY_KEY_DIGEST_ACTIVE_KEY_ID"
	EnvGatewayKeyDigestKeysJSON    = "SUB2API_GATEWAY_KEY_DIGEST_KEYS_JSON"

	EnvBackupEncryptionActiveKeyID = "SUB2API_BACKUP_ENCRYPTION_ACTIVE_KEY_ID"
	EnvBackupEncryptionKeysJSON    = "SUB2API_BACKUP_ENCRYPTION_KEYS_JSON"
)

// ExternalKeyrings contains independently rooted cryptographic domains. Nil
// fields mean that the corresponding migration or encrypted-backup capability
// has not yet been enabled. Production code must fail closed before using an
// unconfigured domain; it must never synthesize a replacement root.
type ExternalKeyrings struct {
	DataEncryption   *secretcrypto.Keyring
	GatewayKeyDigest *secretcrypto.Keyring
	BackupEncryption *secretcrypto.Keyring
}

type externalKeyringDomain struct {
	name      string
	activeEnv string
	keysEnv   string
}

// LoadExternalKeyringsFromEnv parses external secret roots without placing them
// in Viper's serializable configuration graph. Each domain is all-or-none, and
// identical key material cannot be reused within or across domains.
func LoadExternalKeyringsFromEnv() (*ExternalKeyrings, error) {
	domains := []externalKeyringDomain{
		{name: "data encryption", activeEnv: EnvDataEncryptionActiveKeyID, keysEnv: EnvDataEncryptionKeysJSON},
		{name: "gateway key digest", activeEnv: EnvGatewayKeyDigestActiveKeyID, keysEnv: EnvGatewayKeyDigestKeysJSON},
		{name: "backup encryption", activeEnv: EnvBackupEncryptionActiveKeyID, keysEnv: EnvBackupEncryptionKeysJSON},
	}

	loaded := make([]*secretcrypto.Keyring, len(domains))
	seenMaterial := make(map[[sha256.Size]byte]string)
	for i, domain := range domains {
		ring, fingerprints, err := loadExternalKeyringDomain(domain)
		if err != nil {
			return nil, err
		}
		for fingerprint := range fingerprints {
			if previous, exists := seenMaterial[fingerprint]; exists {
				return nil, fmt.Errorf("external key material is reused between %s and %s", previous, domain.name)
			}
			seenMaterial[fingerprint] = domain.name
		}
		loaded[i] = ring
	}

	return &ExternalKeyrings{
		DataEncryption:   loaded[0],
		GatewayKeyDigest: loaded[1],
		BackupEncryption: loaded[2],
	}, nil
}

func loadExternalKeyringDomain(domain externalKeyringDomain) (*secretcrypto.Keyring, map[[sha256.Size]byte]struct{}, error) {
	activeID := strings.TrimSpace(os.Getenv(domain.activeEnv))
	encodedJSON := strings.TrimSpace(os.Getenv(domain.keysEnv))
	if activeID == "" && encodedJSON == "" {
		return nil, nil, nil
	}
	if activeID == "" || encodedJSON == "" {
		return nil, nil, fmt.Errorf("%s requires both %s and %s", domain.name, domain.activeEnv, domain.keysEnv)
	}

	var encodedKeys map[string]string
	if err := json.Unmarshal([]byte(encodedJSON), &encodedKeys); err != nil {
		return nil, nil, fmt.Errorf("parse %s: invalid JSON", domain.keysEnv)
	}
	if len(encodedKeys) == 0 {
		return nil, nil, fmt.Errorf("%s must contain at least one key", domain.keysEnv)
	}

	keys := make(map[string][]byte, len(encodedKeys))
	fingerprints := make(map[[sha256.Size]byte]struct{}, len(encodedKeys))
	for id, encoded := range encodedKeys {
		raw, err := decodeExternalKey(encoded)
		if err != nil {
			return nil, nil, fmt.Errorf("decode key %q from %s: %w", id, domain.keysEnv, err)
		}
		if len(raw) != 32 {
			return nil, nil, fmt.Errorf("key %q from %s must decode to 32 bytes", id, domain.keysEnv)
		}
		fingerprint := sha256.Sum256(raw)
		if _, duplicate := fingerprints[fingerprint]; duplicate {
			return nil, nil, fmt.Errorf("%s contains duplicate key material", domain.keysEnv)
		}
		fingerprints[fingerprint] = struct{}{}
		keys[id] = raw
	}

	ring, err := secretcrypto.NewKeyring(activeID, keys)
	if err != nil {
		return nil, nil, fmt.Errorf("validate %s: %w", domain.name, err)
	}
	return ring, fingerprints, nil
}

func decodeExternalKey(encoded string) ([]byte, error) {
	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return nil, errors.New("empty base64 value")
	}
	if raw, err := base64.RawStdEncoding.DecodeString(encoded); err == nil {
		return raw, nil
	}
	if raw, err := base64.StdEncoding.DecodeString(encoded); err == nil {
		return raw, nil
	}
	return nil, errors.New("invalid base64 value")
}
