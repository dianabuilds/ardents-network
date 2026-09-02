//go:build linux && service_credential_response_linux

package service_test

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestLinuxCredentialResponsePublicationRecoversAfterFileSizeLimit(t *testing.T) {
	endpointBinary := buildProductCommand(t, "ardents")
	custodyBinary := buildProductCommand(t, "ardents-custody")
	directory := t.TempDir()
	vaultRoot := filepath.Join(directory, "authority-vault")
	network := [32]byte{71}
	environment, authorityRoot := [32]byte{72}, [32]byte{73}
	password := "file-size credential custody password"

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
		t.Fatalf("Service Authority receipt = %+v / %v", created, err)
	}

	instanceRoot := filepath.Join(directory, "instance-root")
	if err := os.Mkdir(instanceRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	requestPath := filepath.Join(directory, "instance-request.bin")
	planPath := filepath.Join(directory, "instance-plan.json")
	now := time.Now().UTC().Truncate(time.Second)
	plan, err := json.Marshal(map[string]string{
		"schema":       "ardents-service-instance-initialize-v1",
		"root":         instanceRoot,
		"network_id":   hex.EncodeToString(network[:]),
		"not_before":   now.Format(time.RFC3339),
		"not_after":    now.Add(time.Hour).Format(time.RFC3339),
		"request_file": requestPath,
	})
	if err != nil {
		t.Fatalf("marshal Service Instance plan: %v", err)
	}
	if err := os.WriteFile(planPath, plan, 0o600); err != nil {
		t.Fatalf("write Service Instance plan: %v", err)
	}
	initialized := runCommand(t, t.Context(), directory, endpointBinary,
		"service-instance", "initialize", "--config", planPath)
	var initialization struct {
		Schema        string `json:"schema"`
		Request       []byte `json:"request"`
		RequestSHA256 string `json:"request_sha256"`
	}
	if err := json.Unmarshal(initialized, &initialization); err != nil ||
		initialization.Schema != "ardents-service-instance-request-v1" || initialization.RequestSHA256 == "" {
		t.Fatalf("Service Instance initialization receipt = %+v / %v", initialization, err)
	}
	request, err := os.ReadFile(requestPath)
	if err != nil || !bytes.Equal(request, initialization.Request) {
		t.Fatalf("public request differs from its receipt: %v", err)
	}

	issueArguments := func(responsePath string) []string {
		return []string{"issue-service-credential", "-vault-root", vaultRoot, "-record", created.RecordID,
			"-request", requestPath, "-response", responsePath,
			"-environment-commitment", hex.EncodeToString(environment[:]),
			"-network-commitment", hex.EncodeToString(network[:]),
			"-root-commitment", hex.EncodeToString(authorityRoot[:]),
			"-kind", "service", "-id-commitment", created.IDCommitment}
	}
	issueInputs := []interactiveProductInput{
		{prompt: "service-request SHA-256 from the requesting host:", value: initialization.RequestSHA256},
		{prompt: "vault-unlock password:", value: password, secret: true},
	}
	establishedResponsePath := filepath.Join(directory, "established-response.bin")
	establishedTerminal := runInteractiveProductCommand(t, directory, custodyBinary, issueInputs, issueArguments(establishedResponsePath)...)
	assertTerminalPasswordHidden(t, establishedTerminal, password)
	var established struct {
		Schema         string `json:"schema"`
		RecordID       string `json:"record_id"`
		Generation     uint64 `json:"generation"`
		Response       []byte `json:"response"`
		ResponseSHA256 string `json:"response_sha256"`
	}
	if err := json.Unmarshal(interactiveProductJSON(t, establishedTerminal), &established); err != nil ||
		established.Schema != "ardents-service-credential-response-v1" || established.RecordID == "" ||
		established.RecordID == created.RecordID || established.Generation != 1 || len(established.Response) == 0 ||
		established.ResponseSHA256 == "" {
		t.Fatalf("established Credential response receipt = %+v / %v", established, err)
	}
	if persisted, readErr := os.ReadFile(establishedResponsePath); readErr != nil || !bytes.Equal(persisted, established.Response) {
		t.Fatalf("established public response differs from receipt: %v", readErr)
	}

	responsePath := filepath.Join(directory, "instance-response.bin")
	limitedTerminal, limitedErr := runInteractiveProductCommandWithFileSizeLimitResult(t, directory, custodyBinary, issueInputs, issueArguments(responsePath)...)
	assertTerminalPasswordHidden(t, limitedTerminal, password)
	assertFileSizeLimitFailure(t, limitedErr, limitedTerminal)
	if bytes.Contains(limitedTerminal, []byte("ardents-service-credential-response-v1")) {
		t.Fatalf("file-size-limited custody command emitted a response receipt: %s", limitedTerminal)
	}
	visibleResponse := assertZeroByteVisibleCredentialResponse(t, responsePath)

	retryTerminal, retryErr := runInteractiveProductCommandResult(t, directory, custodyBinary, issueInputs, issueArguments(responsePath)...)
	assertTerminalPasswordHidden(t, retryTerminal, password)
	if retryErr != nil {
		t.Fatalf("exact Credential retry after RLIMIT_FSIZE failure did not recover: first=%v visible_response={mode=%s,size=%d} retry=%v\n%s",
			limitedErr, visibleResponse.Mode(), visibleResponse.Size(), retryErr, retryTerminal)
	}
	var retried struct {
		Schema         string `json:"schema"`
		RecordID       string `json:"record_id"`
		Generation     uint64 `json:"generation"`
		Response       []byte `json:"response"`
		ResponseSHA256 string `json:"response_sha256"`
	}
	if err := json.Unmarshal(interactiveProductJSON(t, retryTerminal), &retried); err != nil ||
		retried.Schema != "ardents-service-credential-response-v1" || retried.RecordID != established.RecordID ||
		retried.Generation != established.Generation || !bytes.Equal(retried.Response, established.Response) ||
		retried.ResponseSHA256 != established.ResponseSHA256 {
		t.Fatalf("recovered Credential response receipt = %+v / %v", retried, err)
	}
	persisted, err := os.ReadFile(responsePath)
	if err != nil || !bytes.Equal(persisted, retried.Response) {
		t.Fatalf("recovered public response differs from receipt: %v", err)
	}
	assertServiceInstanceAccepted(t, directory, endpointBinary, instanceRoot, responsePath)
}

