//go:build linux

package endpoint

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

type textReaderContextEvidence struct {
	ID               string                     `json:"id"`
	InputSHA256      string                     `json:"inputSHA256"`
	ExchangeSHA256   string                     `json:"exchangeSHA256"`
	RequestSHA256    string                     `json:"requestSHA256"`
	PermissionSHA256 string                     `json:"permissionSHA256"`
	HolderSHA256     string                     `json:"holderSHA256"`
	PermissionIDHash string                     `json:"permissionIDHash"`
	Lookups          []textReaderLookupEvidence `json:"lookups"`
}

type textReaderLookupEvidence struct {
	DescriptorSHA256        string    `json:"descriptorSHA256"`
	ResponseRevision        uint64    `json:"responseRevision"`
	HolderSHA256            string    `json:"holderSHA256"`
	PermissionIDHash        string    `json:"permissionIDHash"`
	IssuanceBatches         uint8     `json:"issuanceBatches"`
	Reserved                [3]uint32 `json:"reserved"`
	DescriptorFloorRevision uint64    `json:"descriptorFloorRevision"`
}

type textReaderContextIsolationReceipt struct {
	Carrier                      string                      `json:"carrier"`
	Contexts                     []textReaderContextEvidence `json:"contexts"`
	ForeignPermissionRefusalHash string                      `json:"foreignPermissionRefusalHash"`
	RoleDurableReceiptSHA256     string                      `json:"roleDurableReceiptSHA256"`
	RoleObservations             []textReaderRoleObservation `json:"roleObservations"`
}

type textReaderRoleObservation struct {
	Role        string `json:"role"`
	InputSHA256 string `json:"inputSHA256"`
}

type textReaderContextProcess struct {
	t                           *testing.T
	source                      *textSourceStateFixture
	input                       textReaderProcessInput
	id, inputPath, exchangePath string
	command                     *exec.Cmd
	stdin                       io.WriteCloser
	reader                      *bufio.Reader
	decoder                     *json.Decoder
	encoder                     *json.Encoder
	stderr                      bytes.Buffer
	request, permission         []byte
	lookups                     []textReaderLookupEvidence
	joined                      bool
}

func startTextReaderContextProcess(t *testing.T, source *textSourceStateFixture, target [32]byte, expected []byte, output, id string) *textReaderContextProcess {
	t.Helper()
	source.mu.Lock()
	input := textReaderProcessInput{Snapshot: source.snapshot, View: source.view, Target: target, Expected: append([]byte(nil), expected...)}
	source.mu.Unlock()
	prefix := "reader-context-" + id + "-"
	inputPath := filepath.Join(output, prefix+"input.json")
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inputPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	command := exec.CommandContext(ctx, binary, "-test.run=^TestTextPublicationIsolatedRoleObservations$", "-test.timeout=85s")
	command.Env = []string{"PATH=" + os.Getenv("PATH"), "ARDENTS_TEXT_READER_CHILD=" + inputPath, "ARDENTS_TEXT_READER_PROTOCOL_FD=3"}
	stdin, err := command.StdinPipe()
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	protocolRead, protocolWrite, err := os.Pipe()
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	command.ExtraFiles = []*os.File{protocolWrite}
	process := &textReaderContextProcess{t: t, source: source, input: input, id: id, inputPath: inputPath, exchangePath: filepath.Join(output, prefix+"exchange.json"), command: command, stdin: stdin}
	t.Cleanup(process.stopAfterFailure)
	t.Cleanup(func() { _ = protocolRead.Close() })
	command.Stderr = &process.stderr
	if err := command.Start(); err != nil {
		_ = protocolRead.Close()
		_ = protocolWrite.Close()
		cancel()
		t.Fatal(err)
	}
	if err := protocolWrite.Close(); err != nil {
		process.stopAfterFailure()
		t.Fatal(err)
	}
	t.Cleanup(cancel)
	process.reader = bufio.NewReader(protocolRead)
	process.decoder, process.encoder = json.NewDecoder(process.reader), json.NewEncoder(stdin)
	if err := process.decoder.Decode(&process.request); err != nil {
		process.stopAfterFailure()
		t.Fatalf("reader %s permission request: %v", id, err)
	}
	if _, err := credential.DecodePermissionRequest(process.request); err != nil {
		process.stopAfterFailure()
		t.Fatalf("reader %s permission request framing: %v", id, err)
	}
	return process
}

