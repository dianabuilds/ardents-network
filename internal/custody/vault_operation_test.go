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

func TestVaultRejectsConcurrentCrossHandleOperationAsBusy(t *testing.T) {
	root := t.TempDir()
	creator, err := Open(VaultConfig{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	state := testAuthorityState()
	password := []byte("concurrent custody password")
	created, err := creator.Execute(t.Context(), Operation{Kind: OperationCreateVaultRecord, Authority: state},
		&sequenceSecrets{values: [][]byte{password, password}})
	if err != nil || creator.Close() != nil {
		t.Fatalf("prepare concurrent record: %v", err)
	}
	first, err := Open(VaultConfig{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := Open(VaultConfig{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	operation := Operation{Kind: OperationVerifyVaultRecord, RecordID: created.RecordID, Expected: state.Binding}
	entered, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, executeErr := first.Execute(t.Context(), operation,
			blockingSecret{value: password, entered: entered, release: release})
		done <- executeErr
	}()
	<-entered
	if _, err := second.Execute(t.Context(), operation, unreadSecrets{}); !errors.Is(err, ErrBusy) {
		close(release)
		t.Fatalf("concurrent custody operation error = %v, want busy", err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("first custody operation: %v", err)
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
	if _, err := InspectEnvelope(path); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("inspect unsupported = %v, want unsupported", err)
	}
}

func TestVaultExportsDistinctPasswordBundleAndTestRestoresIt(t *testing.T) {
	vault, err := Open(VaultConfig{Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	state := testAuthorityState()
	vaultPassword := []byte("correct horse battery staple")
	created, err := vault.Execute(t.Context(), Operation{Kind: OperationCreateVaultRecord, Authority: state}, &sequenceSecrets{values: [][]byte{vaultPassword, vaultPassword}})
	if err != nil {
		t.Fatalf("create record: %v", err)
	}
	bundlePath := filepath.Join(t.TempDir(), "owner-chosen-bundle.json")
	bundlePassword := []byte("another long bundle password")
	exported, err := vault.Execute(t.Context(), Operation{Kind: OperationExportRecoveryBundle, RecordID: created.RecordID, Expected: state.Binding, Path: bundlePath}, &sequenceSecrets{values: [][]byte{vaultPassword, bundlePassword, bundlePassword}})
	if err != nil {
		t.Fatalf("export bundle: %v", err)
	}
	if !exported.TestRestored || exported.Envelope.Purpose != PurposeBundle || exported.Authority.Binding != state.Binding {
		t.Fatalf("unexpected bundle receipt: %+v", exported)
	}
	raw, err := os.ReadFile(bundlePath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, state.RootMaterial) {
		t.Fatal("recovery bundle retains plaintext root material")
	}
	if _, err := vault.Execute(t.Context(), Operation{Kind: OperationExportRecoveryBundle, RecordID: created.RecordID, Expected: state.Binding, Path: bundlePath}, &sequenceSecrets{values: [][]byte{vaultPassword, []byte("replacement bundle password"), []byte("replacement bundle password")}, confirmations: []bool{false}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unconfirmed bundle replacement = %v, want invalid", err)
	}
	retained, err := os.ReadFile(bundlePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(retained, raw) {
		t.Fatal("unconfirmed replacement changed the previous bundle")
	}
	replaced, err := vault.Execute(t.Context(), Operation{Kind: OperationExportRecoveryBundle, RecordID: created.RecordID, Expected: state.Binding, Path: bundlePath}, &sequenceSecrets{values: [][]byte{vaultPassword, []byte("replacement bundle password"), []byte("replacement bundle password")}, confirmations: []bool{true}})
	if err != nil {
		t.Fatalf("confirmed bundle replacement: %v", err)
	}
	if !replaced.TestRestored || replaced.Envelope.Purpose != PurposeBundle {
		t.Fatalf("unexpected replacement receipt: %+v", replaced)
	}
	replacedBytes, err := os.ReadFile(bundlePath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(replacedBytes, raw) {
		t.Fatal("confirmed replacement retained old bundle bytes")
	}
	if _, err := vault.Execute(t.Context(), Operation{Kind: OperationExportRecoveryBundle, RecordID: created.RecordID, Expected: state.Binding, Path: filepath.Join(t.TempDir(), "same-password.json")}, &sequenceSecrets{values: [][]byte{vaultPassword, vaultPassword, vaultPassword}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("same password bundle export = %v, want invalid", err)
	}
}

func TestCopyEncryptedBundleKeepsDestinationDuringReplacementPreparation(t *testing.T) {
	parent := t.TempDir()
	destination := filepath.Join(parent, "owner-chosen-bundle.json")
	previous := []byte(`{"already":"encrypted-by-its-own-envelope"}`)
	if err := os.WriteFile(destination, previous, 0o600); err != nil {
		t.Fatal(err)
	}
	backup, err := copyEncryptedBundle(destination, parent)
	if err != nil {
		t.Fatalf("copy previous encrypted bundle: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(backup) })
	retained, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("read original destination: %v", err)
	}
	if !bytes.Equal(retained, previous) {
		t.Fatal("preparing rollback copy changed the current bundle")
	}
	copied, err := os.ReadFile(backup)
	if err != nil {
		t.Fatalf("read encrypted backup: %v", err)
	}
	if !bytes.Equal(copied, previous) {
		t.Fatal("rollback copy differs from the encrypted destination")
	}
}

func TestVaultRestoreKeepsBundleAuthorityLockedAndExportOnly(t *testing.T) {
	state := testAuthorityState()
	bundlePassword := []byte("restored bundle password long")
	plaintext, err := encodeAuthorityState(PurposeBundle, state)
	if err != nil {
		t.Fatal(err)
	}
	defer zero(plaintext)
	bundle, err := sealEnvelope(PurposeBundle, plaintext, bundlePassword)
	if err != nil {
		t.Fatal(err)
	}
	defer zero(bundle)
	bundlePath := filepath.Join(t.TempDir(), "source-bundle.json")
	if err := os.WriteFile(bundlePath, bundle, 0o600); err != nil {
		t.Fatal(err)
	}
	vault, err := Open(VaultConfig{Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	vaultPassword := []byte("new vault password after restore")
	restored, err := vault.Execute(t.Context(), Operation{Kind: OperationRestoreRecoveryBundle, Path: bundlePath, Expected: state.Binding}, &sequenceSecrets{values: [][]byte{bundlePassword, vaultPassword, vaultPassword}})
	if err != nil {
		t.Fatalf("restore bundle: %v", err)
	}
	if restored.State != RecordAuthorityLocked || restored.Authority.Binding != state.Binding {
		t.Fatalf("unexpected restore receipt: %+v", restored)
	}
	if _, err := os.Stat(filepath.Join(vault.quarantine, "record-"+restored.RecordID+".json")); err != nil {
		t.Fatalf("quarantine record: %v", err)
	}
	if _, err := vault.Execute(t.Context(), Operation{Kind: OperationVerifyVaultRecord, RecordID: restored.RecordID, Expected: state.Binding}, &sequenceSecrets{values: [][]byte{vaultPassword}}); err == nil {
		t.Fatal("restored record became active through active verification")
	}
	exported, err := vault.Execute(t.Context(), Operation{Kind: OperationExportRecoveryBundle, RecordID: restored.RecordID, Expected: state.Binding, Path: filepath.Join(t.TempDir(), "export-only-bundle.json")}, &sequenceSecrets{values: [][]byte{vaultPassword, []byte("exported locked bundle password"), []byte("exported locked bundle password")}})
	if err != nil {
		t.Fatalf("export locked record: %v", err)
	}
	if exported.State != RecordAuthorityLocked || !exported.TestRestored {
		t.Fatalf("locked record export receipt: %+v", exported)
	}
}

func TestVaultFloorRejectsNonAdvancingRecordAndLocksSupersededRecord(t *testing.T) {
	vault, err := Open(VaultConfig{Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	password := []byte("correct horse battery staple")
	initial := testAuthorityState()
	first, err := vault.Execute(t.Context(), Operation{Kind: OperationCreateVaultRecord, Authority: initial}, &sequenceSecrets{values: [][]byte{password, password}})
	if err != nil {
		t.Fatalf("create initial record: %v", err)
	}
	if _, err := vault.Execute(t.Context(), Operation{Kind: OperationCreateVaultRecord, Authority: initial}, unreadSecrets{}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("equal active record = %v, want invalid", err)
	}
	successor := initial
	successor.Generation++
	successor.Revision++
	successor.Watermarks = []Watermark{{Domain: "credential-generation", Value: initial.Watermarks[0].Value + 1}}
	second, err := vault.Execute(t.Context(), Operation{Kind: OperationCreateVaultRecord, Authority: successor}, &sequenceSecrets{values: [][]byte{password, password}})
	if err != nil {
		t.Fatalf("create successor record: %v", err)
	}
	if _, err := vault.Execute(t.Context(), Operation{Kind: OperationVerifyVaultRecord, RecordID: first.RecordID, Expected: initial.Binding}, &sequenceSecrets{values: [][]byte{password}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("superseded record verify = %v, want invalid", err)
	}
	verified, err := vault.Execute(t.Context(), Operation{Kind: OperationVerifyVaultRecord, RecordID: second.RecordID, Expected: successor.Binding}, &sequenceSecrets{values: [][]byte{password}})
	if err != nil {
		t.Fatalf("verify successor: %v", err)
	}
	if verified.Authority.Generation != successor.Generation || verified.Authority.Revision != successor.Revision {
		t.Fatalf("successor receipt = %+v", verified)
	}
}

func TestVaultReopenRejectsActiveRecordWhenAuthorityFloorIsCorrupt(t *testing.T) {
	root := t.TempDir()
	vault, err := Open(VaultConfig{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	state := testAuthorityState()
	password := []byte("correct horse battery staple")
	created, err := vault.Execute(t.Context(), Operation{Kind: OperationCreateVaultRecord, Authority: state}, &sequenceSecrets{values: [][]byte{password, password}})
	if err != nil {
		t.Fatalf("create record: %v", err)
	}
	if err := vault.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "authority-floors.json"), []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(VaultConfig{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	if _, err := reopened.Execute(t.Context(), Operation{Kind: OperationVerifyVaultRecord, RecordID: created.RecordID, Expected: state.Binding}, &sequenceSecrets{values: [][]byte{password}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("verify with corrupt floor = %v, want invalid", err)
	}
}

func TestVaultRejectsUnverifiablePersistedRecordAndRemovesIt(t *testing.T) {
	vault, err := Open(VaultConfig{Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	recordID := "00112233445566778899aabbccddeeff"
	if err := vault.writeRecord(recordID, []byte("not a canonical envelope")); !errors.Is(err, ErrInvalid) {
		t.Fatalf("write invalid persisted record = %v, want invalid", err)
	}
	if _, err := os.Stat(filepath.Join(vault.records, "record-"+recordID+".json")); !os.IsNotExist(err) {
		t.Fatalf("invalid persisted record remains after verification failure: %v", err)
	}
}

func TestVaultReopensExactAuthorityFloorAfterAtomicPublication(t *testing.T) {
	vault, err := Open(VaultConfig{Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	expected := []authorityFloor{floorFromState(testAuthorityState())}
	if err := vault.writeFloors(expected); err != nil {
		t.Fatalf("write floors: %v", err)
	}
	actual, err := vault.readFloors()
	if err != nil {
		t.Fatalf("reopen floors: %v", err)
	}
	if !equalFloors(actual, expected) {
		t.Fatalf("persisted floors = %#v, want %#v", actual, expected)
	}
}

func TestCustodyRejectsNonRegularFileInputs(t *testing.T) {
	directory := t.TempDir()
	if _, err := readEnvelopeFile(directory); !errors.Is(err, ErrInvalid) {
		t.Fatalf("read directory as envelope = %v, want invalid", err)
	}
	if _, err := readSmallFile(directory); !errors.Is(err, ErrInvalid) {
		t.Fatalf("read directory as authority floor = %v, want invalid", err)
	}
	vault, err := Open(VaultConfig{Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	state := testAuthorityState()
	password := []byte("correct horse battery staple")
	created, err := vault.Execute(t.Context(), Operation{Kind: OperationCreateVaultRecord, Authority: state}, &sequenceSecrets{values: [][]byte{password, password}})
	if err != nil {
		t.Fatalf("create vault record: %v", err)
	}
	if _, err := vault.Execute(t.Context(), Operation{Kind: OperationExportRecoveryBundle, RecordID: created.RecordID, Expected: state.Binding, Path: directory}, &sequenceSecrets{values: [][]byte{password, []byte("bundle password that is distinct"), []byte("bundle password that is distinct")}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("export to directory = %v, want invalid", err)
	}
}

func TestRestorePreviousBundleRestoresEncryptedBytesAfterFailedPublication(t *testing.T) {
	parent := t.TempDir()
	destination := filepath.Join(parent, "owner-chosen-bundle.json")
	backup := filepath.Join(parent, ".ardents-recovery-bundle-previous-test")
	previous := []byte(`{"previous":"encrypted-by-its-own-envelope"}`)
	replacement := []byte(`{"replacement":"encrypted-by-its-own-envelope"}`)
	if err := os.WriteFile(destination, replacement, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(backup, previous, 0o600); err != nil {
		t.Fatal(err)
	}
	cause := errors.New("final bundle test restore failed")
	returned, keepBackup := restorePreviousBundle(destination, backup, parent, cause)
	if !errors.Is(returned, cause) || keepBackup {
		t.Fatalf("restore result = (%v, keep=%t), want original failure and no retained backup", returned, keepBackup)
	}
	actual, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual, previous) {
		t.Fatal("failed publication did not restore the previous encrypted bundle")
	}
	if _, err := os.Stat(backup); !os.IsNotExist(err) {
		t.Fatalf("restored backup remains at temporary path: %v", err)
	}
}

type sequenceSecrets struct {
	values        [][]byte
	confirmations []bool
}

func (input *sequenceSecrets) Confirm(context.Context, ConfirmationPrompt) (bool, error) {
	if len(input.confirmations) == 0 {
		return false, errors.New("unexpected confirmation")
	}
	confirmed := input.confirmations[0]
	input.confirmations = input.confirmations[1:]
	return confirmed, nil
}

type unreadSecrets struct{}

type blockingSecret struct {
	value   []byte
	entered chan struct{}
	release chan struct{}
}

func (input blockingSecret) ReadSecret(context.Context, SecretPrompt) ([]byte, error) {
	close(input.entered)
	<-input.release
	return append([]byte(nil), input.value...), nil
}

func (blockingSecret) Confirm(context.Context, ConfirmationPrompt) (bool, error) {
	return false, errors.New("unexpected confirmation")
}

func (unreadSecrets) ReadSecret(context.Context, SecretPrompt) ([]byte, error) {
	return nil, errors.New("floor rejection must precede secret read")
}

func (unreadSecrets) Confirm(context.Context, ConfirmationPrompt) (bool, error) {
	return false, errors.New("floor rejection must precede confirmation")
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
