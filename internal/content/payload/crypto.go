// Package payload owns local payload bytes, integrity, encryption, and atomic file mutation.
// It does not own catalogue semantics or peer exchange.
package payload

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"

	model "ardents/internal/content/catalog"
)

const AES256GCMCipher = "aes-256-gcm"

func ValidateKey(key []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("blob encryption key must be 32 bytes")
	}
	return append([]byte(nil), key...), nil
}

func NormalizeKeyID(keyID string, key []byte) string {
	if keyID != "" {
		return keyID
	}
	sum := sha256.Sum256(key)
	return "sha256:" + hex.EncodeToString(sum[:8])
}

func Encrypt(plaintext, key []byte) ([]byte, string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, "", err
	}
	return aead.Seal(nil, nonce, plaintext, nil), base64.StdEncoding.EncodeToString(nonce), nil
}

func Decrypt(blob model.Blob, raw, key []byte) ([]byte, error) {
	nonce, err := base64.StdEncoding.DecodeString(blob.Nonce)
	if err != nil {
		return nil, fmt.Errorf("decode blob nonce: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	plaintext, err := aead.Open(nil, nonce, raw, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt blob payload: %w", err)
	}
	return plaintext, nil
}
