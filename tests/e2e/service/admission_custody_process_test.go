package service_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
)

func TestAdmissionCustodyBuiltTerminalIssuesCanonicalPermissionAndRetainsRetry(t *testing.T) {
	binary := buildProductCommand(t, "ardents-custody")
	directory := t.TempDir()
	vault := filepath.Join(directory, "independent-admission-vault")
	environment, network, root := [32]byte{61}, [32]byte{62}, [32]byte{63}
	password := "native admission custody password"
	createdTerminal := runInteractiveProductCommand(t, directory, binary, []interactiveProductInput{
		{prompt: "vault-create password:", value: password, secret: true},
		{prompt: "vault-create-confirm password:", value: password, secret: true},
	}, "create-admission-authority", "-vault-root", vault, "-environment-commitment", hex.EncodeToString(environment[:]), "-network-commitment", hex.EncodeToString(network[:]), "-root-commitment", hex.EncodeToString(root[:]))
	var created struct {
		RecordID     string `json:"record_id"`
		IDCommitment string `json:"id_commitment"`
		Public       string `json:"authority_public"`
	}
	if err := json.Unmarshal(interactiveProductJSON(t, createdTerminal), &created); err != nil {
		t.Fatal(err)
	}
	public, err := hex.DecodeString(created.Public)
	if err != nil || len(public) != 32 || created.RecordID == "" {
		t.Fatalf("invalid authority receipt: %+v / %v", created, err)
	}
	now := time.Now().UTC().Truncate(time.Hour)
	request, holder, err := admission.PreparePermissionRequest([32]byte(public), network, [32]byte{64}, 65, admission.AllocationUser, now, [3]uint32{3, 0, 0})
	if err != nil {
		t.Fatal(err)
	}
	defer clear(holder)
	raw, err := admission.EncodePermissionRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	path, output := filepath.Join(directory, "holder.request"), filepath.Join(directory, "permission.bin")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	arguments := []string{"issue-admission-permission", "-vault-root", vault, "-record", created.RecordID, "-environment-commitment", hex.EncodeToString(environment[:]), "-network-commitment", hex.EncodeToString(network[:]), "-root-commitment", hex.EncodeToString(root[:]), "-kind", "admission", "-id-commitment", created.IDCommitment, "-request", path, "-permission-output", output}
	inputs := []interactiveProductInput{{prompt: "admission-request SHA-256", value: hex.EncodeToString(digest[:])}, {prompt: "vault-unlock password:", value: password, secret: true}}
	issued := runInteractiveProductCommand(t, directory, binary, inputs, arguments...)
	permissionRaw, err := os.ReadFile(output)
	if err != nil || len(permissionRaw) != 228 {
		t.Fatalf("canonical output: %v / %d", err, len(permissionRaw))
	}
	permission, err := admission.DecodePermission(permissionRaw)
	if err != nil {
		t.Fatal(err)
	}
	if err := admission.VerifyPermission(permission, ed25519.PublicKey(public), network, [32]byte{64}, 65, now); err != nil {
		t.Fatal(err)
	}
	// Independently reconstruct the accepted signature transcript from exact wire
	// bytes rather than using Custody or Admission's transcript function.
	transcript := append([]byte("ardents-issuance-permission-v1\x00"), permissionRaw[:164]...)
	if !ed25519.Verify(ed25519.PublicKey(public), transcript, permissionRaw[164:]) {
		t.Fatal("independent wire signature refused")
	}
	repeated := runInteractiveProductCommand(t, directory, binary, inputs, arguments...)
	after, err := os.ReadFile(output)
	if err != nil || !bytes.Equal(after, permissionRaw) || !bytes.Equal(interactiveProductJSON(t, issued), interactiveProductJSON(t, repeated)) {
		t.Fatalf("retry changed retained result: %v", err)
	}
	for _, transcript := range [][]byte{createdTerminal, issued, repeated} {
		if bytes.Contains(transcript, []byte(password)) || bytes.Contains(transcript, holder) || bytes.Contains(transcript, permissionRaw) {
			t.Fatal("private secret or Permission body escaped terminal receipt")
		}
	}
}

func TestAdmissionCustodyTerminalInterruptJoinsWithoutRecord(t *testing.T) {
	binary := buildProductCommand(t, "ardents-custody")
	directory := t.TempDir()
	root := filepath.Join(directory, "interrupted-vault")
	one, two, three := [32]byte{71}, [32]byte{72}, [32]byte{73}
	arguments := []string{"create-admission-authority", "-vault-root", root, "-environment-commitment", hex.EncodeToString(one[:]), "-network-commitment", hex.EncodeToString(two[:]), "-root-commitment", hex.EncodeToString(three[:])}
	transcript, err := runInteractiveProductCommandResult(t, directory, binary, []interactiveProductInput{{prompt: "vault-create password:", value: "\x03", secret: true}}, arguments...)
	if err == nil || !bytes.Contains(transcript, []byte("context canceled")) {
		t.Fatalf("terminal interrupt did not retain cancellation: %v / %s", err, transcript)
	}
	entries, err := os.ReadDir(filepath.Join(root, "records"))
	if err != nil || len(entries) != 0 {
		t.Fatalf("interrupted terminal created record: %v / %v", entries, err)
	}
	// A fresh operation can reacquire the joined original lease. It does not
	// reuse a supplied success flag or the interrupted operation's private key.
	runInteractiveProductCommand(t, directory, binary, []interactiveProductInput{{prompt: "vault-create password:", value: "after interrupt custody password", secret: true}, {prompt: "vault-create-confirm password:", value: "after interrupt custody password", secret: true}}, arguments...)
}
