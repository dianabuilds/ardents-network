//go:build linux || windows

package service_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/publication"
)

// The Service Instance ceremony runs through the successor publication owner
// (ardents-next publication instance-initialize/instance-accept); the former
// ardents service-instance routes are retired (#562). These process tests keep
// the at-most-once acceptance, exact-retry and durable-conflict evidence on the
// accepting successor surface (#649).

func TestHeadlessServiceInstanceAcquisitionIsAtMostOnceAcrossProcesses(t *testing.T) {
	next := buildProductCommand(t, "ardents-next")
	custodyBinary := buildProductCommand(t, "ardents-custody")
	directory := t.TempDir()
	vaultRoot := filepath.Join(directory, "authority-vault")
	rootPath := filepath.Join(directory, "instance-root")
	network := [32]byte{41}
	environment, authorityRoot := [32]byte{42}, [32]byte{43}
	password := "artifact native custody password"
	createdTerminal := runInteractiveProductCommand(t, directory, custodyBinary,
		[]interactiveProductInput{{prompt: "vault-create password:", value: password, secret: true},
			{prompt: "vault-create-confirm password:", value: password, secret: true}},
		"create-service-authority", "-vault-root", vaultRoot,
		"-environment-commitment", hex.EncodeToString(environment[:]),
		"-network-commitment", hex.EncodeToString(network[:]),
		"-root-commitment", hex.EncodeToString(authorityRoot[:]))
	if bytes.Contains(createdTerminal, []byte(password)) {
		t.Fatal("custody artifact echoed its terminal password")
	}
	var created struct {
		Schema       string `json:"schema"`
		RecordID     string `json:"record_id"`
		IDCommitment string `json:"id_commitment"`
	}
	if err := json.Unmarshal(interactiveProductJSON(t, createdTerminal), &created); err != nil ||
		created.Schema != "ardents-service-authority-v1" || created.RecordID == "" || created.IDCommitment == "" {
		t.Fatalf("Service Authority artifact receipt = %+v / %v", created, err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	requestPath := filepath.Join(directory, "instance-request.bin")
	request, requestSHA256 := initializeSuccessorInstance(t, directory, next, rootPath, network, now, requestPath)
	requestView, err := publication.ParseServiceRequest(request)
	if err != nil || requestView.NetworkID != network {
		t.Fatalf("public request = %+v / %v", requestView, err)
	}

	responsePath := filepath.Join(directory, "instance-response.bin")
	issueArguments := []string{"issue-service-credential", "-vault-root", vaultRoot, "-record", created.RecordID,
		"-request", requestPath, "-response", responsePath,
		"-environment-commitment", hex.EncodeToString(environment[:]),
		"-network-commitment", hex.EncodeToString(network[:]),
		"-root-commitment", hex.EncodeToString(authorityRoot[:]),
		"-kind", "service", "-id-commitment", created.IDCommitment}
	issuedTerminal := runInteractiveProductCommand(t, directory, custodyBinary,
		[]interactiveProductInput{{prompt: "service-request SHA-256 from the requesting Endpoint:", value: requestSHA256},
			{prompt: "vault-unlock password:", value: password, secret: true}}, issueArguments...)
	if bytes.Contains(issuedTerminal, []byte(password)) {
		t.Fatal("custody artifact echoed its unlock password")
	}
	var issued struct {
		Schema     string `json:"schema"`
		RecordID   string `json:"record_id"`
		Generation uint64 `json:"generation"`
		Response   []byte `json:"response"`
	}
	if err := json.Unmarshal(interactiveProductJSON(t, issuedTerminal), &issued); err != nil ||
		issued.Schema != "ardents-service-credential-response-v1" || issued.RecordID == created.RecordID ||
		issued.Generation != 1 || len(issued.Response) == 0 {
		t.Fatalf("Service Credential artifact receipt = %+v / %v", issued, err)
	}
	response, err := os.ReadFile(responsePath)
	if err != nil || !bytes.Equal(response, issued.Response) {
		t.Fatalf("custody response file differs from receipt: %v", err)
	}
	repeatedTerminal := runInteractiveProductCommand(t, directory, custodyBinary,
		[]interactiveProductInput{{prompt: "service-request SHA-256 from the requesting Endpoint:", value: requestSHA256},
			{prompt: "vault-unlock password:", value: password, secret: true}}, issueArguments...)
	var repeated struct {
		RecordID string `json:"record_id"`
		Response []byte `json:"response"`
	}
	if err := json.Unmarshal(interactiveProductJSON(t, repeatedTerminal), &repeated); err != nil ||
		repeated.RecordID != issued.RecordID || !bytes.Equal(repeated.Response, response) {
		t.Fatalf("custody exact retry changed its public result: %+v / %v", repeated, err)
	}
	assertServiceInstanceAccepted(t, directory, next, rootPath, responsePath)
	assertServiceInstanceAccepted(t, directory, next, rootPath, responsePath)

	conflicting := append([]byte(nil), response...)
	conflicting[len(conflicting)-1] ^= 0xff
	conflictingPath := filepath.Join(directory, "conflicting-response.bin")
	if err := os.WriteFile(conflictingPath, conflicting, 0o600); err != nil {
		t.Fatal(err)
	}
	assertServiceInstanceUnavailable(t, directory, next, rootPath, conflictingPath)
	assertServiceInstanceUnavailable(t, directory, next, rootPath, responsePath)
}

func TestHeadlessCredentialResponseConflictRecoversExactlyOnce(t *testing.T) {
	next := buildProductCommand(t, "ardents-next")
	custodyBinary := buildProductCommand(t, "ardents-custody")
	directory := t.TempDir()
	vaultRoot := filepath.Join(directory, "authority-vault")
	network := [32]byte{61}
	environment, authorityRoot := [32]byte{62}, [32]byte{63}
	password := "atomic credential response custody password"
	createdTerminal := runInteractiveProductCommand(t, directory, custodyBinary,
		[]interactiveProductInput{{prompt: "vault-create password:", value: password, secret: true},
			{prompt: "vault-create-confirm password:", value: password, secret: true}},
		"create-service-authority", "-vault-root", vaultRoot,
		"-environment-commitment", hex.EncodeToString(environment[:]),
		"-network-commitment", hex.EncodeToString(network[:]),
		"-root-commitment", hex.EncodeToString(authorityRoot[:]))
	assertTerminalPasswordHidden(t, createdTerminal, password)
	var created struct {
		Schema       string `json:"schema"`
		RecordID     string `json:"record_id"`
		IDCommitment string `json:"id_commitment"`
	}
	if err := json.Unmarshal(interactiveProductJSON(t, createdTerminal), &created); err != nil ||
		created.Schema != "ardents-service-authority-v1" || created.RecordID == "" || created.IDCommitment == "" {
		t.Fatalf("Service Authority artifact receipt = %+v / %v", created, err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	type initializedInstance struct {
		rootPath, requestPath, requestSHA256 string
	}
	initialize := func(name string) initializedInstance {
		t.Helper()
		rootPath := filepath.Join(directory, name+"-root")
		requestPath := filepath.Join(directory, name+"-request.bin")
		_, requestSHA256 := initializeSuccessorInstance(t, directory, next, rootPath, network, now, requestPath)
		return initializedInstance{rootPath: rootPath, requestPath: requestPath, requestSHA256: requestSHA256}
	}
	issueArguments := func(requestPath, responsePath string) []string {
		return []string{"issue-service-credential", "-vault-root", vaultRoot, "-record", created.RecordID,
			"-request", requestPath, "-response", responsePath,
			"-environment-commitment", hex.EncodeToString(environment[:]),
			"-network-commitment", hex.EncodeToString(network[:]),
			"-root-commitment", hex.EncodeToString(authorityRoot[:]),
			"-kind", "service", "-id-commitment", created.IDCommitment}
	}

	blocked := initialize("blocked-instance")
	conflictingResponsePath := filepath.Join(directory, "conflicting-response.bin")
	conflictingResponse := []byte("unrelated public response")
	if err := os.WriteFile(conflictingResponsePath, conflictingResponse, 0o600); err != nil {
		t.Fatal(err)
	}
	failedTerminal, err := runInteractiveProductCommandResult(t, directory, custodyBinary,
		[]interactiveProductInput{{prompt: "service-request SHA-256 from the requesting Endpoint:", value: blocked.requestSHA256},
			{prompt: "vault-unlock password:", value: password, secret: true}}, issueArguments(blocked.requestPath, conflictingResponsePath)...)
	assertTerminalPasswordHidden(t, failedTerminal, password)
	if err == nil || !bytes.Contains(failedTerminal, []byte("service Credential response destination conflicts")) {
		t.Fatalf("conflicting Credential response publication = %v / %s", err, failedTerminal)
	}
	if stored, readErr := os.ReadFile(conflictingResponsePath); readErr != nil || !bytes.Equal(stored, conflictingResponse) {
		t.Fatalf("conflicting public response changed after failed publication: %q / %v", stored, readErr)
	}

	recoveredResponsePath := filepath.Join(directory, "recovered-response.bin")
	recoveredTerminal := runInteractiveProductCommand(t, directory, custodyBinary,
		[]interactiveProductInput{{prompt: "service-request SHA-256 from the requesting Endpoint:", value: blocked.requestSHA256},
			{prompt: "vault-unlock password:", value: password, secret: true}}, issueArguments(blocked.requestPath, recoveredResponsePath)...)
	assertTerminalPasswordHidden(t, recoveredTerminal, password)
	var recovered struct {
		Schema     string `json:"schema"`
		RecordID   string `json:"record_id"`
		Generation uint64 `json:"generation"`
		Response   []byte `json:"response"`
	}
	if err := json.Unmarshal(interactiveProductJSON(t, recoveredTerminal), &recovered); err != nil ||
		recovered.Schema != "ardents-service-credential-response-v1" || recovered.RecordID == created.RecordID ||
		recovered.Generation != 1 || len(recovered.Response) == 0 {
		t.Fatalf("recovered Credential response receipt = %+v / %v", recovered, err)
	}
	if persisted, readErr := os.ReadFile(recoveredResponsePath); readErr != nil || !bytes.Equal(persisted, recovered.Response) {
		t.Fatalf("recovered public response differs from receipt: %v", readErr)
	}
	repeatedTerminal := runInteractiveProductCommand(t, directory, custodyBinary,
		[]interactiveProductInput{{prompt: "service-request SHA-256 from the requesting Endpoint:", value: blocked.requestSHA256},
			{prompt: "vault-unlock password:", value: password, secret: true}}, issueArguments(blocked.requestPath, recoveredResponsePath)...)
	assertTerminalPasswordHidden(t, repeatedTerminal, password)
	var repeated struct {
		RecordID string `json:"record_id"`
		Response []byte `json:"response"`
	}
	if err := json.Unmarshal(interactiveProductJSON(t, repeatedTerminal), &repeated); err != nil ||
		repeated.RecordID != recovered.RecordID || !bytes.Equal(repeated.Response, recovered.Response) {
		t.Fatalf("exact Credential response retry changed its result: %+v / %v", repeated, err)
	}
	assertServiceInstanceAccepted(t, directory, next, blocked.rootPath, recoveredResponsePath)
	assertServiceInstanceAccepted(t, directory, next, blocked.rootPath, recoveredResponsePath)

	conflicting := initialize("conflicting-instance")
	conflictingPublicationPath := filepath.Join(directory, "second-response.bin")
	secondTerminal, secondErr := runInteractiveProductCommandResult(t, directory, custodyBinary,
		[]interactiveProductInput{{prompt: "service-request SHA-256 from the requesting Endpoint:", value: conflicting.requestSHA256},
			{prompt: "vault-unlock password:", value: password, secret: true}}, issueArguments(conflicting.requestPath, conflictingPublicationPath)...)
	assertTerminalPasswordHidden(t, secondTerminal, password)
	if secondErr == nil {
		t.Fatalf("different request published a second Credential response after exact recovery: %s", secondTerminal)
	}
	if _, statErr := os.Stat(conflictingPublicationPath); !os.IsNotExist(statErr) {
		t.Fatalf("rejected different request published a response: %v", statErr)
	}
}

// initializeSuccessorInstance prepares one fresh successor Instance root and
// persists its exact public request. The successor owner creates the root
// itself; an existing destination is never adopted.
func initializeSuccessorInstance(t *testing.T, directory, next, rootPath string, network [32]byte, now time.Time, requestPath string) ([]byte, string) {
	t.Helper()
	output := runCommand(t, t.Context(), directory, next, "publication", "instance-initialize", rootPath,
		hex.EncodeToString(network[:]), now.Format(time.RFC3339), now.Add(time.Hour).Format(time.RFC3339))
	var receipt struct {
		Outcome string `json:"outcome"`
		Request []byte `json:"request"`
	}
	if err := json.Unmarshal(output, &receipt); err != nil || receipt.Outcome != "instance-request-prepared" || len(receipt.Request) == 0 {
		t.Fatalf("successor instance initialization receipt = %+v / %v", receipt, err)
	}
	if err := os.WriteFile(requestPath, receipt.Request, 0o600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(receipt.Request)
	return receipt.Request, hex.EncodeToString(digest[:])
}

func assertTerminalPasswordHidden(t *testing.T, transcript []byte, password string) {
	t.Helper()
	if bytes.Contains(transcript, []byte(password)) {
		t.Fatal("custody command echoed its terminal password")
	}
}

func assertServiceInstanceAccepted(t *testing.T, directory, binary, rootPath, responsePath string) {
	t.Helper()
	output := runCommand(t, t.Context(), directory, binary,
		"publication", "instance-accept", rootPath, responsePath)
	var receipt struct {
		Outcome    string `json:"outcome"`
		Acceptance struct {
			Generation uint64 `json:"Generation"`
		} `json:"acceptance"`
	}
	if err := json.Unmarshal(output, &receipt); err != nil ||
		receipt.Outcome != "instance-response-accepted" || receipt.Acceptance.Generation != 1 {
		t.Fatalf("acceptance receipt = %+v / %v", receipt, err)
	}
}

func assertServiceInstanceUnavailable(t *testing.T, directory, binary, rootPath, responsePath string) {
	t.Helper()
	command := exec.Command(binary, "publication", "instance-accept", rootPath, responsePath)
	command.Dir = directory
	if output, err := command.CombinedOutput(); err == nil {
		t.Fatalf("unavailable Service Instance accepted a response: %s", output)
	}
}
