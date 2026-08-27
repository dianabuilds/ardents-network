package custody

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/argon2"
)

func TestInitializeCreatesEncryptedFixedRoleRecord(t *testing.T) {
	root := t.TempDir()
	password := []byte("release-custody-password")
	receipt, err := Initialize(context.Background(), InitializeConfig{Root: root}, &fixedSecrets{values: [][]byte{password, password}})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(seedPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, password) || receipt.EnvelopeDigest != digest(raw) {
		t.Fatal("release custody record leaked plaintext or reported the wrong digest")
	}
	record, err := decodeEnvelopeForTest(raw, password)
	if err != nil {
		t.Fatal(err)
	}
	for index, role := range roleNames {
		if record.Roles[index].Role != role || len(record.Roles[index].Private) != ed25519.PrivateKeySize || receipt.Roles[index].Role != role {
			t.Fatalf("role %d = %+v / %+v", index, record.Roles[index], receipt.Roles[index])
		}
		public := ed25519.PrivateKey(record.Roles[index].Private).Public().(ed25519.PublicKey)
		if !bytes.Equal(public, receipt.Roles[index].Public[:]) {
			t.Fatalf("public role %s did not match encrypted key", role)
		}
	}
	if _, err := Initialize(context.Background(), InitializeConfig{Root: root}, unreadSecrets{}); !errors.Is(err, ErrExists) {
		t.Fatalf("second initialize = %v, want ErrExists", err)
	}
}

func decodeEnvelopeForTest(raw, password []byte) (seedRecord, error) {
	var parsed envelope
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return seedRecord{}, err
	}
	salt, err := base64.RawURLEncoding.DecodeString(parsed.KDF.Salt)
	if err != nil || len(salt) != kdfSaltBytes {
		return seedRecord{}, ErrInvalid
	}
	defer zero(salt)
	ciphertext, err := base64.RawURLEncoding.DecodeString(parsed.Ciphertext)
	if err != nil {
		return seedRecord{}, ErrInvalid
	}
	defer zero(ciphertext)
	header, err := json.Marshal(envelopeHeader{Profile: parsed.Profile, Schema: parsed.Schema, KDF: parsed.KDF, AEAD: parsed.AEAD})
	if err != nil {
		return seedRecord{}, err
	}
	key := argon2.IDKey(password, salt, kdfPasses, kdfMemoryKiB, kdfLanes, 32)
	defer zero(key)
	block, err := aes.NewCipher(key)
	if err != nil {
		return seedRecord{}, err
	}
	aead, err := cipher.NewGCMWithRandomNonce(block)
	if err != nil {
		return seedRecord{}, err
	}
	plaintext, err := aead.Open(nil, nil, ciphertext, header)
	if err != nil {
		return seedRecord{}, ErrSecret
	}
	defer zero(plaintext)
	var record seedRecord
	if err := json.Unmarshal(plaintext, &record); err != nil {
		return seedRecord{}, err
	}
	return record, nil
}

func TestInitializeRejectsMismatchedConfirmationWithoutWriting(t *testing.T) {
	root := t.TempDir()
	_, err := Initialize(context.Background(), InitializeConfig{Root: root}, &fixedSecrets{values: [][]byte{[]byte("release-custody-password"), []byte("different-release-password")}})
	if !errors.Is(err, ErrConfirmation) {
		t.Fatalf("Initialize = %v, want ErrConfirmation", err)
	}
	if _, statErr := os.Lstat(filepath.Join(root, "release-seeds.json")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("seed record after rejected secrets = %v", statErr)
	}
}

func TestInitializeRejectsShortPassphraseWithoutWriting(t *testing.T) {
	root := t.TempDir()
	password := []byte("elevenchars")
	_, err := Initialize(context.Background(), InitializeConfig{Root: root}, &fixedSecrets{values: [][]byte{password, password}})
	if !errors.Is(err, ErrPasswordLength) {
		t.Fatalf("Initialize = %v, want ErrPasswordLength", err)
	}
	if _, statErr := os.Lstat(filepath.Join(root, "release-seeds.json")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("seed record after rejected short passphrase = %v", statErr)
	}
}

func TestInitializeRejectsTamperedCiphertext(t *testing.T) {
	root := t.TempDir()
	password := []byte("release-custody-password")
	if _, err := Initialize(context.Background(), InitializeConfig{Root: root}, &fixedSecrets{values: [][]byte{password, password}}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(seedPath(root))
	if err != nil {
		t.Fatal(err)
	}
	var parsed envelope
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatal(err)
	}
	ciphertext, err := base64.RawURLEncoding.DecodeString(parsed.Ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	ciphertext[len(ciphertext)-1] ^= 1
	parsed.Ciphertext = base64.RawURLEncoding.EncodeToString(ciphertext)
	tampered, err := json.Marshal(parsed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeEnvelopeForTest(tampered, password); !errors.Is(err, ErrSecret) {
		t.Fatalf("tampered ciphertext = %v, want ErrSecret", err)
	}
}

func TestInspectReturnsOnlyPublicReceiptForExistingRecord(t *testing.T) {
	root := t.TempDir()
	password := []byte("release-custody-password")
	created, err := Initialize(context.Background(), InitializeConfig{Root: root}, &fixedSecrets{values: [][]byte{password, password}})
	if err != nil {
		t.Fatal(err)
	}
	inspected, err := Inspect(context.Background(), InspectConfig{Root: root}, &fixedSecrets{values: [][]byte{password}})
	if err != nil {
		t.Fatal(err)
	}
	if inspected != created {
		t.Fatalf("Inspect receipt = %+v, want %+v", inspected, created)
	}
}

func TestInspectRejectsWrongSecretWithoutChangingRecord(t *testing.T) {
	root := t.TempDir()
	password := []byte("release-custody-password")
	if _, err := Initialize(context.Background(), InitializeConfig{Root: root}, &fixedSecrets{values: [][]byte{password, password}}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(seedPath(root))
	if err != nil {
		t.Fatal(err)
	}
	_, err = Inspect(context.Background(), InspectConfig{Root: root}, &fixedSecrets{values: [][]byte{[]byte("wrong-release-custody-password")}})
	if !errors.Is(err, ErrSecret) {
		t.Fatalf("Inspect = %v, want ErrSecret", err)
	}
	after, err := os.ReadFile(seedPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("Inspect changed the encrypted record")
	}
}

func TestRecordMarshalRejectsInconsistentPrivateKey(t *testing.T) {
	var record seedRecord
	record.Schema = "ardents-release-seed-record-v1"
	for index, role := range roleNames {
		_, private, err := ed25519.GenerateKey(nil)
		if err != nil {
			t.Fatal(err)
		}
		record.Roles[index] = seedRole{Role: role, Private: private}
	}
	record.Roles[0].Private[len(record.Roles[0].Private)-1] ^= 1
	if _, err := marshalRecord(record); !errors.Is(err, ErrInvalid) {
		t.Fatalf("marshalRecord = %v, want ErrInvalid", err)
	}
	zeroRecord(record)
}

type fixedSecrets struct {
	values [][]byte
	next   int
}

func (input *fixedSecrets) ReadSecret(_ context.Context, _ Prompt) ([]byte, error) {
	if input.next >= len(input.values) {
		return nil, errors.New("unexpected secret request")
	}
	value := append([]byte(nil), input.values[input.next]...)
	input.next++
	return value, nil
}

type unreadSecrets struct{}

func (unreadSecrets) ReadSecret(context.Context, Prompt) ([]byte, error) {
	return nil, errors.New("secret read should not occur")
}
