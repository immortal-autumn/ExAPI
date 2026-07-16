// Package secretcrypto provides versioned, purpose-bound cryptographic
// envelopes. Callers must use separate key rings for encryption and keyed
// digests so compromise or rotation of one domain does not affect another.
package secretcrypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

const (
	envelopeScheme = "enc"
	digestScheme   = "hmac"
	formatVersion  = "v1"
)

var keyIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

// Keyring keeps one active key for new writes and any older keys needed for
// zero-downtime reads during rotation. Every key is exactly 32 bytes.
type Keyring struct {
	activeID string
	keys     map[string][32]byte
}

// NewKeyring validates and defensively copies key material.
func NewKeyring(activeID string, keys map[string][]byte) (*Keyring, error) {
	if !keyIDPattern.MatchString(activeID) {
		return nil, errors.New("invalid active key id")
	}
	if len(keys) == 0 {
		return nil, errors.New("keyring is empty")
	}

	copied := make(map[string][32]byte, len(keys))
	for id, raw := range keys {
		if !keyIDPattern.MatchString(id) {
			return nil, fmt.Errorf("invalid key id %q", id)
		}
		if len(raw) != 32 {
			return nil, fmt.Errorf("key %q must be 32 bytes", id)
		}
		var key [32]byte
		copy(key[:], raw)
		copied[id] = key
	}
	if _, ok := copied[activeID]; !ok {
		return nil, errors.New("active key is not present in keyring")
	}
	return &Keyring{activeID: activeID, keys: copied}, nil
}

// Encrypt returns an AES-256-GCM envelope using the active key. Purpose is
// authenticated as associated data and must be a stable field/domain name.
func (k *Keyring) Encrypt(purpose string, plaintext []byte) (string, error) {
	if k == nil || strings.TrimSpace(purpose) == "" {
		return "", errors.New("encryption purpose is required")
	}
	key, ok := k.keys[k.activeID]
	if !ok {
		return "", errors.New("active encryption key is unavailable")
	}
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	prefix := strings.Join([]string{envelopeScheme, formatVersion, k.activeID, ""}, ":")
	payload := gcm.Seal(nonce, nonce, plaintext, associatedData(prefix, purpose))
	return prefix + base64.RawURLEncoding.EncodeToString(payload), nil
}

// Decrypt authenticates and opens a versioned envelope using the key ID
// embedded in it. Older keys can remain read-only members during rotation.
func (k *Keyring) Decrypt(purpose, envelope string) ([]byte, error) {
	if k == nil || strings.TrimSpace(purpose) == "" {
		return nil, errors.New("decryption purpose is required")
	}
	parts := strings.SplitN(envelope, ":", 4)
	if len(parts) != 4 || parts[0] != envelopeScheme || parts[1] != formatVersion || !keyIDPattern.MatchString(parts[2]) || parts[3] == "" {
		return nil, errors.New("invalid encryption envelope")
	}
	key, ok := k.keys[parts[2]]
	if !ok {
		return nil, errors.New("encryption key id is unavailable")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[3])
	if err != nil {
		return nil, errors.New("invalid encryption envelope payload")
	}
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}
	if len(payload) < gcm.NonceSize()+gcm.Overhead() {
		return nil, errors.New("invalid encryption envelope payload")
	}
	nonce, ciphertext := payload[:gcm.NonceSize()], payload[gcm.NonceSize():]
	prefix := strings.Join([]string{envelopeScheme, formatVersion, parts[2], ""}, ":")
	plaintext, err := gcm.Open(nil, nonce, ciphertext, associatedData(prefix, purpose))
	if err != nil {
		return nil, errors.New("encryption envelope authentication failed")
	}
	return plaintext, nil
}

