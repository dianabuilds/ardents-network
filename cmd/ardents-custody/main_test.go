package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/custody"
	"github.com/dianabuilds/ardents-network/internal/service/instance"
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
	beforeEntries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	beforeBody, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := run(t.Context(), []string{"inspect-envelope", "-envelope", path}, &output, nil); err != nil {
		t.Fatalf("inspect envelope: %v", err)
	}
	afterEntries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	afterBody, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(beforeEntries) != 1 || len(afterEntries) != 1 ||
		beforeEntries[0].Name() != afterEntries[0].Name() || !bytes.Equal(beforeBody, afterBody) {
		t.Fatalf("read-only envelope inspection changed its source directory: before=%v after=%v", beforeEntries, afterEntries)
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

func TestServiceAuthorityCommandsCreateAndIssueWithoutSecretDisclosure(t *testing.T) {
	vaultRoot, hostRoot := t.TempDir(), t.TempDir()
	environment, network, authorityRoot := [32]byte{11}, [32]byte{12}, [32]byte{13}
	password := []byte("service custody command password")
	createArguments := []string{"create-service-authority", "-vault-root", vaultRoot,
		"-environment-commitment", hex.EncodeToString(environment[:]),
		"-network-commitment", hex.EncodeToString(network[:]),
		"-root-commitment", hex.EncodeToString(authorityRoot[:])}
	var createdOutput bytes.Buffer
	if err := run(t.Context(), createArguments, &createdOutput,
		&sequenceCommandSecrets{values: [][]byte{password, password}}); err != nil {
		t.Fatalf("create Service Authority command: %v", err)
	}
	var created struct {
		Schema          string `json:"schema"`
		RecordID        string `json:"record_id"`
		IDCommitment    string `json:"id_commitment"`
		AuthorityPublic string `json:"authority_public"`
		Target          string `json:"target"`
	}
	if err := json.Unmarshal(createdOutput.Bytes(), &created); err != nil || created.Schema != "ardents-service-authority-v1" ||
		created.RecordID == "" || created.IDCommitment == "" || created.AuthorityPublic == "" || created.Target == "" ||
		bytes.Contains(createdOutput.Bytes(), password) {
		t.Fatalf("Service Authority receipt = %+v / %v", created, err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	host, err := instance.Initialize(instance.InitializeConfig{Root: hostRoot, NetworkID: network,
		NotBefore: now, NotAfter: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	request, err := host.Request()
	_ = host.Close()
	if err != nil {
		t.Fatal(err)
	}
	requestPath := filepath.Join(t.TempDir(), "service-request.bin")
	if err := os.WriteFile(requestPath, request, 0o600); err != nil {
		t.Fatal(err)
	}
	requestDigest := sha256.Sum256(request)
	issueArguments := []string{"issue-service-credential", "-vault-root", vaultRoot, "-record", created.RecordID,
		"-request", requestPath, "-response", filepath.Join(t.TempDir(), "service-response.bin"),
		"-environment-commitment", hex.EncodeToString(environment[:]),
		"-network-commitment", hex.EncodeToString(network[:]), "-root-commitment", hex.EncodeToString(authorityRoot[:]),
		"-kind", "service", "-id-commitment", created.IDCommitment}
	var issuedOutput bytes.Buffer
	if err := run(t.Context(), issueArguments, &issuedOutput,
		&sequenceCommandSecrets{values: [][]byte{password}, commitments: [][32]byte{requestDigest}}); err != nil {
		t.Fatalf("issue Service Credential command: %v", err)
	}
	var issued struct {
		Schema   string `json:"schema"`
		RecordID string `json:"record_id"`
		Response []byte `json:"response"`
	}
	if err := json.Unmarshal(issuedOutput.Bytes(), &issued); err != nil || issued.Schema != "ardents-service-credential-response-v1" ||
		issued.RecordID == created.RecordID || len(issued.Response) == 0 || bytes.Contains(issuedOutput.Bytes(), password) {
		t.Fatalf("Service Credential receipt = %+v / %v", issued, err)
	}
	if _, err := instance.ParseResponse(issued.Response); err != nil {
		t.Fatalf("parse command response: %v", err)
	}
	persistedResponse, err := os.ReadFile(issueArguments[8])
	if err != nil || !bytes.Equal(persistedResponse, issued.Response) {
		t.Fatalf("persisted public response differs: %v", err)
	}
}

func TestIssueServiceCredentialRejectsSubstitutedRequestBeforePasswordOrMutation(t *testing.T) {
	vaultRoot, hostRoot := t.TempDir(), t.TempDir()
	environment, network, authorityRoot := [32]byte{21}, [32]byte{22}, [32]byte{23}
	password := []byte("service custody substitution password")
	createArguments := []string{"create-service-authority", "-vault-root", vaultRoot,
		"-environment-commitment", hex.EncodeToString(environment[:]),
		"-network-commitment", hex.EncodeToString(network[:]),
		"-root-commitment", hex.EncodeToString(authorityRoot[:])}
	var createdOutput bytes.Buffer
	if err := run(t.Context(), createArguments, &createdOutput,
		&sequenceCommandSecrets{values: [][]byte{password, password}}); err != nil {
		t.Fatal(err)
	}
	var created struct {
		RecordID     string `json:"record_id"`
		IDCommitment string `json:"id_commitment"`
	}
	if err := json.Unmarshal(createdOutput.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	host, err := instance.Initialize(instance.InitializeConfig{Root: hostRoot, NetworkID: network,
		NotBefore: now, NotAfter: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	request, err := host.Request()
	if closeErr := host.Close(); err != nil || closeErr != nil {
		t.Fatalf("read substituted request: %v / %v", err, closeErr)
	}
	requestPath := filepath.Join(t.TempDir(), "substituted-request.bin")
	responsePath := filepath.Join(t.TempDir(), "response.bin")
	if err := os.WriteFile(requestPath, request, 0o600); err != nil {
		t.Fatal(err)
	}
	recordPath := filepath.Join(vaultRoot, "records", "record-"+created.RecordID+".json")
	before, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatal(err)
	}
	trustedDifferentDigest := sha256.Sum256([]byte("independently transferred different host request"))
	input := &sequenceCommandSecrets{values: [][]byte{password}, commitments: [][32]byte{trustedDifferentDigest}}
	arguments := []string{"issue-service-credential", "-vault-root", vaultRoot, "-record", created.RecordID,
		"-request", requestPath, "-response", responsePath,
		"-environment-commitment", hex.EncodeToString(environment[:]),
		"-network-commitment", hex.EncodeToString(network[:]), "-root-commitment", hex.EncodeToString(authorityRoot[:]),
		"-kind", "service", "-id-commitment", created.IDCommitment}
	if err := run(t.Context(), arguments, &bytes.Buffer{}, input); err == nil {
		t.Fatal("custody issued for a request that did not match the independently transferred commitment")
	}
	if input.index != 0 {
		t.Fatal("substituted request reached the password prompt")
	}
	after, err := os.ReadFile(recordPath)
	if err != nil || !bytes.Equal(after, before) {
		t.Fatalf("substituted request changed the Authority record: %v", err)
	}
	if _, err := os.Stat(responsePath); !os.IsNotExist(err) {
		t.Fatalf("substituted request created a response: %v", err)
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
	var receipt map[string]any
	if err := json.Unmarshal(output.Bytes(), &receipt); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"schema", "operation", "record_id", "retained_floor"} {
		if _, found := receipt[key]; !found {
			t.Fatalf("purge receipt lacks canonical %q field: %v", key, receipt)
		}
	}
	for _, key := range []string{"Schema", "Operation", "RecordID"} {
		if _, found := receipt[key]; found {
			t.Fatalf("purge receipt exposes Go field name %q: %v", key, receipt)
		}
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
	commitments   [][32]byte
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

func (input *sequenceCommandSecrets) ReadServiceRequestCommitment(context.Context) ([32]byte, error) {
	if len(input.commitments) == 0 {
		return [32]byte{}, errors.New("unexpected service request commitment")
	}
	commitment := input.commitments[0]
	input.commitments = input.commitments[1:]
	return commitment, nil
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
