//go:build linux && text_worker_installed

package endpoint

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/service/instance"
)

func acquireInstalledServiceInstance(t *testing.T, network [32]byte, now, until time.Time) (*instance.Root, *instance.Binding) {
	t.Helper()
	endpointBinary := installedServiceCommandArtifact(t, "ardents", "ARDENTS_TEXT_ENDPOINT_COMMAND_SHA256")
	custodyBinary := installedServiceCommandArtifact(t, "ardents-custody", "ARDENTS_TEXT_CUSTODY_COMMAND_SHA256")
	directory := textNetworkPrivateRoot(t)
	rootPath := serviceInstanceFixtureRoot(t)
	encode := func(value [32]byte) string { return hex.EncodeToString(value[:]) }
	bindings := []string{"--vault-root", filepath.Join(directory, "vault"), "--environment-commitment", encode(fixtureID(231)), "--network-commitment", encode(network), "--root-commitment", encode(fixtureID(232))}
	output, err := installedServiceCustodyCommand(t, custodyBinary, []installedServiceCustodyInput{
		{"vault-create password:", installedServiceCustodyPassword, true},
		{"vault-create-confirm password:", installedServiceCustodyPassword, true},
	}, append([]string{"create-service-authority"}, bindings...)...)
	if err != nil {
		t.Fatalf("create installed Service Authority: %v", err)
	}
	var authority struct {
		Schema   string
		Record   string `json:"record_id"`
		Identity string `json:"id_commitment"`
	}
	decodeInstalledServiceReceipt(t, output, &authority)
	if authority.Schema != "ardents-service-authority-v1" || authority.Record == "" || authority.Identity == "" {
		t.Fatal("invalid Service Authority receipt")
	}
	requestPath, responsePath := filepath.Join(directory, "request"), filepath.Join(directory, "response")
	plan, err := json.Marshal(map[string]string{"schema": "ardents-service-instance-initialize-v1", "root": rootPath, "network_id": encode(network), "not_before": now.Format(time.RFC3339), "not_after": until.Format(time.RFC3339), "request_file": requestPath})
	if err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(directory, "instance.json")
	if err := os.WriteFile(planPath, plan, 0600); err != nil {
		t.Fatal(err)
	}
	initialized := runInstalledServiceCommand(t, endpointBinary, "service-instance", "initialize", "--config", planPath)
	var request struct {
		Schema  string
		Digest  string `json:"request_sha256"`
		Request []byte
	}
	if err := json.Unmarshal(initialized, &request); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(requestPath)
	digest := sha256.Sum256(raw)
	if err != nil || request.Schema != "ardents-service-instance-request-v1" || request.Digest != hex.EncodeToString(digest[:]) || !bytes.Equal(raw, request.Request) {
		t.Fatal("Instance request command/file mismatch")
	}
	args := append([]string{"issue-service-credential"}, bindings...)
	args = append(args, "--record", authority.Record, "--kind", "service", "--id-commitment", authority.Identity, "--request", requestPath, "--response", responsePath)
	output, err = installedServiceCustodyCommand(t, custodyBinary, []installedServiceCustodyInput{
		{"service-request SHA-256 from the requesting Endpoint:", request.Digest, false},
		{"vault-unlock password:", installedServiceCustodyPassword, true},
	}, args...)
	if err != nil {
		t.Fatalf("issue installed Service Credential: %v", err)
	}
	var response struct {
		Schema     string
		Response   []byte
		Generation uint64
	}
	decodeInstalledServiceReceipt(t, output, &response)
	raw, err = os.ReadFile(responsePath)
	if err != nil || response.Schema != "ardents-service-credential-response-v1" || response.Generation != 1 || !bytes.Equal(raw, response.Response) {
		t.Fatal("Service Credential command/file mismatch")
	}
	runInstalledServiceCommand(t, endpointBinary, "service-instance", "accept", "--root", rootPath, "--response", responsePath)
	root, err := instance.Open(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := root.OpenBinding(0)
	if err != nil {
		_ = root.Close()
		t.Fatal(err)
	}
	return root, binding
}

func installedServiceCommandArtifact(t *testing.T, name, pinName string) string {
	t.Helper()
	path := "/usr/lib/ardents/qualification/" + name
	expected := os.Getenv(pinName)
	raw, err := readTextInstalledFile(path, 128<<20)
	if err != nil {
		t.Fatalf("installed %s unavailable: %v", name, err)
	}
	digest := sha256.Sum256(raw)
	if expected != hex.EncodeToString(digest[:]) {
		t.Fatalf("installed %s independent digest mismatch", name)
	}
	return path
}

func runInstalledServiceCommand(t *testing.T, binary string, arguments ...string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary, arguments...)
	command.WaitDelay = 5 * time.Second
	output, diagnostic := &installedTextOutput{limit: 128 << 10, cancel: cancel}, &installedTextOutput{limit: 4096, cancel: cancel}
	command.Stdout, command.Stderr = output, diagnostic
	if err := command.Run(); err != nil || ctx.Err() != nil || diagnostic.buffer.Len() != 0 {
		t.Fatalf("installed Instance command failed: %v / %v", err, ctx.Err())
	}
	return output.buffer.Bytes()
}

func decodeInstalledServiceReceipt(t *testing.T, output []byte, target any) {
	t.Helper()
	start, end := bytes.IndexByte(output, '{'), bytes.LastIndexByte(output, '}')
	if start < 0 || end < start {
		t.Fatal("installed custody receipt absent")
	}
	if err := json.Unmarshal(output[start:end+1], target); err != nil {
		t.Fatal(err)
	}
}
