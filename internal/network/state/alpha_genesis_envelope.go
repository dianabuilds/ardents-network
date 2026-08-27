package state

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"

	"golang.org/x/crypto/argon2"
)

const (
	alphaGenesisMinimumPasswordBytes = 16
	alphaGenesisMaximumPasswordBytes = 1024
	alphaGenesisKDFMemoryKiB         = 262144
	alphaGenesisKDFPasses            = 3
	alphaGenesisKDFLanes             = 4
	alphaGenesisKDFSaltBytes         = 16
)

type alphaGenesisEnvelope struct {
	Profile    string                 `json:"profile"`
	Schema     uint64                 `json:"schema"`
	KDF        alphaGenesisKDFProfile `json:"kdf"`
	AEAD       string                 `json:"aead"`
	Ciphertext string                 `json:"ciphertext"`
}

type alphaGenesisEnvelopeHeader struct {
	Profile string                 `json:"profile"`
	Schema  uint64                 `json:"schema"`
	KDF     alphaGenesisKDFProfile `json:"kdf"`
	AEAD    string                 `json:"aead"`
}

type alphaGenesisKDFProfile struct {
	Name      string `json:"name"`
	Version   uint64 `json:"version"`
	MemoryKiB uint64 `json:"memory_kib"`
	Passes    uint64 `json:"passes"`
	Lanes     uint64 `json:"lanes"`
	Salt      string `json:"salt"`
}

type alphaGenesisSeedRecord struct {
	Schema        string   `json:"schema"`
	NetworkID     [32]byte `json:"network_id"`
	GenesisDigest [32]byte `json:"genesis_digest"`
	AuthoritySeed []byte   `json:"authority_seed"`
}

func sealAlphaGenesisRecord(record alphaGenesisSeedRecord, password []byte, policy alphaGenesisPolicy) ([]byte, error) {
	plaintext, err := json.Marshal(record)
	if err != nil {
		return nil, err
	}
	defer zeroAlphaGenesis(plaintext)
	salt := make([]byte, alphaGenesisKDFSaltBytes)
	if _, err := io.ReadFull(policy.random, salt); err != nil {
		return nil, fmt.Errorf("fresh functional alpha State salt: %w", err)
	}
	defer zeroAlphaGenesis(salt)
	kdf := alphaGenesisKDFProfile{Name: "argon2id", Version: 19, MemoryKiB: uint64(policy.kdfMemoryKiB),
		Passes: uint64(policy.kdfPasses), Lanes: uint64(policy.kdfLanes), Salt: base64.RawURLEncoding.EncodeToString(salt)}
	header := alphaGenesisEnvelopeHeader{Profile: "ardents-state-authority-envelope-v1", Schema: 1,
		KDF: kdf, AEAD: "aes-256-gcm-random-nonce"}
	aad, err := json.Marshal(header)
	if err != nil {
		return nil, err
	}
	key := argon2.IDKey(password, salt, policy.kdfPasses, policy.kdfMemoryKiB, policy.kdfLanes, 32)
	defer zeroAlphaGenesis(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCMWithRandomNonce(block)
	if err != nil {
		return nil, err
	}
	ciphertext := aead.Seal(nil, nil, plaintext, aad)
	defer zeroAlphaGenesis(ciphertext)
	return json.Marshal(alphaGenesisEnvelope{Profile: header.Profile, Schema: header.Schema, KDF: kdf,
		AEAD: header.AEAD, Ciphertext: base64.RawURLEncoding.EncodeToString(ciphertext)})
}

func validAlphaGenesisPassword(value []byte) bool {
	return len(value) >= alphaGenesisMinimumPasswordBytes && len(value) <= alphaGenesisMaximumPasswordBytes
}

func zeroAlphaGenesis(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
