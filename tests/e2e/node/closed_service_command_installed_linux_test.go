//go:build linux && text_worker_installed

package state_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func acquireCommandServiceInstance(t *testing.T, endpointBinary string, network [32]byte, now, until time.Time) string {
	t.Helper()
	custodyBinary := buildCommand(t, "ardents-custody")
	directory := t.TempDir()
	rootPath := filepath.Join(t.TempDir(), "instance")
	encode := func(value [32]byte) string { return hex.EncodeToString(value[:]) }
	bindings := []string{"--vault-root", filepath.Join(directory, "vault"), "--environment-commitment", encode([32]byte{231}), "--network-commitment", encode(network), "--root-commitment", encode([32]byte{232})}
	output, err := closedCustodyCommand(t, custodyBinary, []closedCustodyInput{
		{"vault-create password:", closedCustodyPassword, true},
		{"vault-create-confirm password:", closedCustodyPassword, true},
	}, append([]string{"create-service-authority"}, bindings...)...)
	if err != nil {
		t.Fatalf("create installed Service Authority: %v", err)
	}
	var authority struct {
		Schema   string
		Record   string `json:"record_id"`
		Identity string `json:"id_commitment"`
	}
	decodeClosedCustodyReceipt(t, output, &authority)
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
	initialized := runCommandServiceProvisioning(t, endpointBinary, "service-instance", "initialize", "--config", planPath)
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
	output, err = closedCustodyCommand(t, custodyBinary, []closedCustodyInput{
		{"service-request SHA-256 from the requesting Endpoint:", request.Digest, false},
		{"vault-unlock password:", closedCustodyPassword, true},
	}, args...)
	if err != nil {
		t.Fatalf("issue installed Service Credential: %v", err)
	}
	var response struct {
		Schema     string
		Response   []byte
		Generation uint64
	}
	decodeClosedCustodyReceipt(t, output, &response)
	raw, err = os.ReadFile(responsePath)
	if err != nil || response.Schema != "ardents-service-credential-response-v1" || response.Generation != 1 || !bytes.Equal(raw, response.Response) {
		t.Fatal("Service Credential command/file mismatch")
	}
	runCommandServiceProvisioning(t, endpointBinary, "service-instance", "accept", "--root", rootPath, "--response", responsePath)
	return rootPath
}

func runCommandServiceProvisioning(t *testing.T, binary string, arguments ...string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	output, diagnostic, err := installedCommandExec(ctx, nil, binary, arguments...)
	if err != nil {
		t.Fatalf("Service provisioning command: %v / %s", err, diagnostic)
	}
	return output
}
