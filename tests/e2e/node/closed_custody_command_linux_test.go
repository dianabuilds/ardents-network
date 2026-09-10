//go:build linux

package state_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type closedCommandAuthority struct {
	Public                         [32]byte
	binary, root, record, identity string
	network                        [32]byte
}

func createClosedCommandAuthority(t *testing.T, network [32]byte) closedCommandAuthority {
	t.Helper()
	authority := closedCommandAuthority{binary: buildCommand(t, "ardents-custody"), root: t.TempDir(), network: network}
	output, err := closedCustodyCommand(t, authority.binary, []closedCustodyInput{
		{"vault-create password:", closedCustodyPassword, true},
		{"vault-create-confirm password:", closedCustodyPassword, true},
	}, append([]string{"create-admission-authority"}, authority.bindings()...)...)
	if err != nil {
		t.Fatalf("create admission authority command: %v / %s", err, output)
	}
	var receipt struct {
		Schema   string
		Record   string `json:"record_id"`
		Identity string `json:"id_commitment"`
		Public   string `json:"authority_public"`
	}
	decodeClosedCustodyReceipt(t, output, &receipt)
	public, err := hex.DecodeString(receipt.Public)
	if err != nil || len(public) != 32 || receipt.Schema != "ardents-admission-authority-v1" || receipt.Record == "" || receipt.Identity == "" {
		t.Fatalf("invalid admission authority receipt: %v", err)
	}
	copy(authority.Public[:], public)
	authority.record, authority.identity = receipt.Record, receipt.Identity
	return authority
}

func (authority closedCommandAuthority) bindings() []string {
	return []string{"--vault-root", authority.root, "--environment-commitment", identifierNode(21),
		"--network-commitment", hex.EncodeToString(authority.network[:]), "--root-commitment", identifierNode(22)}
}

func (authority closedCommandAuthority) issue(t *testing.T, request []byte) []byte {
	t.Helper()
	path, permissionPath := filepath.Join(t.TempDir(), "permission.request"), filepath.Join(t.TempDir(), "permission.bin")
	if err := os.WriteFile(path, request, 0o600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(request)
	args := append([]string{"issue-admission-permission"}, authority.bindings()...)
	args = append(args, "--record", authority.record, "--kind", "admission", "--id-commitment", authority.identity,
		"--request", path, "--permission-output", permissionPath)
	wrong := digest
	wrong[0] ^= 1
	rejected, refusal := closedCustodyCommand(t, authority.binary, []closedCustodyInput{
		{"admission-request SHA-256 from the requesting Endpoint:", hex.EncodeToString(wrong[:]), false},
	}, args...)
	_, outputErr := os.Lstat(permissionPath)
	if refusal == nil || !bytes.Contains(rejected, []byte("does not match the independently transferred commitment")) ||
		bytes.Contains(rejected, []byte("vault-unlock password:")) || !os.IsNotExist(outputErr) {
		t.Fatalf("Custody did not refuse the substituted request commitment before unlocking: %v / %s", refusal, rejected)
	}
	output, err := closedCustodyCommand(t, authority.binary, []closedCustodyInput{
		{"admission-request SHA-256 from the requesting Endpoint:", hex.EncodeToString(digest[:]), false},
		{"vault-unlock password:", closedCustodyPassword, true},
	}, args...)
	if err != nil {
		t.Fatalf("issue admission permission command: %v / %s", err, output)
	}
	permission, err := os.ReadFile(permissionPath)
	if err != nil {
		t.Fatal(err)
	}
	var receipt struct {
		Schema string
		Record string `json:"record_id"`
		Digest string `json:"permission_sha256"`
	}
	decodeClosedCustodyReceipt(t, output, &receipt)
	expected := sha256.Sum256(permission)
	if receipt.Schema != "ardents-admission-permission-receipt-v1" || receipt.Record != authority.record ||
		receipt.Digest != hex.EncodeToString(expected[:]) || len(permission) != 228 || bytes.Contains(output, permission) || bytes.Contains(output, request) {
		t.Fatal("Custody permission receipt does not match private output")
	}
	return permission
}

func decodeClosedCustodyReceipt(t *testing.T, output []byte, target any) {
	t.Helper()
	start, end := bytes.IndexByte(output, '{'), bytes.LastIndexByte(output, '}')
	if start < 0 || end < start {
		t.Fatalf("Custody command produced no receipt: %s", output)
	}
	if err := json.Unmarshal(output[start:end+1], target); err != nil {
		t.Fatal(err)
	}
}
