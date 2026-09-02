package service_test

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/service/instance"
)

func TestHeadlessServiceInstanceAcquisitionIsAtMostOnceAcrossProcesses(t *testing.T) {
	binary := buildProductCommand(t, "ardents")
	custodyBinary := buildProductCommand(t, "ardents-custody")
	directory := t.TempDir()
	vaultRoot := filepath.Join(directory, "authority-vault")
	rootPath := filepath.Join(directory, "instance-root")
	if err := os.Mkdir(rootPath, 0o700); err != nil {
		t.Fatal(err)
	}
	network := [32]byte{41}
	environment, authorityRoot := [32]byte{42}, [32]byte{43}
	password := "artifact native custody password"
	createdTerminal := runInteractiveProductCommand(t, directory, custodyBinary,
		[]interactiveProductInput{{prompt: "vault-create password:", value: password},
			{prompt: "vault-create-confirm password:", value: password}},
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
	planPath := filepath.Join(directory, "instance-plan.json")
	plan, err := json.Marshal(map[string]string{
		"schema":       "ardents-service-instance-initialize-v1",
		"root":         rootPath,
		"network_id":   hex.EncodeToString(network[:]),
		"not_before":   now.Format(time.RFC3339),
		"not_after":    now.Add(time.Hour).Format(time.RFC3339),
		"request_file": requestPath,
	})
	if err != nil || os.WriteFile(planPath, plan, 0o600) != nil {
		t.Fatal("write Service Instance process plan")
	}

	initialized := runCommand(t, t.Context(), directory, binary,
		"service-instance", "initialize", "--config", planPath)
	var initialization struct {
		Schema        string `json:"schema"`
		Request       []byte `json:"request"`
		RequestSHA256 string `json:"request_sha256"`
	}
	if err := json.Unmarshal(initialized, &initialization); err != nil ||
		initialization.Schema != "ardents-service-instance-request-v1" || initialization.RequestSHA256 == "" {
		t.Fatalf("initialization receipt = %+v / %v", initialization, err)
	}
	request, err := os.ReadFile(requestPath)
	if err != nil || string(request) != string(initialization.Request) {
		t.Fatalf("persisted public request differs: %v", err)
	}
	requestView, err := instance.ParseRequest(request)
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
		[]interactiveProductInput{{prompt: "service-request SHA-256 from the requesting host:", value: initialization.RequestSHA256},
			{prompt: "vault-unlock password:", value: password}}, issueArguments...)
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
		[]interactiveProductInput{{prompt: "service-request SHA-256 from the requesting host:", value: initialization.RequestSHA256},
			{prompt: "vault-unlock password:", value: password}}, issueArguments...)
	var repeated struct {
		RecordID string `json:"record_id"`
		Response []byte `json:"response"`
	}
	if err := json.Unmarshal(interactiveProductJSON(t, repeatedTerminal), &repeated); err != nil ||
		repeated.RecordID != issued.RecordID || !bytes.Equal(repeated.Response, response) {
		t.Fatalf("custody exact retry changed its public result: %+v / %v", repeated, err)
	}
	assertServiceInstanceAccepted(t, directory, binary, rootPath, responsePath)
	assertServiceInstanceAccepted(t, directory, binary, rootPath, responsePath)

	conflicting := append([]byte(nil), response...)
	conflicting[len(conflicting)-1] ^= 0xff
	conflictingPath := filepath.Join(directory, "conflicting-response.bin")
	if err := os.WriteFile(conflictingPath, conflicting, 0o600); err != nil {
		t.Fatal(err)
	}
	assertServiceInstanceUnavailable(t, directory, binary, rootPath, conflictingPath)
	assertServiceInstanceUnavailable(t, directory, binary, rootPath, responsePath)
}

