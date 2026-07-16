package repository

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/security/secretcrypto"
)

const gatewayAPIKeyDigestPurpose = "gateway.api_key"

var errGatewayAPIKeyDigestRequired = errors.New("gateway API-key digest keyring is required")

// gatewayAPIKeyDigester owns the one-way verifier domain for inbound gateway
// keys. It deliberately has no method that can recover the original key.
type gatewayAPIKeyDigester struct {
	keyring *secretcrypto.Keyring
}

func loadGatewayAPIKeyDigester() (*gatewayAPIKeyDigester, error) {
	keyrings, err := config.LoadExternalKeyringsFromEnv()
	if err != nil {
		return nil, fmt.Errorf("load gateway API-key digest keyring: %w", err)
	}
	return newGatewayAPIKeyDigester(keyrings.GatewayKeyDigest)
}

func newGatewayAPIKeyDigester(keyring *secretcrypto.Keyring) (*gatewayAPIKeyDigester, error) {
	if keyring == nil {
		return nil, errGatewayAPIKeyDigestRequired
	}
	return &gatewayAPIKeyDigester{keyring: keyring}, nil
}

func (d *gatewayAPIKeyDigester) Digest(raw string) (string, error) {
	if d == nil || d.keyring == nil {
		return "", errGatewayAPIKeyDigestRequired
	}
	if strings.TrimSpace(raw) == "" {
		return "", errors.New("gateway API key is empty")
	}
	return d.keyring.Digest(gatewayAPIKeyDigestPurpose, []byte(raw))
}

func (d *gatewayAPIKeyDigester) CandidateDigests(raw string) ([]string, error) {
	if d == nil || d.keyring == nil {
		return nil, errGatewayAPIKeyDigestRequired
	}
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("gateway API key is empty")
	}
	return d.keyring.DigestCandidates(gatewayAPIKeyDigestPurpose, []byte(raw))
}

func (d *gatewayAPIKeyDigester) Verify(raw, encoded string) bool {
	return d != nil && d.keyring != nil && d.keyring.VerifyDigest(gatewayAPIKeyDigestPurpose, []byte(raw), encoded)
}

func (d *gatewayAPIKeyDigester) IsDigest(encoded string) bool {
	return d != nil && d.keyring != nil && d.keyring.IsDigest(encoded)
}

func gatewayAPIKeyDisplayPrefix(raw string) string {
	const maxRunes = 12
	runes := []rune(raw)
	if len(runes) > maxRunes {
		runes = runes[:maxRunes]
	}
	return string(runes)
}

func gatewayAPIKeyStoragePlaceholder(digest string) string {
	// The legacy NOT NULL/UNIQUE key column remains during the rollback window.
	// Store only a deterministic non-secret identifier derived from the verifier.
	const prefix = "__hmac__"
	sum := sha256.Sum256([]byte(digest))
	return prefix + hex.EncodeToString(sum[:])
}
