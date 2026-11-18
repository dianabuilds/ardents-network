package custody

import "testing"

// countDerivations installs a counting wrapper around the envelope KDF for the
// duration of the test and restores the production derivation on cleanup. The
// wrapper still performs the real argon2id derivation, so receipts and signatures
// are produced exactly as in production (audit DD-5).
func countDerivations(t *testing.T) *int {
	t.Helper()
	real := deriveKey
	count := 0
	deriveKey = func(password, salt []byte) []byte {
		count++
		return real(password, salt)
	}
	t.Cleanup(func() { deriveKey = real })
	return &count
}

// TestAdmissionIssueReusesLedgerDerivation proves that a second allocation does
// not re-derive the ledger key: openCurrentAdmissionAuthority already decrypted
// and validated the current ledger within the same locked operation, and
// writeAdmissionSuccessor reuses it. Before the fix the issue path derived four
// keys (base, ledger, redundant ledger re-open, successor seal); after it derives
// three (base, ledger, successor seal).
func TestAdmissionIssueReusesLedgerDerivation(t *testing.T) {
	fixture := newAllocationVaultFixture(t)
	if _, err := fixture.vault.Execute(t.Context(), fixture.request(t, 1), &sequenceSecrets{values: [][]byte{fixture.password}}); err != nil {
		t.Fatal(err)
	}
	count := countDerivations(t)
	receipt, err := fixture.vault.Execute(t.Context(), fixture.request(t, 2), &sequenceSecrets{values: [][]byte{fixture.password}})
	if err != nil {
		t.Fatal(err)
	}
	if *count != 3 {
		t.Fatalf("second admission issue derived %d keys, want 3 (base + ledger + successor seal)", *count)
	}
	if receipt.State != RecordActive || receipt.Authority.Generation != 3 || receipt.AdmissionPermission == nil {
		t.Fatalf("second admission issue receipt = %+v", receipt)
	}
}

// TestAdmissionVerifyReusesBaseDerivation proves the active admission verify path
// derives two keys (base + ledger), not three: verifyRecord reuses the base
// plaintext it already opened instead of a second base KDF, and the ledger is
// opened once. This guards the already-accepted base reuse against regression.
func TestAdmissionVerifyReusesBaseDerivation(t *testing.T) {
	fixture := newAllocationVaultFixture(t)
	if _, err := fixture.vault.Execute(t.Context(), fixture.request(t, 1), &sequenceSecrets{values: [][]byte{fixture.password}}); err != nil {
		t.Fatal(err)
	}
	count := countDerivations(t)
	receipt, err := fixture.vault.Execute(t.Context(), Operation{Kind: OperationVerifyVaultRecord,
		RecordID: fixture.created.RecordID, Expected: fixture.created.Authority.Binding}, &sequenceSecrets{values: [][]byte{fixture.password}})
	if err != nil {
		t.Fatal(err)
	}
	if *count != 2 {
		t.Fatalf("active admission verify derived %d keys, want 2 (base + ledger)", *count)
	}
	if receipt.State != RecordActive || receipt.Authority.Generation != 2 {
		t.Fatalf("active admission verify receipt = %+v", receipt)
	}
}
