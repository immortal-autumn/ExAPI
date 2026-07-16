package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/security/secretcrypto"
)

const accountCredentialEnvelopeField = "__sub2api_encrypted_credentials"

type accountCredentialProtector struct {
	keyring *secretcrypto.Keyring
}

func newAccountCredentialProtector(keyring *secretcrypto.Keyring) (*accountCredentialProtector, error) {
	if keyring == nil {
		return nil, errors.New("data-encryption keyring is required for account credentials")
	}
	return &accountCredentialProtector{keyring: keyring}, nil
}

func (p *accountCredentialProtector) purpose(accountID int64) (string, error) {
	if accountID <= 0 {
		return "", errors.New("account id is required for credential protection")
	}
	return "accounts/" + strconv.FormatInt(accountID, 10) + "/credentials", nil
}

func (p *accountCredentialProtector) seal(accountID int64, credentials map[string]any) (map[string]any, error) {
	if p == nil || p.keyring == nil {
		return nil, errors.New("data-encryption keyring is required for account credentials")
	}
	purpose, err := p.purpose(accountID)
	if err != nil {
		return nil, err
	}
	canonical := normalizeJSONMap(credentials)
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return nil, fmt.Errorf("marshal account credentials: %w", err)
	}
	envelope, err := p.keyring.Encrypt(purpose, encoded)
	if err != nil {
		return nil, fmt.Errorf("seal account credentials: %w", err)
	}
	return map[string]any{accountCredentialEnvelopeField: envelope}, nil
}

func (p *accountCredentialProtector) open(accountID int64, stored map[string]any) (map[string]any, bool, error) {
	if p == nil || p.keyring == nil {
		return nil, false, errors.New("data-encryption keyring is required for account credentials")
	}
	if len(stored) != 1 {
		return nil, false, errors.New("invalid account credential envelope shape")
	}
	raw, ok := stored[accountCredentialEnvelopeField].(string)
	if !ok || strings.TrimSpace(raw) == "" || raw != strings.TrimSpace(raw) {
		return nil, false, errors.New("invalid account credential envelope")
	}
	purpose, err := p.purpose(accountID)
	if err != nil {
		return nil, false, err
	}
	plaintext, err := p.keyring.Decrypt(purpose, raw)
	if err != nil {
		return nil, false, fmt.Errorf("open account credential envelope: %w", err)
	}
	var credentials map[string]any
	if err := json.Unmarshal(plaintext, &credentials); err != nil {
		return nil, false, errors.New("invalid decrypted account credential JSON")
	}
	if credentials == nil {
		credentials = map[string]any{}
	}
	rewrap, err := p.keyring.NeedsRewrap(raw)
	if err != nil {
		return nil, false, err
	}
	return credentials, rewrap, nil
}

func (p *accountCredentialProtector) openLegacy(accountID int64, stored map[string]any) (map[string]any, bool, error) {
	if _, exists := stored[accountCredentialEnvelopeField]; exists {
		return p.open(accountID, stored)
	}
	return copyJSONMap(stored), true, nil
}
