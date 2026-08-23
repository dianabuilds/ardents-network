package custody

import (
	"bytes"
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
	maximumEnvelopeBytes        = 16 << 20
	maximumPlaintextBytes       = 8 << 20
	maximumRootMaterialBytes    = 65536
	maximumWatermarks           = 32
	maximumWatermarkDomainBytes = 64
	maximumVaultRecords         = 1024
	maximumVaultBytes           = 1 << 30
	minimumPasswordBytes        = 16
	maximumPasswordBytes        = 1024
	kdfMemoryKiB                = 262144
	kdfPasses                   = 3
	kdfLanes                    = 4
	kdfSaltBytes                = 16
)

type envelope struct {
	Profile       string     `json:"profile"`
	SchemaVersion uint64     `json:"schema_version"`
	Purpose       Purpose    `json:"purpose"`
	KDF           kdfProfile `json:"kdf"`
	AEAD          string     `json:"aead"`
	Ciphertext    string     `json:"ciphertext"`
}

type envelopeHeader struct {
	Profile       string     `json:"profile"`
	SchemaVersion uint64     `json:"schema_version"`
	Purpose       Purpose    `json:"purpose"`
	KDF           kdfProfile `json:"kdf"`
	AEAD          string     `json:"aead"`
}

type kdfProfile struct {
	Name      string `json:"name"`
	Version   uint64 `json:"version"`
	MemoryKiB uint64 `json:"memory_kib"`
	Passes    uint64 `json:"passes"`
	Lanes     uint64 `json:"lanes"`
	Salt      string `json:"salt"`
}

func sealEnvelope(purpose Purpose, plaintext, password []byte) ([]byte, error) {
	if err := validatePassword(password); err != nil {
		return nil, err
	}
	if len(plaintext) == 0 || len(plaintext) > maximumPlaintextBytes || (purpose != PurposeVault && purpose != PurposeBundle) {
		return nil, ErrInvalid
	}
	salt := make([]byte, kdfSaltBytes)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("fresh envelope salt: %w", err)
	}
	defer zero(salt)
	profile := fixedKDF(salt)
	header := envelopeHeader{Profile: "ardents-authority-envelope-v1", SchemaVersion: 1, Purpose: purpose, KDF: profile, AEAD: "aes-256-gcm-random-nonce"}
	aad, err := marshalCanonical(header)
	if err != nil {
		return nil, err
	}
	key := deriveKey(password, salt)
	defer zero(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes key: %w", err)
	}
	aead, err := cipher.NewGCMWithRandomNonce(block)
	if err != nil {
		return nil, fmt.Errorf("aead: %w", err)
	}
	ciphertext := aead.Seal(nil, nil, plaintext, aad)
	defer zero(ciphertext)
	encoded, err := marshalCanonical(envelope{Profile: header.Profile, SchemaVersion: header.SchemaVersion, Purpose: purpose, KDF: profile, AEAD: header.AEAD, Ciphertext: base64.RawURLEncoding.EncodeToString(ciphertext)})
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

func openEnvelope(raw, password []byte) (Purpose, []byte, EnvelopeInfo, error) {
	parsed, info, salt, ciphertext, aad, err := parseEnvelope(raw)
	if err != nil {
		return "", nil, EnvelopeInfo{}, err
	}
	defer zero(salt)
	defer zero(ciphertext)
	if err := validatePassword(password); err != nil {
		return "", nil, EnvelopeInfo{}, err
	}
	key := deriveKey(password, salt)
	defer zero(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", nil, EnvelopeInfo{}, ErrUnlockFailed
	}
	aead, err := cipher.NewGCMWithRandomNonce(block)
	if err != nil {
		return "", nil, EnvelopeInfo{}, ErrUnlockFailed
	}
	plaintext, err := aead.Open(nil, nil, ciphertext, aad)
	if err != nil {
		return "", nil, EnvelopeInfo{}, ErrUnlockFailed
	}
	return parsed.Purpose, plaintext, info, nil
}

