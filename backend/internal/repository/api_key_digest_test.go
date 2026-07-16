package repository

import (
	"bytes"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/security/secretcrypto"
)

func testGatewayDigestKeyring(t *testing.T, active string, keys map[string]byte) *secretcrypto.Keyring {
	t.Helper()
	raw := make(map[string][]byte, len(keys))
	for id, fill := range keys {
		raw[id] = bytes.Repeat([]byte{fill}, 32)
	}
	ring, err := secretcrypto.NewKeyring(active, raw)
	if err != nil {
		t.Fatal(err)
	}
	return ring
}

func mustGatewayAPIKeyDigesterForTest(t *testing.T) *gatewayAPIKeyDigester {
	t.Helper()
	d, err := newGatewayAPIKeyDigester(testGatewayDigestKeyring(t, "test-digest", map[string]byte{"test-digest": 0x42}))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestGatewayAPIKeyDigesterFailsClosedWithoutKeyring(t *testing.T) {
	if _, err := newGatewayAPIKeyDigester(nil); err == nil {
		t.Fatal("missing gateway digest keyring was accepted")
	}
}

func TestGatewayAPIKeyDigesterProducesOneWayPurposeBoundVerifier(t *testing.T) {
	d, err := newGatewayAPIKeyDigester(testGatewayDigestKeyring(t, "k1", map[string]byte{"k1": 1}))
	if err != nil {
		t.Fatal(err)
	}

	const raw = "test-gateway-key-value"
	encoded, err := d.Digest(raw)
	if err != nil {
		t.Fatal(err)
	}
	if encoded == raw || bytes.Contains([]byte(encoded), []byte(raw)) {
		t.Fatal("verifier contains reusable API-key material")
	}
	if !d.IsDigest(encoded) || !d.Verify(raw, encoded) {
		t.Fatal("valid verifier did not authenticate")
	}
	if d.Verify("wrong", encoded) || d.IsDigest(raw) {
		t.Fatal("wrong or plaintext key was accepted")
	}
}

func TestGatewayAPIKeyDigesterSupportsRetainedKeysDuringRotation(t *testing.T) {
	old, err := newGatewayAPIKeyDigester(testGatewayDigestKeyring(t, "old", map[string]byte{"old": 2}))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := old.Digest("rotation-candidate")
	if err != nil {
		t.Fatal(err)
	}

	rotated, err := newGatewayAPIKeyDigester(testGatewayDigestKeyring(t, "new", map[string]byte{"old": 2, "new": 3}))
	if err != nil {
		t.Fatal(err)
	}
	if !rotated.Verify("rotation-candidate", encoded) {
		t.Fatal("retained key could not verify pre-rotation digest")
	}
	candidates, err := rotated.CandidateDigests("rotation-candidate")
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 2 || !containsString(candidates, encoded) {
		t.Fatalf("lookup candidates do not include retained-key verifier: %v", candidates)
	}
	fresh, err := rotated.Digest("rotation-candidate")
	if err != nil {
		t.Fatal(err)
	}
	if fresh == encoded {
		t.Fatal("new writes did not use the active digest key")
	}
	if !containsString(candidates, fresh) {
		t.Fatal("lookup candidates do not include active-key verifier")
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
