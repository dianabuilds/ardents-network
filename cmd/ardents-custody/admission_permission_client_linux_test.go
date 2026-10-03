//go:build linux

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/custody"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAdmissionAuthorityCommandsKeepPermissionOutOfPublicReceipt(t *testing.T) {
	vaultRoot := t.TempDir()
	environment, network, authorityRoot := [32]byte{21}, [32]byte{22}, [32]byte{23}
	password := []byte("admission custody command password")
	createArguments := []string{"create-admission-authority", "-vault-root", vaultRoot,
		"-environment-commitment", hex.EncodeToString(environment[:]),
		"-network-commitment", hex.EncodeToString(network[:]), "-root-commitment", hex.EncodeToString(authorityRoot[:])}
	var createdOutput bytes.Buffer
	if err := run(t.Context(), createArguments, &createdOutput, &sequenceCommandSecrets{values: [][]byte{password, password}}); err != nil {
		t.Fatalf("create admission authority: %v", err)
	}
	var created struct {
		Schema          string `json:"schema"`
		RecordID        string `json:"record_id"`
		IDCommitment    string `json:"id_commitment"`
		AuthorityPublic string `json:"authority_public"`
	}
	if err := json.Unmarshal(createdOutput.Bytes(), &created); err != nil || created.Schema != "ardents-admission-authority-v1" ||
		created.RecordID == "" || created.IDCommitment == "" || len(created.AuthorityPublic) != 64 || bytes.Contains(createdOutput.Bytes(), password) {
		t.Fatalf("admission authority receipt = %+v / %v", created, err)
	}
	var authority [32]byte
	decoded, err := hex.DecodeString(created.AuthorityPublic)
	if err != nil || len(decoded) != len(authority) {
		t.Fatal(err)
	}
	copy(authority[:], decoded)
	now := time.Now().UTC().Truncate(time.Hour)
	request, holder, err := admission.PreparePermissionRequest(authority, network, [32]byte{24}, 25, admission.AllocationUser,
		now, [3]uint32{16, 0, 0})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		for index := range holder {
			holder[index] = 0
		}
	}()
	requestRaw, err := admission.EncodePermissionRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	requestPath, permissionPath := filepath.Join(t.TempDir(), "allocation.request"), filepath.Join(t.TempDir(), "permission.bin")
	if err := os.WriteFile(requestPath, requestRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	issueArguments := []string{"issue-admission-permission", "-vault-root", vaultRoot, "-record", created.RecordID,
		"-request", requestPath, "-permission-output", permissionPath,
		"-environment-commitment", hex.EncodeToString(environment[:]), "-network-commitment", hex.EncodeToString(network[:]),
		"-root-commitment", hex.EncodeToString(authorityRoot[:]), "-kind", "admission", "-id-commitment", created.IDCommitment}
	var id [32]byte
	idBytes, err := hex.DecodeString(created.IDCommitment)
	if err != nil || len(idBytes) != len(id) {
		t.Fatal("invalid authority commitment")
	}
	copy(id[:], idBytes)
	currentGeneration := func() uint64 {
		t.Helper()
		vault, err := custody.Open(custody.VaultConfig{Root: vaultRoot})
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = vault.Close() }()
		verified, err := vault.Execute(t.Context(), custody.Operation{Kind: custody.OperationVerifyVaultRecord, RecordID: created.RecordID,
			Expected: custody.AuthorityBinding{Environment: environment, Network: network, Root: authorityRoot, Kind: custody.AuthorityAdmission, IDCommitment: id}},
			&sequenceCommandSecrets{values: [][]byte{password}})
		if err != nil {
			t.Fatal(err)
		}
		return verified.Authority.Generation
	}
	var failedOutput bytes.Buffer
	if err := run(t.Context(), issueArguments, &failedOutput, &sequenceCommandSecrets{values: [][]byte{password}, commitments: [][32]byte{{9}}}); err == nil {
		t.Fatal("unapproved request reached issuance")
	}
	if currentGeneration() != 1 {
		t.Fatal("wrong commitment consumed allocation")
	}
	// A conflicting private output fails after genuine encrypted allocation.
	// Retry must retain that debit and produce the same permission, not refund it.
	if err := os.WriteFile(permissionPath, make([]byte, 228), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run(t.Context(), issueArguments, &failedOutput, &sequenceCommandSecrets{values: [][]byte{password}, commitments: [][32]byte{sha256.Sum256(requestRaw)}}); err == nil {
		t.Fatal("conflicting output accepted")
	}
	if currentGeneration() != 2 {
		t.Fatal("output failure lost or duplicated durable allocation")
	}
	if failedOutput.Len() != 0 {
		t.Fatal("failed issuance leaked a public success receipt")
	}
	if err := os.Remove(permissionPath); err != nil {
		t.Fatal(err)
	}
	var issuedOutput bytes.Buffer
	if err := run(t.Context(), issueArguments, &issuedOutput, &sequenceCommandSecrets{values: [][]byte{password}, commitments: [][32]byte{sha256.Sum256(requestRaw)}}); err != nil {
		t.Fatalf("issue admission permission: %v", err)
	}
	permission, err := os.ReadFile(permissionPath)
	if err != nil || len(permission) != 228 || bytes.Contains(issuedOutput.Bytes(), permission) || bytes.Contains(issuedOutput.Bytes(), requestRaw) {
		t.Fatalf("private permission output = %d bytes, public receipt %q, %v", len(permission), issuedOutput.String(), err)
	}
	if _, err := admission.DecodePermission(permission); err != nil {
		t.Fatalf("decode private permission: %v", err)
	}
	if currentGeneration() != 2 {
		t.Fatal("exact retry after output failure charged another allocation")
	}
	var repeatedOutput bytes.Buffer
	if err := run(t.Context(), issueArguments, &repeatedOutput, &sequenceCommandSecrets{values: [][]byte{password}, commitments: [][32]byte{sha256.Sum256(requestRaw)}}); err != nil {
		t.Fatal(err)
	}
	repeated, err := os.ReadFile(permissionPath)
	if err != nil || !bytes.Equal(permission, repeated) || currentGeneration() != 2 {
		t.Fatal("stable output retry changed bytes or debit")
	}

}
