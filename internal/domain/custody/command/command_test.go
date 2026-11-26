package command

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/custody"
	"github.com/dianabuilds/ardents-network/internal/domain/publication"
)

func TestVerifyRecordUsesCustodyWithoutPrintingSecretMaterial(t *testing.T) {
	root := t.TempDir()
	vault, err := custody.Open(custody.VaultConfig{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	binding := commandAuthorityBinding()
	password := []byte("correct horse battery staple")
	created, err := vault.Execute(t.Context(), custody.Operation{Kind: custody.OperationCreateServiceAuthority, Binding: binding}, commandSecrets{values: [][]byte{password, password}})
	if err != nil {
		t.Fatalf("create vault record: %v", err)
	}
	binding = created.Authority.Binding
	arguments := []string{"verify-record", "-vault-root", root, "-record", created.RecordID,
		"-environment-commitment", hex.EncodeToString(binding.Environment[:]),
		"-network-commitment", hex.EncodeToString(binding.Network[:]),
		"-root-commitment", hex.EncodeToString(binding.Root[:]),
		"-kind", string(binding.Kind), "-id-commitment", hex.EncodeToString(binding.IDCommitment[:])}
	var output bytes.Buffer
	if err := run(t.Context(), arguments, &output, commandSecrets{values: [][]byte{password}}); err != nil {
		t.Fatalf("verify record: %v", err)
	}
	if bytes.Contains(output.Bytes(), password) {
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
	root := filepath.Join(t.TempDir(), "vault")
	input := &sequenceCommandSecrets{values: [][]byte{[]byte("unused fixture password")}}
	var output bytes.Buffer
	err := run(t.Context(), []string{"verify-record", "-vault-root", root, "-record", "not-a-record"}, &output, input)
	if err == nil || err.Error() != "custody operation requires lowercase SHA-256 commitments" {
		t.Fatalf("missing public binding refusal = %v", err)
	}
	if input.index != 0 || output.Len() != 0 {
		t.Fatalf("missing binding produced a secret prompt or receipt: prompts=%d, receipt=%q", input.index, output.Bytes())
	}
	if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing binding touched the vault root: %v", err)
	}
}

func TestServiceAuthorityCommandsCreateAndIssueWithoutSecretDisclosure(t *testing.T) {
	vaultRoot := t.TempDir()
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
	request, requestView := serviceCredentialRequest(t, network)
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
	// Output refusal follows the committed encrypted transition. Recover only
	// its exact response; neither a file conflict nor a receipt failure refunds it.
	responsePath := issueArguments[8]
	if err := os.WriteFile(responsePath, []byte("conflicting public response"), 0o600); err != nil {
		t.Fatal(err)
	}
	var refusedOutput bytes.Buffer
	if err := run(t.Context(), issueArguments, &refusedOutput,
		&sequenceCommandSecrets{values: [][]byte{password}, commitments: [][32]byte{requestDigest}}); err == nil || refusedOutput.Len() != 0 {
		t.Fatal("conflicting response output acknowledged")
	}
	floorPath := filepath.Join(vaultRoot, "authority-floors.json")
	committedFloor, err := os.ReadFile(floorPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(responsePath); err != nil {
		t.Fatal(err)
	}
	if err := run(t.Context(), issueArguments, failedServiceOutput{},
		&sequenceCommandSecrets{values: [][]byte{password}, commitments: [][32]byte{requestDigest}}); err == nil {
		t.Fatal("failed receipt output acknowledged")
	}
	committedResponse, err := os.ReadFile(responsePath)
	if err != nil {
		t.Fatal("receipt refusal lost committed public file", err)
	}
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
	if _, err := publication.VerifyServiceResponse(issued.Response, requestView); err != nil {
		t.Fatalf("verify command response against the original request: %v", err)
	}
	persistedResponse, err := os.ReadFile(issueArguments[8])
	if err != nil || !bytes.Equal(persistedResponse, issued.Response) || !bytes.Equal(committedResponse, issued.Response) {
		t.Fatalf("persisted public response differs: %v", err)
	}
	afterFloor, err := os.ReadFile(floorPath)
	entries, listErr := os.ReadDir(filepath.Join(vaultRoot, "records"))
	if err != nil || listErr != nil || !bytes.Equal(committedFloor, afterFloor) || len(entries) != 2 {
		t.Fatal("output recovery changed committed generation", err, listErr)
	}
}

type failedServiceOutput struct{}

func (failedServiceOutput) Write([]byte) (int, error) {
	return 0, errors.New("injected Service receipt output refusal")
}

func TestIssueServiceCredentialRejectsSubstitutedRequestBeforePasswordOrMutation(t *testing.T) {
	vaultRoot := t.TempDir()
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
	request, _ := serviceCredentialRequest(t, network)
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

func serviceCredentialRequest(t *testing.T, network [32]byte) ([]byte, publication.RequestView) {
	t.Helper()
	public, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	view := publication.RequestView{
		NetworkID:      network,
		InstancePublic: [32]byte(public),
		NotBefore:      now.Unix(),
		NotAfter:       now.Add(time.Hour).Unix(),
	}
	view.Commitment = publication.ServiceRequestCommitment(view)
	return publication.EncodeServiceRequest(view), view
}

func TestRecoveryBundleCommandsKeepAuthorityLockedOnRestore(t *testing.T) {
	sourceRoot := t.TempDir()
	vault, err := custody.Open(custody.VaultConfig{Root: sourceRoot})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	binding := commandAuthorityBinding()
	vaultPassword := []byte("source vault password")
	created, err := vault.Execute(t.Context(), custody.Operation{Kind: custody.OperationCreateServiceAuthority, Binding: binding}, &sequenceCommandSecrets{values: [][]byte{vaultPassword, vaultPassword}})
	if err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(t.TempDir(), "recovery-bundle.json")
	bindingArguments := custodyBindingArguments(created.Authority.Binding)
	exportArguments := append([]string{"export-recovery-bundle", "-vault-root", sourceRoot, "-record", created.RecordID, "-bundle", bundle}, bindingArguments...)
	var exportOutput bytes.Buffer
	bundlePassword := []byte("separate bundle password")
	if err := run(t.Context(), exportArguments, &exportOutput, &sequenceCommandSecrets{values: [][]byte{vaultPassword, bundlePassword, bundlePassword}}); err != nil {
		t.Fatalf("export recovery bundle: %v", err)
	}
	if bytes.Contains(exportOutput.Bytes(), bundlePassword) {
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
	binding := commandAuthorityBinding()
	password := []byte("purge command password")
	created, err := vault.Execute(t.Context(), custody.Operation{Kind: custody.OperationCreateServiceAuthority, Binding: binding}, &sequenceCommandSecrets{values: [][]byte{password, password}})
	if err != nil {
		t.Fatal(err)
	}
	arguments := append([]string{"purge-record", "-vault-root", root, "-record", created.RecordID}, custodyBindingArguments(created.Authority.Binding)...)
	var output bytes.Buffer
	if err := run(t.Context(), arguments, &output, &sequenceCommandSecrets{values: [][]byte{password}, confirmations: []bool{true}}); err != nil {
		t.Fatalf("purge record: %v", err)
	}
	if bytes.Contains(output.Bytes(), password) {
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
	var prompts bytes.Buffer
	input := terminalSecretInput{terminal: reader, prompts: &prompts}
	value, err := input.ReadSecret(t.Context(), custody.SecretPromptVaultUnlock)
	if err == nil || err.Error() != "custody secret input requires an interactive terminal" {
		t.Fatalf("non-terminal refusal = %v", err)
	}
	if len(value) != 0 || prompts.Len() != 0 {
		t.Fatalf("non-terminal produced a value or prompt: value length=%d, prompt length=%d", len(value), prompts.Len())
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

func (input *sequenceCommandSecrets) ReadAdmissionRequestCommitment(context.Context) ([32]byte, error) {
	if len(input.commitments) == 0 {
		return [32]byte{}, errors.New("unexpected admission request commitment")
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

func commandAuthorityBinding() custody.AuthorityBinding {
	var binding custody.AuthorityBinding
	for index := range binding.Environment {
		binding.Environment[index] = byte(index + 1)
		binding.Network[index] = byte(index + 2)
		binding.Root[index] = byte(index + 3)

	}
	binding.Kind = custody.AuthorityService
	return binding
}

func TestPublicCommandFailureEmitsOnlyFiniteLocalObservation(t *testing.T) {
	var output, prompts bytes.Buffer
	err := Run(t.Context(), []string{"verify-record", "--vault-root", "PRIVATE-root"}, &output, &prompts, nil)
	if err == nil {
		t.Fatal("invalid custody command accepted")
	}
	var event map[string]string
	if json.Unmarshal(prompts.Bytes(), &event) != nil || len(event) != 3 || event["operation"] != "custody.verify-record" || event["phase"] != "command" || event["outcome"] != "failed" {
		t.Fatalf("local observation absent: %s", prompts.String())
	}
	if strings.Contains(prompts.String(), "PRIVATE") || output.Len() != 0 {
		t.Fatal("private input or receipt leaked into diagnostics")
	}
}

func TestPublicCommandFailureEmitsOnlySafeLocalCategories(t *testing.T) {
	var output, prompts bytes.Buffer
	err := Run(t.Context(), []string{"verify-record", "--vault-root", "PRIVATE-root"}, &output, &prompts, nil)
	if err == nil {
		t.Fatal("invalid command succeeded")
	}
	var event map[string]string
	if json.Unmarshal(prompts.Bytes(), &event) != nil || len(event) != 3 || event["operation"] != "custody.verify-record" || event["phase"] != "command" || event["outcome"] != "failed" {
		t.Fatalf("missing local command diagnostic: %s", prompts.String())
	}
	if bytes.Contains(prompts.Bytes(), []byte("PRIVATE")) || output.Len() != 0 {
		t.Fatal("private command arguments entered diagnostic")
	}
}

func TestLocalObservationStartsAfterUnterminatedPrompt(t *testing.T) {
	var output, prompts bytes.Buffer
	prompts.WriteString("private-sentinel prompt: ")
	if err := Run(t.Context(), []string{"verify-record", "--vault-root", "unused"}, &output, &prompts, nil); err == nil {
		t.Fatal("invalid command succeeded")
	}
	lines := bytes.Split(bytes.TrimSpace(prompts.Bytes()), []byte("\n"))
	var report map[string]string
	if len(lines) != 2 || json.Unmarshal(lines[1], &report) != nil || report["operation"] != "custody.verify-record" {
		t.Fatalf("observation merged with a private prompt: %q", prompts.Bytes())
	}
	if bytes.Contains(lines[1], []byte("private-sentinel")) {
		t.Fatal("prompt copied into structured record")
	}
}
