package repository

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/security/secretcrypto"
)

const securitySecretEnvelopePrefix = "enc:"

type securitySecretProtector struct {
	keyring *secretcrypto.Keyring
}

func newSecuritySecretProtector(keyring *secretcrypto.Keyring) (*securitySecretProtector, error) {
	if keyring == nil {
		return nil, errors.New("data-encryption keyring is required")
	}
	return &securitySecretProtector{keyring: keyring}, nil
}

func securitySecretPurpose(key string) (string, error) {
	canonical := strings.TrimSpace(key)
	if key != canonical || canonical == "" || strings.ContainsAny(canonical, "/\\") {
		return "", errors.New("invalid security-secret key")
	}
	// The exact unique logical key identifies the row, so the purpose binds the
	// envelope to table, row, and field without aliasing whitespace-distinct rows.
	return fmt.Sprintf("security_secrets/%s/value", key), nil
}

func (p *securitySecretProtector) seal(key, plaintext string) (string, error) {
	if p == nil || p.keyring == nil {
		return "", errors.New("data-encryption keyring is required")
	}
	purpose, err := securitySecretPurpose(key)
	if err != nil {
		return "", err
	}
	return p.keyring.Encrypt(purpose, []byte(plaintext))
}

// open returns plaintext and whether the stored value should be rewritten
// under the active key. Plaintext is accepted only as the explicit legacy
// migration format; malformed envelope-looking values fail closed.
func (p *securitySecretProtector) open(key, stored string) (plaintext string, rewrite bool, err error) {
	if p == nil || p.keyring == nil {
		return "", false, errors.New("data-encryption keyring is required")
	}
	purpose, err := securitySecretPurpose(key)
	if err != nil {
		return "", false, err
	}
	canonicalStored := strings.TrimSpace(stored)
	if !strings.HasPrefix(canonicalStored, securitySecretEnvelopePrefix) {
		return canonicalStored, true, nil
	}
	if stored != canonicalStored {
		return "", false, errors.New("security-secret envelope must not contain surrounding whitespace")
	}
	opened, err := p.keyring.Decrypt(purpose, canonicalStored)
	if err != nil {
		return "", false, fmt.Errorf("open security-secret envelope: %w", err)
	}
	rewrite, err = p.keyring.NeedsRewrap(canonicalStored)
	if err != nil {
		return "", false, fmt.Errorf("inspect security-secret envelope: %w", err)
	}
	return string(opened), rewrite, nil
}