func (process *textReaderContextProcess) stopAfterFailure() {
	if process == nil || process.joined || process.command == nil || process.command.Process == nil {
		return
	}
	_ = process.command.Process.Kill()
	_ = process.command.Wait()
	process.joined = true
}

func (process *textReaderContextProcess) importPermission(permission []byte, accepted bool) {
	process.t.Helper()
	if err := process.encoder.Encode(textReaderControl{Command: "permission", Permission: permission}); err != nil {
		process.t.Fatal(err)
	}
	var event textReaderObservationEvent
	if err := process.decoder.Decode(&event); err != nil {
		process.t.Fatal(err)
	}
	want := "permission-refused"
	if accepted {
		want = "permission-imported"
		process.permission = append([]byte(nil), permission...)
	}
	if event.Phase != want {
		process.t.Fatalf("reader %s permission event = %#v, want %q", process.id, event, want)
	}
}

func (process *textReaderContextProcess) issueAndImport() {
	process.t.Helper()
	permission := process.source.issueRawPermission(process.t, process.request, sha256.Sum256(process.request))
	process.importPermission(permission, true)
}

func (process *textReaderContextProcess) lookup(repeat bool) {
	process.t.Helper()
	command := "lookup"
	if repeat {
		command = "repeat-lookup"
	}
	if err := process.encoder.Encode(textReaderControl{Command: command}); err != nil {
		process.t.Fatal(err)
	}
	var event textReaderObservationEvent
	if !repeat {
		var raw json.RawMessage
		if err := process.decoder.Decode(&raw); err != nil {
			process.t.Fatalf("reader %s active lookup event: %v", process.id, err)
		}
		if err := json.Unmarshal(raw, &event); err != nil || event.Phase != "protected-lookup-active" {
			process.t.Fatalf("reader %s active lookup event: %#v / %q / %v", process.id, event, raw, err)
		}
		if err := process.encoder.Encode(textReaderControl{Command: "release-lookup"}); err != nil {
			process.t.Fatal(err)
		}
	}
	if err := process.decoder.Decode(&event); err != nil || event.Phase != "response-received-before-close" {
		process.t.Fatalf("reader %s response event: %#v / %v", process.id, event, err)
	}
	response := sha256.Sum256(process.input.Expected)
	if event.ResponseDescriptorSHA256 != hex.EncodeToString(response[:]) {
		process.t.Fatalf("reader %s response belongs to another Descriptor", process.id)
	}
	var proof []byte
	if err := process.decoder.Decode(&proof); err != nil || !bytes.Equal(proof, process.input.Expected) {
		process.t.Fatalf("reader %s lookup proof: %v", process.id, err)
	}
	request, err := credential.DecodePermissionRequest(process.request)
	if err != nil {
		process.t.Fatalf("decode reader permission request after lookup: %v", err)
	}
	holderDigest := sha256.Sum256(request.Permission.HolderKey[:])
	permissionIDDigest := sha256.Sum256(request.Permission.PermissionID[:])
	if event.HolderSHA256 != hex.EncodeToString(holderDigest[:]) || event.PermissionIDSHA256 != hex.EncodeToString(permissionIDDigest[:]) || event.IssuanceBatches == 0 {
		process.t.Fatalf("reader %s lookup did not preserve its actual permission allocation", process.id)
	}
	for index, reserved := range event.Reserved {
		if reserved > request.Permission.Maxima[index] {
			process.t.Fatalf("reader %s lookup exceeded its permission quota", process.id)
		}
	}
	if event.ResponseRevision == 0 || event.DescriptorFloorRevision < event.ResponseRevision {
		process.t.Fatalf("reader %s lookup did not retain its descriptor floor", process.id)
	}
	process.lookups = append(process.lookups, textReaderLookupEvidence{DescriptorSHA256: event.ResponseDescriptorSHA256, ResponseRevision: event.ResponseRevision, HolderSHA256: event.HolderSHA256, PermissionIDHash: event.PermissionIDSHA256, IssuanceBatches: event.IssuanceBatches, Reserved: event.Reserved, DescriptorFloorRevision: event.DescriptorFloorRevision})
}

