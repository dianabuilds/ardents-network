package custody

import (
	"crypto/ed25519"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/service/publication"
)

func TestCreateServiceAuthorityReturnsPublicIdentityAndRetainsEncryptedRoot(t *testing.T) {
	vault, err := Open(VaultConfig{Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	var binding AuthorityBinding
	for index := range binding.Environment {
		binding.Environment[index] = byte(index + 1)
		binding.Network[index] = byte(index + 2)
		binding.Root[index] = byte(index + 3)
	}
	binding.Kind = AuthorityService
	password := []byte("service authority vault password")
	receipt, err := vault.Execute(t.Context(), Operation{
		Kind:      OperationCreateServiceAuthority,
		Authority: AuthorityState{Binding: binding},
	}, &sequenceSecrets{values: [][]byte{password, password}})
	if err != nil {
		t.Fatalf("create Service Authority: %v", err)
	}
	if receipt.Operation != OperationCreateServiceAuthority || receipt.RecordID == "" || receipt.State != RecordActive {
		t.Fatalf("creation receipt = %+v", receipt)
	}
	if receipt.Authority.Binding.Kind != AuthorityService || receipt.Authority.Binding.IDCommitment == [32]byte{} ||
		receipt.ServiceAuthority.Public == [ed25519.PublicKeySize]byte{} {
		t.Fatalf("public Authority receipt = %+v", receipt)
	}
	if receipt.ServiceAuthority.Target != publication.Target(receipt.ServiceAuthority.Public) {
		t.Fatal("Service Target was not derived from the created Authority")
	}
	verified, err := vault.Execute(t.Context(), Operation{Kind: OperationVerifyVaultRecord,
		RecordID: receipt.RecordID, Expected: receipt.Authority.Binding}, &sequenceSecrets{values: [][]byte{password}})
	if err != nil || verified.State != RecordActive {
		t.Fatalf("verify encrypted Service Authority: %+v / %v", verified, err)
	}
}
