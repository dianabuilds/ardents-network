package custody

import (
	"crypto/ed25519"
	"crypto/sha256"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
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

func TestAdmissionJournalAcceptsCompleteHourlyReservationSet(t *testing.T) {
	allocations := make([]admissionAllocation, 0, maximumUserAllocation+maximumPublisherAllocation)
	for index := uint64(0); index < maximumUserAllocation+maximumPublisherAllocation; index++ {
		role := credential.AllocationUser
		if index >= maximumUserAllocation {
			role = credential.AllocationPublisher
		}
		allocation := admissionAllocation{window: 1, role: role, tokens: 1}
		value := index + 1
		allocation.id[28] = byte(value >> 24)
		allocation.id[29] = byte(value >> 16)
		allocation.id[30] = byte(value >> 8)
		allocation.id[31] = byte(value)
		allocation.digest[0] = 1
		allocation.digest[28] = allocation.id[28]
		allocation.digest[29] = allocation.id[29]
		allocation.digest[30] = allocation.id[30]
		allocation.digest[31] = allocation.id[31]
		allocations = append(allocations, allocation)
	}
	raw, err := encodeAdmissionJournal(allocations)
	if err != nil {
		t.Fatalf("encode complete reservation set: %v", err)
	}
	decoded, err := decodeAdmissionJournal(raw)
	if err != nil || len(decoded) != len(allocations) {
		t.Fatalf("decode complete reservation set = %d / %v", len(decoded), err)
	}
}

func TestAdmissionAllocationsForWindowExpiresPreviousReservations(t *testing.T) {
	allocations := []admissionAllocation{
		{window: 100, role: credential.AllocationUser, tokens: 1, id: [32]byte{1}, digest: [32]byte{1}},
		{window: 101, role: credential.AllocationPublisher, tokens: 1, id: [32]byte{2}, digest: [32]byte{2}},
	}
	current := admissionAllocationsForWindow(allocations, 101)
	if len(current) != 1 || current[0].window != 101 || current[0].role != credential.AllocationPublisher {
		t.Fatalf("current hourly allocations = %#v", current)
	}
}
