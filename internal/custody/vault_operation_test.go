package custody

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestVaultCreatesEncryptedRecordAndVerifiesExpectedBinding(t *testing.T) {
	vault, err := Open(VaultConfig{Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	state := testAuthorityState()
	password := []byte("correct horse battery staple")
	created, err := vault.Execute(t.Context(), Operation{Kind: OperationCreateVaultRecord, Authority: state}, &sequenceSecrets{values: [][]byte{password, password}})
	if err != nil {
		t.Fatalf("create record: %v", err)
	}
	if created.RecordID == "" || created.Envelope.Purpose != PurposeVault || created.Authority.Binding != state.Binding {
		t.Fatalf("unexpected create receipt: %+v", created)
	}
	raw, err := os.ReadFile(filepath.Join(vault.records, "record-"+created.RecordID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, state.RootMaterial) {
		t.Fatal("vault record retains plaintext root material")
	}
	verified, err := vault.Execute(t.Context(), Operation{Kind: OperationVerifyVaultRecord, RecordID: created.RecordID, Expected: state.Binding}, &sequenceSecrets{values: [][]byte{password}})
	if err != nil {
		t.Fatalf("verify record: %v", err)
	}
	if verified.Authority.Generation != state.Generation || verified.Authority.Revision != state.Revision || len(verified.Authority.Watermarks) != 1 || verified.Authority.Watermarks[0] != state.Watermarks[0] {
		t.Fatalf("unexpected verify receipt: %+v", verified)
	}
}

func TestVaultReturnsOneUnlockFailureForWrongSecretAndAuthenticatedCiphertextMutation(t *testing.T) {
	vault, err := Open(VaultConfig{Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	state := testAuthorityState()
	password := []byte("correct horse battery staple")
	created, err := vault.Execute(t.Context(), Operation{Kind: OperationCreateVaultRecord, Authority: state}, &sequenceSecrets{values: [][]byte{password, password}})
	if err != nil {
		t.Fatalf("create record: %v", err)
	}
	request := Operation{Kind: OperationVerifyVaultRecord, RecordID: created.RecordID, Expected: state.Binding}
	if _, err := vault.Execute(t.Context(), request, &sequenceSecrets{values: [][]byte{[]byte("wrong password but long enough")}}); !errors.Is(err, ErrUnlockFailed) {
		t.Fatalf("wrong secret error = %v, want unlock failure", err)
	}
	path := filepath.Join(vault.records, "record-"+created.RecordID+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var parsed envelope
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatal(err)
	}
	ciphertext, err := decodeRawURL(parsed.Ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	ciphertext[0] ^= 1
	parsed.Ciphertext = base64.RawURLEncoding.EncodeToString(ciphertext)
	zero(ciphertext)
	tampered, err := marshalCanonical(parsed)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, tampered, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := vault.Execute(t.Context(), request, &sequenceSecrets{values: [][]byte{password}}); !errors.Is(err, ErrUnlockFailed) {
		t.Fatalf("tampered envelope error = %v, want unlock failure", err)
	}
}

func TestInspectEnvelopeRejectsUnsupportedParametersBeforeSecretInput(t *testing.T) {
	vault, err := Open(VaultConfig{Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	path := filepath.Join(t.TempDir(), "unsupported.json")
	if err := os.WriteFile(path, []byte(`{"profile":"ardents-authority-envelope-v1","schema_version":1,"purpose":"authority-vault","kdf":{"name":"argon2id","version":19,"memory_kib":1,"passes":3,"lanes":4,"salt":"AAAAAAAAAAAAAAAAAAAAAA"},"aead":"aes-256-gcm-random-nonce","ciphertext":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := vault.Execute(context.Background(), Operation{Kind: OperationInspectEnvelope, Path: path}, nil); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("inspect unsupported = %v, want unsupported", err)
	}
}

type sequenceSecrets struct {
	values [][]byte
}

func (input *sequenceSecrets) ReadSecret(context.Context, SecretPrompt) ([]byte, error) {
	if len(input.values) == 0 {
		return nil, errors.New("unexpected secret read")
	}
	value := append([]byte(nil), input.values[0]...)
	input.values = input.values[1:]
	return value, nil
}

func testAuthorityState() AuthorityState {
	var binding AuthorityBinding
	for index := range binding.Environment {
		binding.Environment[index] = byte(index + 1)
		binding.Network[index] = byte(index + 2)
		binding.Root[index] = byte(index + 3)
		binding.IDCommitment[index] = byte(index + 4)
	}
	binding.Kind = AuthorityService
	return AuthorityState{Binding: binding, RootMaterial: []byte("authority-root-material-v1"), Generation: 3, Revision: 7, Watermarks: []Watermark{{Domain: "credential-generation", Value: 3}}}
}
