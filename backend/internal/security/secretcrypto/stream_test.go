package secretcrypto

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestEncryptedStreamRoundTripAcrossChunks(t *testing.T) {
	ring, err := NewKeyring("backup-2", map[string][]byte{"backup-1": testKey(1), "backup-2": testKey(2)})
	if err != nil {
		t.Fatal(err)
	}
	plaintext := bytes.Repeat([]byte("pg-dump-row\n"), streamChunkSize/6+37)

	var encrypted bytes.Buffer
	if err := ring.EncryptStream("backup.pg_dump", &encrypted, bytes.NewReader(plaintext)); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encrypted.Bytes(), plaintext) {
		t.Fatal("encrypted stream contains plaintext")
	}
	if !bytes.HasPrefix(encrypted.Bytes(), streamMagic) {
		t.Fatal("encrypted stream has no versioned magic")
	}

	var restored bytes.Buffer
	if err := ring.DecryptStream("backup.pg_dump", &restored, bytes.NewReader(encrypted.Bytes())); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(restored.Bytes(), plaintext) {
		t.Fatal("restored stream differs from plaintext")
	}
}

func TestEncryptedStreamRejectsWrongPurposeTamperingTruncationAndTrailingData(t *testing.T) {
	ring, err := NewKeyring("backup-1", map[string][]byte{"backup-1": testKey(1)})
	if err != nil {
		t.Fatal(err)
	}
	var encrypted bytes.Buffer
	if err := ring.EncryptStream("backup.pg_dump", &encrypted, bytes.NewReader(bytes.Repeat([]byte("data"), 20000))); err != nil {
		t.Fatal(err)
	}
	original := encrypted.Bytes()

	tests := []struct {
		name    string
		purpose string
		mutate  func([]byte) []byte
	}{
		{name: "wrong_purpose", purpose: "backup.other", mutate: func(in []byte) []byte { return in }},
		{name: "tampered", purpose: "backup.pg_dump", mutate: func(in []byte) []byte {
			out := append([]byte(nil), in...)
			out[len(out)/2] ^= 0x40
			return out
		}},
		{name: "truncated", purpose: "backup.pg_dump", mutate: func(in []byte) []byte { return append([]byte(nil), in[:len(in)-1]...) }},
		{name: "trailing_data", purpose: "backup.pg_dump", mutate: func(in []byte) []byte { return append(append([]byte(nil), in...), 0x01) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var restored bytes.Buffer
			if err := ring.DecryptStream(tc.purpose, &restored, bytes.NewReader(tc.mutate(original))); err == nil {
				t.Fatal("invalid encrypted stream accepted")
			}
		})
	}
}

func TestEncryptedStreamRotationReadsOldKey(t *testing.T) {
	oldRing, err := NewKeyring("backup-1", map[string][]byte{"backup-1": testKey(1)})
	if err != nil {
		t.Fatal(err)
	}
	var encrypted bytes.Buffer
	if err := oldRing.EncryptStream("backup.pg_dump", &encrypted, bytes.NewReader([]byte("old backup"))); err != nil {
		t.Fatal(err)
	}

	rotated, err := NewKeyring("backup-2", map[string][]byte{"backup-1": testKey(1), "backup-2": testKey(2)})
	if err != nil {
		t.Fatal(err)
	}
	var restored bytes.Buffer
	if err := rotated.DecryptStream("backup.pg_dump", &restored, bytes.NewReader(encrypted.Bytes())); err != nil {
		t.Fatal(err)
	}
	if restored.String() != "old backup" {
		t.Fatalf("restored = %q", restored.String())
	}
}

func TestEncryptedStreamHandlesEmptyInput(t *testing.T) {
	ring, err := NewKeyring("backup-1", map[string][]byte{"backup-1": testKey(1)})
	if err != nil {
		t.Fatal(err)
	}
	var encrypted bytes.Buffer
	if err := ring.EncryptStream("backup.pg_dump", &encrypted, bytes.NewReader(nil)); err != nil {
		t.Fatal(err)
	}
	var restored bytes.Buffer
	if err := ring.DecryptStream("backup.pg_dump", &restored, bytes.NewReader(encrypted.Bytes())); err != nil {
		t.Fatal(err)
	}
	if restored.Len() != 0 {
		t.Fatal("empty stream restored non-empty data")
	}
}

func TestEncryptedStreamRejectsRecordSequenceAttacksAndOversizedLengths(t *testing.T) {
	ring, err := NewKeyring("backup-1", map[string][]byte{"backup-1": testKey(1)})
	if err != nil {
		t.Fatal(err)
	}
	var encrypted bytes.Buffer
	plaintext := bytes.Repeat([]byte{0x5a}, streamChunkSize*2+17)
	if err := ring.EncryptStream("backup.pg_dump", &encrypted, bytes.NewReader(plaintext)); err != nil {
		t.Fatal(err)
	}
	header, records := splitEncryptedStreamForTest(t, encrypted.Bytes())
	if len(records) < 4 {
		t.Fatalf("expected at least 3 data records and final, got %d", len(records))
	}
	join := func(parts ...[]byte) []byte {
		out := append([]byte(nil), header...)
		for _, part := range parts {
			out = append(out, part...)
		}
		return out
	}
	allExcept := func(skip int) [][]byte {
		out := make([][]byte, 0, len(records)-1)
		for i := range records {
			if i != skip {
				out = append(out, records[i])
			}
		}
		return out
	}

	mutations := map[string][]byte{
		"reordered":      join(records[1], records[0], records[2], records[3]),
		"duplicated":     join(records[0], records[0], records[1], records[2], records[3]),
		"deleted_middle": join(allExcept(1)...),
		"final_first":    join(records[len(records)-1], records[0], records[1], records[2]),
	}
	oversized := append([]byte(nil), encrypted.Bytes()...)
	firstRecordOffset := len(header)
	binary.BigEndian.PutUint32(oversized[firstRecordOffset+1:firstRecordOffset+5], uint32(streamChunkSize+1024))
	mutations["oversized_length"] = oversized

	for name, mutated := range mutations {
		t.Run(name, func(t *testing.T) {
			var restored bytes.Buffer
			if err := ring.DecryptStream("backup.pg_dump", &restored, bytes.NewReader(mutated)); err == nil {
				t.Fatal("invalid encrypted stream accepted")
			}
		})
	}
}

func splitEncryptedStreamForTest(t *testing.T, encrypted []byte) ([]byte, [][]byte) {
	t.Helper()
	if len(encrypted) < len(streamMagic)+1 {
		t.Fatal("encrypted stream too short")
	}
	headerLen := len(streamMagic) + 1 + int(encrypted[len(streamMagic)])
	if headerLen > len(encrypted) {
		t.Fatal("encrypted stream header truncated")
	}
	header := append([]byte(nil), encrypted[:headerLen]...)
	var records [][]byte
	for offset := headerLen; offset < len(encrypted); {
		if offset+5 > len(encrypted) {
			t.Fatal("record header truncated")
		}
		length := int(binary.BigEndian.Uint32(encrypted[offset+1 : offset+5]))
		end := offset + 5 + length
		if end > len(encrypted) {
			t.Fatal("record payload truncated")
		}
		records = append(records, append([]byte(nil), encrypted[offset:end]...))
		offset = end
	}
	return header, records
}
