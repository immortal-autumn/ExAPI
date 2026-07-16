package secretcrypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	streamChunkSize = 64 * 1024
	streamFlagData  = byte(0)
	streamFlagFinal = byte(1)
)

var streamMagic = []byte("S2BKENC1")

// EncryptStream writes a framed, chunked AES-256-GCM stream. Every record is
// bound to its purpose, key ID, sequence number, and record type. A final
// authenticated record makes truncation detectable without buffering the dump.
func (k *Keyring) EncryptStream(purpose string, dst io.Writer, src io.Reader) error {
	if k == nil || strings.TrimSpace(purpose) == "" || dst == nil || src == nil {
		return errors.New("stream encryption requires keyring, purpose, source, and destination")
	}
	key, ok := k.keys[k.activeID]
	if !ok {
		return errors.New("active stream encryption key is unavailable")
	}
	aead, err := newGCM(key)
	if err != nil {
		return err
	}
	if err := writeStreamHeader(dst, k.activeID); err != nil {
		return err
	}

	buf := make([]byte, streamChunkSize)
	var index uint64
	for {
		n, readErr := io.ReadFull(src, buf)
		if n > 0 {
			if err := writeEncryptedRecord(dst, aead, k.activeID, purpose, index, streamFlagData, buf[:n]); err != nil {
				return err
			}
			index++
		}
		switch readErr {
		case nil:
			continue
		case io.EOF, io.ErrUnexpectedEOF:
			return writeEncryptedRecord(dst, aead, k.activeID, purpose, index, streamFlagFinal, nil)
		default:
			return fmt.Errorf("read plaintext stream: %w", readErr)
		}
	}
}

// DecryptStream authenticates and writes a framed stream. On any error callers
// must discard the destination; restore code should decrypt into a temporary
// file and only pass that file to pg_restore after this method succeeds.
func (k *Keyring) DecryptStream(purpose string, dst io.Writer, src io.Reader) error {
	if k == nil || strings.TrimSpace(purpose) == "" || dst == nil || src == nil {
		return errors.New("stream decryption requires keyring, purpose, source, and destination")
	}
	keyID, err := readStreamHeader(src)
	if err != nil {
		return err
	}
	key, ok := k.keys[keyID]
	if !ok {
		return errors.New("stream encryption key id is unavailable")
	}
	aead, err := newGCM(key)
	if err != nil {
		return err
	}
	maxPayload := streamChunkSize + aead.NonceSize() + aead.Overhead()

	var index uint64
	for {
		var recordHeader [5]byte
		if _, err := io.ReadFull(src, recordHeader[:]); err != nil {
			return errors.New("encrypted stream is truncated before final record")
		}
		flag := recordHeader[0]
		if flag != streamFlagData && flag != streamFlagFinal {
			return errors.New("encrypted stream has invalid record type")
		}
		payloadLen := int(binary.BigEndian.Uint32(recordHeader[1:]))
		if payloadLen < aead.NonceSize()+aead.Overhead() || payloadLen > maxPayload {
			return errors.New("encrypted stream has invalid record length")
		}
		payload := make([]byte, payloadLen)
		if _, err := io.ReadFull(src, payload); err != nil {
			return errors.New("encrypted stream record is truncated")
		}
		nonce, ciphertext := payload[:aead.NonceSize()], payload[aead.NonceSize():]
		plaintext, err := aead.Open(nil, nonce, ciphertext, streamAAD(keyID, purpose, index, flag))
		if err != nil {
			return errors.New("encrypted stream authentication failed")
		}

		if flag == streamFlagFinal {
			if len(plaintext) != 0 {
				return errors.New("encrypted stream final record is invalid")
			}
			var trailing [1]byte
			if _, err := io.ReadFull(src, trailing[:]); err != io.EOF {
				if err == nil {
					return errors.New("encrypted stream has trailing data")
				}
				return fmt.Errorf("check encrypted stream trailing data: %w", err)
			}
			return nil
		}

		if len(plaintext) == 0 {
			return errors.New("encrypted stream contains empty data record")
		}
		if err := writeAll(dst, plaintext); err != nil {
			return fmt.Errorf("write decrypted stream: %w", err)
		}
		index++
	}
}

func newGCM(key [32]byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("create stream cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create stream gcm: %w", err)
	}
	return aead, nil
}

func writeStreamHeader(dst io.Writer, keyID string) error {
	header := make([]byte, 0, len(streamMagic)+1+len(keyID))
	header = append(header, streamMagic...)
	header = append(header, byte(len(keyID)))
	header = append(header, keyID...)
	if err := writeAll(dst, header); err != nil {
		return fmt.Errorf("write encrypted stream header: %w", err)
	}
	return nil
}

func readStreamHeader(src io.Reader) (string, error) {
	magic := make([]byte, len(streamMagic))
	if _, err := io.ReadFull(src, magic); err != nil || string(magic) != string(streamMagic) {
		return "", errors.New("invalid encrypted stream header")
	}
	var keyIDLen [1]byte
	if _, err := io.ReadFull(src, keyIDLen[:]); err != nil || keyIDLen[0] == 0 || keyIDLen[0] > 64 {
		return "", errors.New("invalid encrypted stream key id")
	}
	keyIDBytes := make([]byte, int(keyIDLen[0]))
	if _, err := io.ReadFull(src, keyIDBytes); err != nil {
		return "", errors.New("invalid encrypted stream key id")
	}
	keyID := string(keyIDBytes)
	if !keyIDPattern.MatchString(keyID) {
		return "", errors.New("invalid encrypted stream key id")
	}
	return keyID, nil
}

func writeEncryptedRecord(dst io.Writer, aead cipher.AEAD, keyID, purpose string, index uint64, flag byte, plaintext []byte) error {
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return fmt.Errorf("generate stream nonce: %w", err)
	}
	payload := aead.Seal(nonce, nonce, plaintext, streamAAD(keyID, purpose, index, flag))
	var header [5]byte
	header[0] = flag
	binary.BigEndian.PutUint32(header[1:], uint32(len(payload)))
	if err := writeAll(dst, header[:]); err != nil {
		return fmt.Errorf("write encrypted record header: %w", err)
	}
	if err := writeAll(dst, payload); err != nil {
		return fmt.Errorf("write encrypted record: %w", err)
	}
	return nil
}

func streamAAD(keyID, purpose string, index uint64, flag byte) []byte {
	aad := make([]byte, 0, len(streamMagic)+1+len(keyID)+2+len(purpose)+8+1)
	aad = append(aad, streamMagic...)
	aad = append(aad, byte(len(keyID)))
	aad = append(aad, keyID...)
	var purposeLen [2]byte
	binary.BigEndian.PutUint16(purposeLen[:], uint16(len(purpose)))
	aad = append(aad, purposeLen[:]...)
	aad = append(aad, purpose...)
	var indexBytes [8]byte
	binary.BigEndian.PutUint64(indexBytes[:], index)
	aad = append(aad, indexBytes[:]...)
	aad = append(aad, flag)
	return aad
}

func writeAll(dst io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := dst.Write(data)
		if err != nil {
			return err
		}
		if n <= 0 || n > len(data) {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}