func inspectEnvelope(raw []byte) (EnvelopeInfo, error) {
	_, info, salt, ciphertext, _, err := parseEnvelope(raw)
	zero(salt)
	zero(ciphertext)
	return info, err
}

func parseEnvelope(raw []byte) (envelope, EnvelopeInfo, []byte, []byte, []byte, error) {
	var parsed envelope
	if err := decodeCanonical(raw, &parsed, maximumEnvelopeBytes); err != nil {
		return envelope{}, EnvelopeInfo{}, nil, nil, nil, fmt.Errorf("envelope canonical: %w", ErrInvalid)
	}
	if parsed.Profile != "ardents-authority-envelope-v1" || parsed.SchemaVersion != 1 || (parsed.Purpose != PurposeVault && parsed.Purpose != PurposeBundle) || parsed.AEAD != "aes-256-gcm-random-nonce" {
		return envelope{}, EnvelopeInfo{}, nil, nil, nil, ErrUnsupported
	}
	if err := validateKDF(parsed.KDF); err != nil {
		return envelope{}, EnvelopeInfo{}, nil, nil, nil, err
	}
	salt, err := decodeRawURL(parsed.KDF.Salt)
	if err != nil || len(salt) != kdfSaltBytes {
		zero(salt)
		return envelope{}, EnvelopeInfo{}, nil, nil, nil, ErrInvalid
	}
	ciphertext, err := decodeRawURL(parsed.Ciphertext)
	if err != nil || len(ciphertext) < 28 || len(ciphertext) > maximumPlaintextBytes+28 {
		zero(salt)
		zero(ciphertext)
		return envelope{}, EnvelopeInfo{}, nil, nil, nil, ErrInvalid
	}
	aad, err := marshalCanonical(envelopeHeader{Profile: parsed.Profile, SchemaVersion: parsed.SchemaVersion, Purpose: parsed.Purpose, KDF: parsed.KDF, AEAD: parsed.AEAD})
	if err != nil {
		zero(salt)
		zero(ciphertext)
		return envelope{}, EnvelopeInfo{}, nil, nil, nil, err
	}
	return parsed, EnvelopeInfo{Purpose: parsed.Purpose, CiphertextSize: uint64(len(ciphertext)), Digest: sha256.Sum256(raw)}, salt, ciphertext, aad, nil
}

func fixedKDF(salt []byte) kdfProfile {
	return kdfProfile{Name: "argon2id", Version: 19, MemoryKiB: kdfMemoryKiB, Passes: kdfPasses, Lanes: kdfLanes, Salt: base64.RawURLEncoding.EncodeToString(salt)}
}

func validateKDF(profile kdfProfile) error {
	if profile.Name != "argon2id" || profile.Version != 19 || profile.MemoryKiB != kdfMemoryKiB || profile.Passes != kdfPasses || profile.Lanes != kdfLanes {
		return ErrUnsupported
	}
	return nil
}

func deriveKey(password, salt []byte) []byte {
	return argon2.IDKey(password, salt, kdfPasses, kdfMemoryKiB, kdfLanes, 32)
}

func validatePassword(password []byte) error {
	if len(password) < minimumPasswordBytes || len(password) > maximumPasswordBytes {
		return ErrInvalid
	}
	return nil
}

func decodeCanonical(raw []byte, target any, maximum int) error {
	if len(raw) == 0 || len(raw) > maximum {
		return ErrInvalid
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return ErrInvalid
	}
	reencoded, err := marshalCanonical(target)
	if err != nil || !bytes.Equal(raw, reencoded) {
		return ErrInvalid
	}
	return nil
}

func marshalCanonical(value any) ([]byte, error) {
	return json.Marshal(value)
}

func decodeRawURL(value string) ([]byte, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || base64.RawURLEncoding.EncodeToString(decoded) != value {
		return nil, ErrInvalid
	}
	return decoded, nil
}

func zero(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
