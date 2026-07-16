package secretcrypto

import (
	"bytes"
	"strings"
	"testing"
)

func testKey(fill byte) []byte { return bytes.Repeat([]byte{fill}, 32) }

func TestEnvelopeRoundTripAndPurposeBinding(t *testing.T) {
	ring, err := NewKeyring("k2", map[string][]byte{"k1": testKey(1), "k2": testKey(2)})
	if err != nil {
		t.Fatal(err)
	}

	first, err := ring.Encrypt("account.credentials", []byte("credential-json"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := ring.Encrypt("account.credentials", []byte("credential-json"))
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("AEAD envelopes must use fresh nonces")
	}
	if !strings.HasPrefix(first, "enc:v1:k2:") {
		t.Fatalf("unexpected envelope prefix: %q", first)
	}

	plaintext, err := ring.Decrypt("account.credentials", first)
	if err != nil {
		t.Fatal(err)
	}
	if string(plaintext) != "credential-json" {
		t.Fatalf("plaintext = %q", plaintext)
	}
	if _, err := ring.Decrypt("proxy.password", first); err == nil {
		t.Fatal("decrypt with wrong purpose succeeded")
	}
}

func TestEnvelopeRotationDecryptsOldKeyAndWritesActiveKey(t *testing.T) {
	oldRing, err := NewKeyring("k1", map[string][]byte{"k1": testKey(1)})
	if err != nil {
		t.Fatal(err)
	}
	oldEnvelope, err := oldRing.Encrypt("settings.smtp_password", []byte("smtp-secret"))
	if err != nil {
		t.Fatal(err)
	}

	rotated, err := NewKeyring("k2", map[string][]byte{"k1": testKey(1), "k2": testKey(2)})
	if err != nil {
		t.Fatal(err)
	}
	plaintext, err := rotated.Decrypt("settings.smtp_password", oldEnvelope)
	if err != nil {
		t.Fatal(err)
	}
	if string(plaintext) != "smtp-secret" {
		t.Fatalf("plaintext = %q", plaintext)
	}
	newEnvelope, err := rotated.Encrypt("settings.smtp_password", plaintext)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(newEnvelope, "enc:v1:k2:") {
		t.Fatalf("new envelope does not use active key: %q", newEnvelope)
	}
}

func TestVersionedDigestVerificationAndRotation(t *testing.T) {
	ring, err := NewKeyring("h2", map[string][]byte{"h1": testKey(3), "h2": testKey(4)})
	if err != nil {
		t.Fatal(err)
	}

	digest, err := ring.Digest("gateway.api_key", []byte("gateway-secret"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(digest, "hmac:v1:h2:") {
		t.Fatalf("unexpected digest prefix: %q", digest)
	}
	if strings.Contains(digest, "gateway-secret") {
		t.Fatal("digest contains plaintext")
	}
	if !ring.VerifyDigest("gateway.api_key", []byte("gateway-secret"), digest) {
		t.Fatal("correct key did not verify")
	}
	if !ring.IsDigest(digest) {
		t.Fatal("valid digest was not recognized")
	}
	if ring.IsDigest("gateway-secret") || ring.IsDigest("hmac:v1:missing:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA") {
		t.Fatal("plaintext or unavailable-key digest was recognized")
	}
	if ring.VerifyDigest("gateway.api_key", []byte("wrong"), digest) {
		t.Fatal("wrong key verified")
	}
	if ring.VerifyDigest("other-purpose", []byte("gateway-secret"), digest) {
		t.Fatal("wrong purpose verified")
	}

	oldRing, err := NewKeyring("h1", map[string][]byte{"h1": testKey(3)})
	if err != nil {
		t.Fatal(err)
	}
	oldDigest, err := oldRing.Digest("gateway.api_key", []byte("gateway-secret"))
	if err != nil {
		t.Fatal(err)
	}
	if !ring.VerifyDigest("gateway.api_key", []byte("gateway-secret"), oldDigest) {
		t.Fatal("rotated ring could not verify old digest")
	}
}

func TestKeyringRejectsUnsafeConfigurationAndMalformedInputs(t *testing.T) {
	badConfigs := []struct {
		name   string
		active string
		keys   map[string][]byte
	}{
		{"missing_active", "missing", map[string][]byte{"k1": testKey(1)}},
		{"short_key", "k1", map[string][]byte{"k1": []byte("short")}},
		{"unsafe_id", "bad:key", map[string][]byte{"bad:key": testKey(1)}},
	}
	for _, tc := range badConfigs {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewKeyring(tc.active, tc.keys); err == nil {
				t.Fatal("invalid keyring configuration accepted")
			}
		})
	}

	ring, err := NewKeyring("k1", map[string][]byte{"k1": testKey(1)})
	if err != nil {
		t.Fatal(err)
	}
	for _, malformed := range []string{"", "enc:v2:k1:x", "enc:v1:missing:x", "enc:v1:k1:not-base64!"} {
		if _, err := ring.Decrypt("purpose", malformed); err == nil {
			t.Fatalf("malformed envelope accepted: %q", malformed)
		}
	}
	if ring.VerifyDigest("purpose", []byte("x"), "hmac:v2:k1:x") {
		t.Fatal("malformed digest verified")
	}
}