// NeedsRewrap reports whether an authenticated envelope was written with a
// retained non-active key. Callers should invoke this only after Decrypt has
// authenticated the envelope successfully.
func (k *Keyring) NeedsRewrap(envelope string) (bool, error) {
	if k == nil {
		return false, errors.New("encryption keyring is required")
	}
	parts := strings.SplitN(envelope, ":", 4)
	if len(parts) != 4 || parts[0] != envelopeScheme || parts[1] != formatVersion || !keyIDPattern.MatchString(parts[2]) || parts[3] == "" {
		return false, errors.New("invalid encryption envelope")
	}
	if _, ok := k.keys[parts[2]]; !ok {
		return false, errors.New("encryption key id is unavailable")
	}
	return parts[2] != k.activeID, nil
}

// Digest returns a versioned HMAC-SHA-256 verifier. Use a dedicated key ring;
// do not share these keys with Encrypt/Decrypt domains.
func (k *Keyring) Digest(purpose string, secret []byte) (string, error) {
	if k == nil || strings.TrimSpace(purpose) == "" {
		return "", errors.New("digest purpose is required")
	}
	key, ok := k.keys[k.activeID]
	if !ok {
		return "", errors.New("active digest key is unavailable")
	}
	sum := keyedDigest(key, purpose, secret)
	prefix := strings.Join([]string{digestScheme, formatVersion, k.activeID, ""}, ":")
	return prefix + base64.RawURLEncoding.EncodeToString(sum), nil
}

// DigestCandidates returns deterministic versioned HMAC verifiers for every
// key retained in the ring, active first. It enables indexed lookup across a
// rotation window without exposing key material or scanning stored rows.
func (k *Keyring) DigestCandidates(purpose string, secret []byte) ([]string, error) {
	if k == nil || strings.TrimSpace(purpose) == "" {
		return nil, errors.New("digest purpose is required")
	}
	if _, ok := k.keys[k.activeID]; !ok {
		return nil, errors.New("active digest key is unavailable")
	}
	ids := make([]string, 0, len(k.keys))
	ids = append(ids, k.activeID)
	for id := range k.keys {
		if id != k.activeID {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids[1:])
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		sum := keyedDigest(k.keys[id], purpose, secret)
		prefix := strings.Join([]string{digestScheme, formatVersion, id, ""}, ":")
		out = append(out, prefix+base64.RawURLEncoding.EncodeToString(sum))
	}
	return out, nil
}

// IsDigest reports whether encoded is a structurally valid versioned HMAC
// verifier whose referenced key is present in this keyring. It does not verify
// a candidate secret.
func (k *Keyring) IsDigest(encoded string) bool {
	if k == nil {
		return false
	}
	parts := strings.SplitN(encoded, ":", 4)
	if len(parts) != 4 || parts[0] != digestScheme || parts[1] != formatVersion || !keyIDPattern.MatchString(parts[2]) || parts[3] == "" {
		return false
	}
	if _, ok := k.keys[parts[2]]; !ok {
		return false
	}
	provided, err := base64.RawURLEncoding.DecodeString(parts[3])
	return err == nil && len(provided) == sha256.Size
}

// VerifyDigest checks a versioned verifier in constant time. Invalid formats,
// unavailable old keys, and mismatches all return false without leaking detail.
func (k *Keyring) VerifyDigest(purpose string, secret []byte, encoded string) bool {
	if k == nil || strings.TrimSpace(purpose) == "" {
		return false
	}
	parts := strings.SplitN(encoded, ":", 4)
	if len(parts) != 4 || parts[0] != digestScheme || parts[1] != formatVersion || !keyIDPattern.MatchString(parts[2]) || parts[3] == "" {
		return false
	}
	key, ok := k.keys[parts[2]]
	if !ok {
		return false
	}
	provided, err := base64.RawURLEncoding.DecodeString(parts[3])
	if err != nil || len(provided) != sha256.Size {
		return false
	}
	expected := keyedDigest(key, purpose, secret)
	return hmac.Equal(provided, expected)
}

func associatedData(prefix, purpose string) []byte {
	return []byte(prefix + "purpose=" + purpose)
}

func keyedDigest(key [32]byte, purpose string, secret []byte) []byte {
	mac := hmac.New(sha256.New, key[:])
	_, _ = mac.Write([]byte(purpose))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write(secret)
	return mac.Sum(nil)
}