func TestHeadlessCredentialResponsePublicationFailsAtomicallyAndRetriesExactlyOnce(t *testing.T) {
	binary := buildProductCommand(t, "ardents")
	custodyBinary := buildProductCommand(t, "ardents-custody")
	directory := t.TempDir()
	vaultRoot := filepath.Join(directory, "authority-vault")
	network := [32]byte{61}
	environment, authorityRoot := [32]byte{62}, [32]byte{63}
	password := "atomic credential response custody password"
	createdTerminal := runInteractiveProductCommand(t, directory, custodyBinary,
		[]interactiveProductInput{{prompt: "vault-create password:", value: password},
			{prompt: "vault-create-confirm password:", value: password}},
		"create-service-authority", "-vault-root", vaultRoot,
		"-environment-commitment", hex.EncodeToString(environment[:]),
		"-network-commitment", hex.EncodeToString(network[:]),
		"-root-commitment", hex.EncodeToString(authorityRoot[:]))
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
		if err := os.Mkdir(rootPath, 0o700); err != nil {
			t.Fatal(err)
		}
		requestPath := filepath.Join(directory, name+"-request.bin")
		planPath := filepath.Join(directory, name+"-plan.json")
		plan, err := json.Marshal(map[string]string{
			"schema":       "ardents-service-instance-initialize-v1",
			"root":         rootPath,
			"network_id":   hex.EncodeToString(network[:]),
			"not_before":   now.Format(time.RFC3339),
			"not_after":    now.Add(time.Hour).Format(time.RFC3339),
			"request_file": requestPath,
		})
		if err != nil || os.WriteFile(planPath, plan, 0o600) != nil {
			t.Fatal("write Service Instance process plan")
		}
		initialized := runCommand(t, t.Context(), directory, binary,
			"service-instance", "initialize", "--config", planPath)
		var receipt struct {
			Schema        string `json:"schema"`
			RequestSHA256 string `json:"request_sha256"`
		}
		if err := json.Unmarshal(initialized, &receipt); err != nil ||
			receipt.Schema != "ardents-service-instance-request-v1" || receipt.RequestSHA256 == "" {
			t.Fatalf("Service Instance initialization receipt = %+v / %v", receipt, err)
		}
		return initializedInstance{rootPath: rootPath, requestPath: requestPath, requestSHA256: receipt.RequestSHA256}
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
		[]interactiveProductInput{{prompt: "service-request SHA-256 from the requesting host:", value: blocked.requestSHA256},
			{prompt: "vault-unlock password:", value: password}}, issueArguments(blocked.requestPath, conflictingResponsePath)...)
	if err == nil || !bytes.Contains(failedTerminal, []byte("service Credential response destination conflicts")) {
		t.Fatalf("conflicting Credential response publication = %v / %s", err, failedTerminal)
	}
	if stored, readErr := os.ReadFile(conflictingResponsePath); readErr != nil || !bytes.Equal(stored, conflictingResponse) {
		t.Fatalf("conflicting public response changed after failed publication: %q / %v", stored, readErr)
	}

	recoveredResponsePath := filepath.Join(directory, "recovered-response.bin")
	recoveredTerminal := runInteractiveProductCommand(t, directory, custodyBinary,
		[]interactiveProductInput{{prompt: "service-request SHA-256 from the requesting host:", value: blocked.requestSHA256},
			{prompt: "vault-unlock password:", value: password}}, issueArguments(blocked.requestPath, recoveredResponsePath)...)
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
		[]interactiveProductInput{{prompt: "service-request SHA-256 from the requesting host:", value: blocked.requestSHA256},
			{prompt: "vault-unlock password:", value: password}}, issueArguments(blocked.requestPath, recoveredResponsePath)...)
	var repeated struct {
		RecordID string `json:"record_id"`
		Response []byte `json:"response"`
	}
	if err := json.Unmarshal(interactiveProductJSON(t, repeatedTerminal), &repeated); err != nil ||
		repeated.RecordID != recovered.RecordID || !bytes.Equal(repeated.Response, recovered.Response) {
		t.Fatalf("exact Credential response retry changed its result: %+v / %v", repeated, err)
	}
	assertServiceInstanceAccepted(t, directory, binary, blocked.rootPath, recoveredResponsePath)
	assertServiceInstanceAccepted(t, directory, binary, blocked.rootPath, recoveredResponsePath)

	conflicting := initialize("conflicting-instance")
	conflictingPublicationPath := filepath.Join(directory, "second-response.bin")
	secondTerminal, secondErr := runInteractiveProductCommandResult(t, directory, custodyBinary,
		[]interactiveProductInput{{prompt: "service-request SHA-256 from the requesting host:", value: conflicting.requestSHA256},
			{prompt: "vault-unlock password:", value: password}}, issueArguments(conflicting.requestPath, conflictingPublicationPath)...)
	if secondErr == nil {
		t.Fatalf("different request published a second Credential response after exact recovery: %s", secondTerminal)
	}
	if _, statErr := os.Stat(conflictingPublicationPath); !os.IsNotExist(statErr) {
		t.Fatalf("rejected different request published a response: %v", statErr)
	}
}

func assertServiceInstanceAccepted(t *testing.T, directory, binary, rootPath, responsePath string) {
	t.Helper()
	output := runCommand(t, t.Context(), directory, binary,
		"service-instance", "accept", "--root", rootPath, "--response", responsePath)
	var receipt struct {
		Schema     string `json:"schema"`
		State      string `json:"state"`
		Generation uint64 `json:"generation"`
	}
	if err := json.Unmarshal(output, &receipt); err != nil ||
		receipt.Schema != "ardents-service-instance-acceptance-v1" ||
		receipt.State != "accepted" || receipt.Generation != 1 {
		t.Fatalf("acceptance receipt = %+v / %v", receipt, err)
	}
}

func assertServiceInstanceUnavailable(t *testing.T, directory, binary, rootPath, responsePath string) {
	t.Helper()
	command := exec.Command(binary, "service-instance", "accept", "--root", rootPath, "--response", responsePath)
	command.Dir = directory
	if output, err := command.CombinedOutput(); err == nil {
		t.Fatalf("unavailable Service Instance accepted a response: %s", output)
	}
}
