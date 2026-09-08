package custody

import (
	"crypto/ed25519"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

func TestCreateAdmissionAuthorityKeepsSigningKeyEncrypted(t *testing.T) {
	vault, err := Open(VaultConfig{Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	binding := AuthorityBinding{Environment: [32]byte{1}, Network: [32]byte{2}, Root: [32]byte{3}, Kind: AuthorityAdmission}
	password := []byte("closed admission authority password")
	receipt, err := vault.Execute(t.Context(), Operation{Kind: OperationCreateAdmissionAuthority,
		Authority: AuthorityState{Binding: binding}}, &sequenceSecrets{values: [][]byte{password, password}})
	if err != nil {
		t.Fatalf("create admission authority: %v", err)
	}
	if receipt.Operation != OperationCreateAdmissionAuthority || receipt.RecordID == "" || receipt.AdmissionAuthority.Public == [ed25519.PublicKeySize]byte{} ||
		receipt.Authority.Binding.Kind != AuthorityAdmission || receipt.Authority.Binding.IDCommitment != sha256.Sum256(receipt.AdmissionAuthority.Public[:]) {
		t.Fatalf("admission authority receipt = %+v", receipt)
	}
	verified, err := vault.Execute(t.Context(), Operation{Kind: OperationVerifyVaultRecord, RecordID: receipt.RecordID,
		Expected: receipt.Authority.Binding}, &sequenceSecrets{values: [][]byte{password}})
	if err != nil || verified.State != RecordActive {
		t.Fatalf("verify encrypted admission authority = %+v / %v", verified, err)
	}
}

func TestIssueAdmissionPermissionAdvancesEncryptedAllocationLedger(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	vault, err := Open(VaultConfig{Root: t.TempDir(), Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	binding := AuthorityBinding{Environment: [32]byte{1}, Network: [32]byte{2}, Root: [32]byte{3}, Kind: AuthorityAdmission}
	password := []byte("admission allocation custody password")
	created, err := vault.Execute(t.Context(), Operation{Kind: OperationCreateAdmissionAuthority,
		Authority: AuthorityState{Binding: binding}}, &sequenceSecrets{values: [][]byte{password, password}})
	if err != nil {
		t.Fatalf("create admission authority: %v", err)
	}
	request, holder, err := credential.PreparePermissionRequest(created.AdmissionAuthority.Public, binding.Network, [32]byte{4}, 5,
		credential.AllocationUser, now, [3]uint32{32, 0, 16})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for index := range holder {
			holder[index] = 0
		}
	})
	raw, err := credential.EncodePermissionRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	issue := Operation{Kind: OperationIssueAdmissionPermission, RecordID: created.RecordID, Expected: created.Authority.Binding,
		AdmissionRequest: raw, AdmissionRequestCommitment: sha256.Sum256(raw)}
	first, err := vault.Execute(t.Context(), issue, &sequenceSecrets{values: [][]byte{password}})
	if err != nil || first.RecordID == created.RecordID || len(first.AdmissionPermission) == 0 {
		t.Fatalf("issue admission permission = %+v / %v", first, err)
	}
	permission, err := credential.DecodePermission(first.AdmissionPermission)
	if err != nil || credential.VerifyPermission(permission, ed25519.PublicKey(created.AdmissionAuthority.Public[:]), binding.Network, request.Permission.IssuerNodeID, request.Permission.DutyGeneration, now) != nil {
		t.Fatalf("verify signed admission permission = %+v / %v", permission, err)
	}
	retry, err := vault.Execute(t.Context(), issue, &sequenceSecrets{values: [][]byte{password}})
	if err != nil || retry.RecordID != first.RecordID || string(retry.AdmissionPermission) != string(first.AdmissionPermission) {
		t.Fatalf("exact retry = %+v / %v", retry, err)
	}
	changed := request
	changed.Permission.Maxima[0]++
	changed, err = credential.SealPermissionRequest(changed, holder)
	if err != nil {
		t.Fatal(err)
	}
	changedRaw, err := credential.EncodePermissionRequest(changed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := vault.Execute(t.Context(), Operation{Kind: OperationIssueAdmissionPermission, RecordID: first.RecordID,
		Expected: first.Authority.Binding, AdmissionRequest: changedRaw, AdmissionRequestCommitment: sha256.Sum256(changedRaw)},
		&sequenceSecrets{values: [][]byte{password}}); err == nil {
		t.Fatal("accepted changed allocation for one permission ID")
	}
}