func (process *textReaderContextProcess) stop() {
	process.t.Helper()
	if process.joined {
		return
	}
	defer func() {
		if !process.joined {
			process.stopAfterFailure()
		}
	}()
	if err := process.encoder.Encode(textReaderControl{Command: "stop"}); err != nil {
		process.t.Fatal(err)
	}
	if err := process.stdin.Close(); err != nil {
		process.t.Fatal(err)
	}
	_, readErr := io.Copy(io.Discard, io.MultiReader(process.decoder.Buffered(), process.reader))
	err := process.command.Wait()
	process.joined = true
	if err != nil {
		process.t.Fatal(fmt.Errorf("reader %s process: %w; %s", process.id, err, process.stderr.String()))
	}
	if readErr != nil {
		process.t.Fatal(readErr)
	}
}

func (process *textReaderContextProcess) writeEvidence() textReaderContextEvidence {
	process.t.Helper()
	if len(process.request) == 0 || len(process.permission) == 0 || len(process.lookups) == 0 {
		process.t.Fatal("reader context has incomplete evidence")
	}
	exchange, err := json.Marshal(struct {
		Request, Permission, Descriptor []byte
		Lookups                         []textReaderLookupEvidence
	}{Request: process.request, Permission: process.permission, Descriptor: process.input.Expected, Lookups: process.lookups})
	if err != nil {
		process.t.Fatal(err)
	}
	if err := os.WriteFile(process.exchangePath, exchange, 0o600); err != nil {
		process.t.Fatal(err)
	}
	input, err := os.ReadFile(process.inputPath)
	if err != nil {
		process.t.Fatal(err)
	}
	persisted, err := os.ReadFile(process.exchangePath)
	if err != nil {
		process.t.Fatal(err)
	}
	request, err := credential.DecodePermissionRequest(process.request)
	if err != nil {
		process.t.Fatal(err)
	}
	inputHash, exchangeHash := sha256.Sum256(input), sha256.Sum256(persisted)
	requestHash, permissionHash := sha256.Sum256(process.request), sha256.Sum256(process.permission)
	holderHash, permissionIDHash := sha256.Sum256(request.Permission.HolderKey[:]), sha256.Sum256(request.Permission.PermissionID[:])
	return textReaderContextEvidence{ID: process.id, InputSHA256: hex.EncodeToString(inputHash[:]), ExchangeSHA256: hex.EncodeToString(exchangeHash[:]),
		RequestSHA256: hex.EncodeToString(requestHash[:]), PermissionSHA256: hex.EncodeToString(permissionHash[:]), HolderSHA256: hex.EncodeToString(holderHash[:]), PermissionIDHash: hex.EncodeToString(permissionIDHash[:]), Lookups: append([]textReaderLookupEvidence(nil), process.lookups...)}
}

