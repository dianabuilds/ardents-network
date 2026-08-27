package custody

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"

	"golang.org/x/crypto/argon2"
)

const (
	minimumPasswordBytes = 16
	maximumPasswordBytes = 1024
	kdfMemoryKiB         = 262144
	kdfPasses            = 3
	kdfLanes             = 4
	kdfSaltBytes         = 16
)

type envelope struct {
	Profile    string     `json:"profile"`
	Schema     uint64     `json:"schema"`
	KDF        kdfProfile `json:"kdf"`
	AEAD       string     `json:"aead"`
	Ciphertext string     `json:"ciphertext"`
}

type envelopeHeader struct {
	Profile string     `json:"profile"`
	Schema  uint64     `json:"schema"`
	KDF     kdfProfile `json:"kdf"`
	AEAD    string     `json:"aead"`
}

type kdfProfile struct {
	Name      string `json:"name"`
	Version   uint64 `json:"version"`
	MemoryKiB uint64 `json:"memory_kib"`
	Passes    uint64 `json:"passes"`
	Lanes     uint64 `json:"lanes"`
	Salt      string `json:"salt"`
}

type seedRole struct {
	Role    string `json:"role"`
	Private []byte `json:"private"`
}

type seedRecord struct {
	Schema string       `json:"schema"`
	Roles  [10]seedRole `json:"roles"`
}

func seal(plaintext, password []byte) ([]byte, error) {
	salt := make([]byte, kdfSaltBytes)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("fresh release custody salt: %w", err)
	}
	defer zero(salt)
	profile := fixedKDF(salt)
	header := envelopeHeader{Profile: "ardents-release-seed-envelope-v1", Schema: 1, KDF: profile, AEAD: "aes-256-gcm-random-nonce"}
	aad, err := json.Marshal(header)
	if err != nil {
		return nil, err
	}
	key := argon2.IDKey(password, salt, kdfPasses, kdfMemoryKiB, kdfLanes, 32)
	defer zero(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCMWithRandomNonce(block)
	if err != nil {
		return nil, err
	}
	ciphertext := aead.Seal(nil, nil, plaintext, aad)
	defer zero(ciphertext)
	return json.Marshal(envelope{Profile: header.Profile, Schema: header.Schema, KDF: profile, AEAD: header.AEAD, Ciphertext: base64.RawURLEncoding.EncodeToString(ciphertext)})
}

func marshalRecord(record seedRecord) ([]byte, error) {
	if record.Schema != "ardents-release-seed-record-v1" {
		return nil, ErrInvalid
	}
	for index, role := range roleNames {
		if record.Roles[index].Role != role || len(record.Roles[index].Private) != 64 {
			return nil, ErrInvalid
		}
	}
	return json.Marshal(record)
}

func fixedKDF(salt []byte) kdfProfile {
	return kdfProfile{Name: "argon2id", Version: 19, MemoryKiB: kdfMemoryKiB, Passes: kdfPasses, Lanes: kdfLanes, Salt: base64.RawURLEncoding.EncodeToString(salt)}
}

func validPassword(value []byte) bool {
	return len(value) >= minimumPasswordBytes && len(value) <= maximumPasswordBytes
}

func digest(value []byte) [32]byte { return sha256.Sum256(value) }

func zero(value []byte) {
	for index := range value {
		value[index] = 0
	}
}

func zeroRecord(record seedRecord) {
	for index := range record.Roles {
		zero(record.Roles[index].Private)
	}
}
