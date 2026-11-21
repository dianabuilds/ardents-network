//go:build linux && text_worker_installed

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/publication"
)

func installedRecoveryInstance(t *testing.T, network [32]byte) (publicationPlan, [32]byte) {
	t.Helper()
	next := recoveryPinnedCommand(t, "ardents-next", "ARDENTS_RECOVERY_INSTANCE_SHA256")
	custody := recoveryPinnedCommand(t, "ardents-custody", "ARDENTS_RECOVERY_CUSTODY_SHA256")
	return issueRecoveryInstance(t, next, custody, network)
}

// The same accepted native Instance is consumed by the recovered Publisher.
// Both artifact-driven commands must finish successfully before Route starts.
func issueRecoveryInstance(t *testing.T, next, custody string, network [32]byte) (publicationPlan, [32]byte) {
	t.Helper()
	directory := t.TempDir()
	plan := publicationPlan{InstanceRoot: filepath.Join(directory, "instance"), Root: filepath.Join(directory, "publication")}
	now := time.Now().UTC().Truncate(time.Second)
	encoded := func(value [32]byte) string { return hex.EncodeToString(value[:]) }
	prepared := recoveryInstanceCommand(t, next, "publication", "instance-initialize", plan.InstanceRoot, encoded(network), now.Format(time.RFC3339), now.Add(time.Hour).Format(time.RFC3339))
	observed := recoveryInstanceCommand(t, next, "publication", "instance-request", plan.InstanceRoot)
	var request struct {
		Outcome string `json:"outcome"`
		Request []byte `json:"request"`
	}
	if err := json.Unmarshal(prepared, &request); err != nil || request.Outcome != "instance-request-prepared" || !bytes.Equal(prepared, observed) {
		t.Fatal("Instance request is unavailable or changed after process reopen", err)
	}
	view, err := publication.ParseServiceRequest(request.Request)
	if err != nil {
		t.Fatal("Instance request could not be verified", err)
	}
	requestPath, responsePath := filepath.Join(directory, "request.bin"), filepath.Join(directory, "response.bin")
	if err := os.WriteFile(requestPath, request.Request, 0o600); err != nil {
		t.Fatal(err)
	}

	bindings := []string{"--vault-root", filepath.Join(directory, "vault"), "--environment-commitment", encoded([32]byte{81}),
		"--network-commitment", encoded(network), "--root-commitment", encoded([32]byte{83})}
	created, err := recoveryCustodyCommand(t, custody, []recoveryCustodyInput{
		{prompt: "vault-create password:", value: recoveryCustodyPassword, secret: true},
		{prompt: "vault-create-confirm password:", value: recoveryCustodyPassword, secret: true},
	}, append([]string{"create-service-authority"}, bindings...)...)
	if err != nil {
		t.Fatal("Credential authority command failed", err)
	}
	var authority struct {
		Schema   string `json:"schema"`
		RecordID string `json:"record_id"`
		Identity string `json:"id_commitment"`
		Public   string `json:"authority_public"`
		Target   string `json:"target"`
	}
	recoveryCustodyJSON(t, created, &authority)
	if authority.Schema != "ardents-service-authority-v1" || authority.RecordID == "" || authority.Identity == "" {
		t.Fatal("Credential authority receipt is incomplete")
	}
	arguments := append([]string{"issue-service-credential"}, bindings...)
	arguments = append(arguments, "--record", authority.RecordID, "--kind", "service", "--id-commitment", authority.Identity, "--request", requestPath, "--response", responsePath)
	issued, err := recoveryCustodyCommand(t, custody, []recoveryCustodyInput{
		{prompt: "service-request SHA-256", value: encoded(sha256.Sum256(request.Request))},
		{prompt: "vault-unlock password:", value: recoveryCustodyPassword, secret: true},
	}, arguments...)
	if err != nil {
		t.Fatal("Credential issuance command failed", err)
	}
	var response struct {
		Schema     string `json:"schema"`
		Response   []byte `json:"response"`
		Generation uint64 `json:"generation"`
	}
	recoveryCustodyJSON(t, issued, &response)
	raw, err := os.ReadFile(responsePath)
	if err != nil || response.Schema != "ardents-service-credential-response-v1" || response.Generation != 1 || !bytes.Equal(raw, response.Response) {
		t.Fatal("Credential command/file evidence differs", err)
	}
	credential, err := publication.VerifyServiceResponse(raw, view)
	if err != nil {
		t.Fatal("Credential could not be verified against the original request", err)
	}
	delegation := credential.Delegation()
	if delegation.Generation != 1 || encoded(delegation.Authority) != authority.Public || encoded(delegation.Target) != authority.Target {
		t.Fatal("Credential does not belong to the original Authority")
	}
	accepted := recoveryInstanceCommand(t, next, "publication", "instance-accept", plan.InstanceRoot, responsePath)
	var acceptance struct {
		Outcome string `json:"outcome"`
	}
	if err := json.Unmarshal(accepted, &acceptance); err != nil || acceptance.Outcome != "instance-response-accepted" {
		t.Fatal("original Instance did not accept the issued Credential", err)
	}
	return plan, delegation.Target
}

func recoveryCustodyJSON(t *testing.T, transcript []byte, result any) {
	t.Helper()
	start, end := bytes.IndexByte(transcript, '{'), bytes.LastIndexByte(transcript, '}')
	if start < 0 || end <= start {
		t.Fatal("Custody receipt is absent")
	}
	if err := json.Unmarshal(transcript[start:end+1], result); err != nil {
		t.Fatal("Custody receipt is invalid", err)
	}
}

func recoveryInstanceCommand(t *testing.T, binary string, arguments ...string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, binary, arguments...)
	command.WaitDelay = 5 * time.Second
	output := &recoveryCustodyCapture{cancel: cancel}
	var diagnostic recoveryCustodyCapture
	diagnostic.cancel = cancel
	command.Stdout, command.Stderr = output, &diagnostic
	if err := command.Run(); err != nil || ctx.Err() != nil || len(diagnostic.bytes()) != 0 {
		t.Fatal("Instance process did not complete cleanly", err, ctx.Err())
	}
	return output.bytes()
}

func recoveryPinnedCommand(t *testing.T, name, pinVariable string) string {
	t.Helper()
	const directory = "/usr/lib/ardents/qualification"
	path := filepath.Join(directory, name)
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		t.Fatal("qualification command is unavailable", err)
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		t.Fatal("qualification command identity is unavailable", err)
	}
	identity, ok := stat.Sys().(*syscall.Stat_t)
	if !ok || identity.Uid != 0 || identity.Gid != 0 || !stat.Mode().IsRegular() || stat.Mode().Perm() != 0o555 || stat.Size() <= 0 || stat.Size() > 128<<20 {
		t.Fatal("qualification command is not an immutable installed candidate")
	}
	digest := sha256.New()
	size, err := io.Copy(digest, io.LimitReader(file, stat.Size()+1))
	if err != nil || size != stat.Size() || hex.EncodeToString(digest.Sum(nil)) != os.Getenv(pinVariable) {
		t.Fatal("qualification command differs from its independent pin", err)
	}
	return path
}