func observeTextIndependentReaderContexts(t *testing.T, source *textSourceStateFixture, target [32]byte, expected []byte, output string) ([]textReaderContextEvidence, [32]byte) {
	t.Helper()
	first := startTextReaderContextProcess(t, source, target, expected, output, "first")
	defer first.stopAfterFailure()
	first.issueAndImport()
	first.lookup(false)
	second := startTextReaderContextProcess(t, source, target, expected, output, "second")
	defer second.stopAfterFailure()
	second.importPermission(first.permission, false)
	second.issueAndImport()
	second.lookup(false)
	first.lookup(true)
	first.stop()
	second.stop()
	firstRequest, err := credential.DecodePermissionRequest(first.request)
	if err != nil {
		t.Fatal(err)
	}
	secondRequest, err := credential.DecodePermissionRequest(second.request)
	if err != nil || firstRequest.Role != credential.AllocationUser || secondRequest.Role != credential.AllocationUser || firstRequest.Permission.NotBefore != secondRequest.Permission.NotBefore || firstRequest.Permission.NotAfter != secondRequest.Permission.NotAfter || firstRequest.Permission.Maxima != [3]uint32{64, 64, 0} || secondRequest.Permission.Maxima != [3]uint32{64, 64, 0} {
		t.Fatalf("reader contexts did not retain the selected permission/hour scope: %#v / %#v / %v", firstRequest.Permission, secondRequest.Permission, err)
	}
	firstEvidence, secondEvidence := first.writeEvidence(), second.writeEvidence()
	if firstEvidence.HolderSHA256 == secondEvidence.HolderSHA256 || firstEvidence.PermissionIDHash == secondEvidence.PermissionIDHash || firstEvidence.RequestSHA256 == secondEvidence.RequestSHA256 || len(firstEvidence.Lookups) != 2 || len(secondEvidence.Lookups) != 1 {
		t.Fatal("reader contexts did not preserve independent holder/request boundaries and first-context repetition")
	}
	for _, context := range []textReaderContextEvidence{firstEvidence, secondEvidence} {
		for _, lookup := range context.Lookups {
			if lookup.HolderSHA256 != context.HolderSHA256 || lookup.PermissionIDHash != context.PermissionIDHash || lookup.IssuanceBatches == 0 {
				t.Fatalf("reader context %s lookup lost its allocation boundary", context.ID)
			}
		}
	}
	if firstEvidence.Lookups[1].IssuanceBatches < firstEvidence.Lookups[0].IssuanceBatches {
		t.Fatalf("reader %s reset its issuance batch state across the repeat lookup", firstEvidence.ID)
	}
	for index, reserved := range firstEvidence.Lookups[0].Reserved {
		if firstEvidence.Lookups[1].Reserved[index] < reserved {
			t.Fatalf("reader %s reset its quota floor across the repeat lookup: %#v / %#v", firstEvidence.ID, firstEvidence.Lookups[0], firstEvidence.Lookups[1])
		}
	}
	if firstEvidence.Lookups[1].DescriptorFloorRevision < firstEvidence.Lookups[0].DescriptorFloorRevision {
		t.Fatalf("reader %s reset its quota or spend floor across the repeat lookup: %#v / %#v", firstEvidence.ID, firstEvidence.Lookups[0], firstEvidence.Lookups[1])
	}
	return []textReaderContextEvidence{firstEvidence, secondEvidence}, sha256.Sum256(first.permission)
}

func writeAndVerifyTextReaderContextIsolation(t *testing.T, output, carrier string, contexts []textReaderContextEvidence, foreignHash [32]byte) {
	t.Helper()
	if len(contexts) != 2 {
		t.Fatal("reader context evidence count is invalid")
	}
	roleReceipt, err := os.ReadFile(filepath.Join(output, "durable-receipt.json"))
	if err != nil {
		t.Fatal(err)
	}
	roleHash := sha256.Sum256(roleReceipt)
	var durable textRoleDurableReceipt
	if err := json.Unmarshal(roleReceipt, &durable); err != nil || durable.Carrier != carrier {
		t.Fatalf("reader context durable receipt = %#v / %v", durable, err)
	}
	roles := make([]textReaderRoleObservation, 0, 3)
	seen := map[string]bool{}
	for _, role := range durable.Roles {
		if !seen[role.Role] && (role.Role == "issuer" || role.Role == "forwarding" || role.Role == "resolution") {
			seen[role.Role] = true
			roles = append(roles, textReaderRoleObservation{Role: role.Role, InputSHA256: role.InputSHA256})
		}
	}
	if len(roles) != 3 {
		t.Fatal("durable receipt lacks issuer, forwarding, or Gateway resolution observation")
	}
	receiptBytes, err := json.Marshal(textReaderContextIsolationReceipt{Carrier: carrier, Contexts: contexts, ForeignPermissionRefusalHash: hex.EncodeToString(foreignHash[:]), RoleDurableReceiptSHA256: hex.EncodeToString(roleHash[:]), RoleObservations: roles})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(output, "reader-context-isolation.json")
	if err := os.WriteFile(path, receiptBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	persisted, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var receipt textReaderContextIsolationReceipt
	if err := json.Unmarshal(persisted, &receipt); err != nil || receipt.Carrier != carrier || !reflect.DeepEqual(receipt.Contexts, contexts) || receipt.ForeignPermissionRefusalHash != hex.EncodeToString(foreignHash[:]) || receipt.RoleDurableReceiptSHA256 != hex.EncodeToString(roleHash[:]) || !reflect.DeepEqual(receipt.RoleObservations, roles) {
		t.Fatalf("reader context receipt = %#v / %v", receipt, err)
	}
}
