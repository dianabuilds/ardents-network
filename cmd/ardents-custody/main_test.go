package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/custody"
)

func TestInspectEnvelopeRejectsMissingInputsBeforeCreatingCustodyState(t *testing.T) {
	var output bytes.Buffer
	if err := run(t.Context(), []string{"inspect-envelope"}, &output, nil); err == nil {
		t.Fatal("inspect accepted missing inputs")
	}
}

func TestInspectEnvelopeRendersOnlyPublicHeaderFacts(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "envelope.json")
	body := []byte(`{"profile":"ardents-authority-envelope-v1","schema_version":1,"purpose":"recovery-bundle","kdf":{"name":"argon2id","version":19,"memory_kib":262144,"passes":3,"lanes":4,"salt":"AAAAAAAAAAAAAAAAAAAAAA"},"aead":"aes-256-gcm-random-nonce","ciphertext":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}`)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := run(t.Context(), []string{"inspect-envelope", "-vault-root", filepath.Join(root, "vault"), "-envelope", path}, &output, nil); err != nil {
		t.Fatalf("inspect envelope: %v", err)
	}
	var result struct {
		Schema    string `json:"schema"`
		Purpose   string `json:"purpose"`
		Operation string `json:"operation"`
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Schema != "ardents-custody-inspection-v1" || result.Purpose != "recovery-bundle" || result.Operation != "inspect-envelope" {
		t.Fatalf("unexpected inspection result: %+v", result)
	}
}