func runInteractiveProductCommandWithFileSizeLimitResult(t *testing.T, directory, binary string, inputs []interactiveProductInput, arguments ...string) ([]byte, error) {
	t.Helper()
	shellArguments := []string{"-c", "ulimit -f 0; exec \"$@\"", "ardents-custody-file-size-limit", binary}
	shellArguments = append(shellArguments, arguments...)
	return runInteractiveProductCommandResult(t, directory, "/bin/sh", inputs, shellArguments...)
}

func assertFileSizeLimitFailure(t *testing.T, err error, terminal []byte) {
	t.Helper()
	if err == nil {
		t.Fatal("file-size-limited custody command succeeded")
	}
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) {
		t.Fatalf("file-size-limited custody command did not return an exit error: %v", err)
	}
	status, ok := exitError.ProcessState.Sys().(syscall.WaitStatus)
	if ok && status.Signaled() && status.Signal() == syscall.SIGXFSZ {
		return
	}
	if ok && status.Exited() && status.ExitStatus() != 0 && bytes.Contains(bytes.ToLower(terminal), []byte("file too large")) {
		return
	}
	t.Fatalf("file-size-limited custody termination = %#v / %s, want SIGXFSZ or a nonzero file-too-large failure", exitError.ProcessState.Sys(), terminal)
}

func assertZeroByteVisibleCredentialResponse(t *testing.T, path string) os.FileInfo {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("file-size-limited custody response is not visible: %v", err)
	}
	if !info.Mode().IsRegular() || info.Size() != 0 {
		t.Fatalf("file-size-limited custody response = mode=%s size=%d, want a zero-byte regular file", info.Mode(), info.Size())
	}
	body, err := os.ReadFile(path)
	if err != nil || len(body) != 0 {
		t.Fatalf("file-size-limited custody response bytes = %d / %v, want zero", len(body), err)
	}
	return info
}
