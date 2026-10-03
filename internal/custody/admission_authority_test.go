package custody

import (
	"crypto/ed25519"
	"crypto/sha256"
	"testing"
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