func TestVerifyRecordUsesCustodyWithoutPrintingSecretMaterial(t *testing.T) {
	root := t.TempDir()
	vault, err := custody.Open(custody.VaultConfig{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	state := commandAuthorityState()
	password := []byte("correct horse battery staple")
	created, err := vault.Execute(t.Context(), custody.Operation{Kind: custody.OperationCreateVaultRecord, Authority: state}, commandSecrets{values: [][]byte{password, password}})
	if err != nil {
		t.Fatalf("create vault record: %v", err)
	}
	binding := state.Binding
	arguments := []string{"verify-record", "-vault-root", root, "-record", created.RecordID,
		"-environment-commitment", hex.EncodeToString(binding.Environment[:]),
		"-network-commitment", hex.EncodeToString(binding.Network[:]),
		"-root-commitment", hex.EncodeToString(binding.Root[:]),
		"-kind", string(binding.Kind), "-id-commitment", hex.EncodeToString(binding.IDCommitment[:])}
	var output bytes.Buffer
	if err := run(t.Context(), arguments, &output, commandSecrets{values: [][]byte{password}}); err != nil {
		t.Fatalf("verify record: %v", err)
	}
	if bytes.Contains(output.Bytes(), state.RootMaterial) || bytes.Contains(output.Bytes(), password) {
		t.Fatal("verify command returned secret material")
	}
	var result struct {
		Schema    string `json:"schema"`
		Operation string `json:"operation"`
		RecordID  string `json:"record_id"`
		State     string `json:"state"`
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Schema != "ardents-custody-verification-v1" || result.Operation != "verify-vault-record" || result.RecordID != created.RecordID || result.State != "active" {
		t.Fatalf("unexpected verification result: %+v", result)
	}
}

func TestVerifyRecordRejectsMissingPublicBindingBeforeSecretInput(t *testing.T) {
	var output bytes.Buffer
	if err := run(t.Context(), []string{"verify-record", "-vault-root", t.TempDir(), "-record", "not-a-record"}, &output, commandSecrets{}); err == nil {
		t.Fatal("verify record accepted missing binding")
	}
}

func TestRecoveryBundleCommandsKeepAuthorityLockedOnRestore(t *testing.T) {
	sourceRoot := t.TempDir()
	vault, err := custody.Open(custody.VaultConfig{Root: sourceRoot})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	state := commandAuthorityState()
	vaultPassword := []byte("source vault password")
	created, err := vault.Execute(t.Context(), custody.Operation{Kind: custody.OperationCreateVaultRecord, Authority: state}, &sequenceCommandSecrets{values: [][]byte{vaultPassword, vaultPassword}})
	if err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(t.TempDir(), "recovery-bundle.json")
	bindingArguments := custodyBindingArguments(state.Binding)
	exportArguments := append([]string{"export-recovery-bundle", "-vault-root", sourceRoot, "-record", created.RecordID, "-bundle", bundle}, bindingArguments...)
	var exportOutput bytes.Buffer
	bundlePassword := []byte("separate bundle password")
	if err := run(t.Context(), exportArguments, &exportOutput, &sequenceCommandSecrets{values: [][]byte{vaultPassword, bundlePassword, bundlePassword}}); err != nil {
		t.Fatalf("export recovery bundle: %v", err)
	}
	if bytes.Contains(exportOutput.Bytes(), state.RootMaterial) || bytes.Contains(exportOutput.Bytes(), bundlePassword) {
		t.Fatal("export command returned secret material")
	}
	restoreArguments := append([]string{"restore-recovery-bundle", "-vault-root", t.TempDir(), "-bundle", bundle}, bindingArguments...)
	var restoreOutput bytes.Buffer
	restoredVaultPassword := []byte("restored vault password")
	if err := run(t.Context(), restoreArguments, &restoreOutput, &sequenceCommandSecrets{values: [][]byte{bundlePassword, restoredVaultPassword, restoredVaultPassword}}); err != nil {
		t.Fatalf("restore recovery bundle: %v", err)
	}
	var restored struct {
		Operation string `json:"operation"`
		State     string `json:"state"`
	}
	if err := json.Unmarshal(restoreOutput.Bytes(), &restored); err != nil || restored.Operation != "restore-recovery-bundle" || restored.State != "authority-locked" {
		t.Fatalf("restore receipt = %+v / %v", restored, err)
	}
}

func TestPurgeRecordRequiresConfirmedCustodyTransition(t *testing.T) {
	root := t.TempDir()
	vault, err := custody.Open(custody.VaultConfig{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	state := commandAuthorityState()
	password := []byte("purge command password")
	created, err := vault.Execute(t.Context(), custody.Operation{Kind: custody.OperationCreateVaultRecord, Authority: state}, &sequenceCommandSecrets{values: [][]byte{password, password}})
	if err != nil {
		t.Fatal(err)
	}
	arguments := append([]string{"purge-record", "-vault-root", root, "-record", created.RecordID}, custodyBindingArguments(state.Binding)...)
	var output bytes.Buffer
	if err := run(t.Context(), arguments, &output, &sequenceCommandSecrets{values: [][]byte{password}, confirmations: []bool{true}}); err != nil {
		t.Fatalf("purge record: %v", err)
	}
	if bytes.Contains(output.Bytes(), state.RootMaterial) || bytes.Contains(output.Bytes(), password) {
		t.Fatal("purge command returned secret material")
	}
	if _, err := os.Stat(filepath.Join(root, "records", "record-"+created.RecordID+".json")); !os.IsNotExist(err) {
		t.Fatalf("purged record remains: %v", err)
	}
}

func TestTerminalSecretInputRejectsNonTerminalDescriptor(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = reader.Close()
		_ = writer.Close()
	})
	input := terminalSecretInput{terminal: reader, prompts: &bytes.Buffer{}}
	if _, err := input.ReadSecret(t.Context(), custody.SecretPromptVaultUnlock); err == nil {
		t.Fatal("terminal secret input accepted a non-terminal descriptor")
	}
}

type commandSecrets struct{ values [][]byte }

func (input commandSecrets) ReadSecret(context.Context, custody.SecretPrompt) ([]byte, error) {
	if len(input.values) == 0 {
		return nil, errors.New("unexpected secret request")
	}
	return append([]byte(nil), input.values[0]...), nil
}

func (commandSecrets) Confirm(context.Context, custody.ConfirmationPrompt) (bool, error) {
	return false, errors.New("unexpected confirmation")
}

type sequenceCommandSecrets struct {
	values        [][]byte
	confirmations []bool
	index         int
}

func (input *sequenceCommandSecrets) ReadSecret(context.Context, custody.SecretPrompt) ([]byte, error) {
	if input.index >= len(input.values) {
		return nil, errors.New("unexpected secret request")
	}
	value := append([]byte(nil), input.values[input.index]...)
	input.index++
	return value, nil
}

func (input *sequenceCommandSecrets) Confirm(context.Context, custody.ConfirmationPrompt) (bool, error) {
	if len(input.confirmations) == 0 {
		return false, errors.New("unexpected confirmation")
	}
	confirmed := input.confirmations[0]
	input.confirmations = input.confirmations[1:]
	return confirmed, nil
}

func custodyBindingArguments(binding custody.AuthorityBinding) []string {
	return []string{"-environment-commitment", hex.EncodeToString(binding.Environment[:]),
		"-network-commitment", hex.EncodeToString(binding.Network[:]), "-root-commitment", hex.EncodeToString(binding.Root[:]),
		"-kind", string(binding.Kind), "-id-commitment", hex.EncodeToString(binding.IDCommitment[:])}
}

func commandAuthorityState() custody.AuthorityState {
	var binding custody.AuthorityBinding
	for index := range binding.Environment {
		binding.Environment[index] = byte(index + 1)
		binding.Network[index] = byte(index + 2)
		binding.Root[index] = byte(index + 3)
		binding.IDCommitment[index] = byte(index + 4)
	}
	binding.Kind = custody.AuthorityService
	return custody.AuthorityState{Binding: binding, RootMaterial: []byte("command-authority-root-material"), Generation: 3, Revision: 7, Watermarks: []custody.Watermark{{Domain: "credential-generation", Value: 3}}}
}
