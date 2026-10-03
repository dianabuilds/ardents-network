//go:build linux

package custody

import (
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
)

type allocationVaultFixture struct {
	vault    *Vault
	created  Receipt
	password []byte
	now      time.Time
}

func newAllocationVaultFixture(t *testing.T) *allocationVaultFixture {
	t.Helper()
	fixture := &allocationVaultFixture{password: []byte("allocation scenario custody password"), now: time.Unix(1_800_000_000, 0).UTC()}
	vault, err := Open(VaultConfig{Root: t.TempDir(), Now: func() time.Time { return fixture.now }})
	if err != nil {
		t.Fatal(err)
	}
	fixture.vault = vault
	t.Cleanup(func() { _ = fixture.vault.Close(); zero(fixture.password) })
	binding := AuthorityBinding{Environment: [32]byte{1}, Network: [32]byte{2}, Root: [32]byte{3}, Kind: AuthorityAdmission}
	created, err := vault.Execute(t.Context(), Operation{Kind: OperationCreateAdmissionAuthority, Authority: AuthorityState{Binding: binding}}, &sequenceSecrets{values: [][]byte{fixture.password, fixture.password}})
	if err != nil {
		t.Fatal(err)
	}
	fixture.created = created
	return fixture
}

func (fixture *allocationVaultFixture) request(t *testing.T, count uint32) Operation {
	t.Helper()
	request, holder, err := admission.PreparePermissionRequest(fixture.created.AdmissionAuthority.Public, fixture.created.Authority.Binding.Network, [32]byte{4}, 5, admission.AllocationUser, fixture.now, [3]uint32{count, 0, 0})
	if err != nil {
		t.Fatal(err)
	}
	defer zero(holder)
	raw, err := admission.EncodePermissionRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	return Operation{Kind: OperationIssueAdmissionPermission, RecordID: fixture.created.RecordID, Expected: fixture.created.Authority.Binding, AdmissionRequest: raw, AdmissionRequestCommitment: sha256.Sum256(raw)}
}

func TestAdmissionConcurrentLastReservationHasOneWinner(t *testing.T) {
	fixture := newAllocationVaultFixture(t)
	if _, err := fixture.vault.Execute(t.Context(), fixture.request(t, 4095), &sequenceSecrets{values: [][]byte{fixture.password}}); err != nil {
		t.Fatal(err)
	}
	requests := []Operation{fixture.request(t, 1), fixture.request(t, 1)}
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, request := range requests {
		go func() {
			<-start
			_, err := fixture.vault.Execute(t.Context(), request, &sequenceSecrets{values: [][]byte{fixture.password}})
			results <- err
		}()
	}
	close(start)
	successes, refusals := 0, 0
	for range requests {
		err := <-results
		if err == nil {
			successes++
		} else if errors.Is(err, ErrInvalid) {
			refusals++
		} else {
			t.Fatalf("unexpected concurrent failure: %v", err)
		}
	}
	if successes != 1 || refusals != 1 {
		t.Fatalf("last reservation: successes=%d refusals=%d", successes, refusals)
	}
	receipt, err := fixture.vault.Execute(t.Context(), Operation{Kind: OperationVerifyVaultRecord, RecordID: fixture.created.RecordID, Expected: fixture.created.Authority.Binding}, &sequenceSecrets{values: [][]byte{fixture.password}})
	if err != nil || receipt.Authority.Generation != 3 || receipt.Authority.Revision != 2 {
		t.Fatalf("concurrent debit floor: %+v / %v", receipt.Authority, err)
	}
}

func TestAdmissionRecoveryRefusesSkippedSuccessor(t *testing.T) {
	fixture := newAllocationVaultFixture(t)
	originalFloor, err := readSmallFile(fixture.vault.floors)
	if err != nil {
		t.Fatal(err)
	}
	defer zero(originalFloor)
	first, second := fixture.request(t, 1), fixture.request(t, 1)
	for _, request := range []Operation{first, second} {
		if _, err := fixture.vault.Execute(t.Context(), request, &sequenceSecrets{values: [][]byte{fixture.password}}); err != nil {
			t.Fatal(err)
		}
	}
	// The encrypted journal is two generations beyond this valid earlier floor.
	// It is not the recoverable single interrupted publication state.
	if err := writeAtomicPrivate(fixture.vault.floors, originalFloor); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.vault.Execute(t.Context(), second, &sequenceSecrets{values: [][]byte{fixture.password}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("skipped successor accepted: %v", err)
	}
}
