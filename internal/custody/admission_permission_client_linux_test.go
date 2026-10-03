//go:build linux

package custody

import (
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/admission"
	"github.com/dianabuilds/ardents-network/internal/admission/allocation"
	"testing"
	"time"
)

func TestIssueAdmissionPermissionRejectsWallClockRollback(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	vault, err := Open(VaultConfig{Root: t.TempDir(), Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	binding := AuthorityBinding{Environment: [32]byte{1}, Network: [32]byte{2}, Root: [32]byte{3}, Kind: AuthorityAdmission}
	password := []byte("admission rollback custody password")
	created, err := vault.Execute(t.Context(), Operation{Kind: OperationCreateAdmissionAuthority,
		Authority: AuthorityState{Binding: binding}}, &sequenceSecrets{values: [][]byte{password, password}})
	if err != nil {
		t.Fatalf("create admission authority: %v", err)
	}
	issue := func(at time.Time, maxima [3]uint32) error {
		request, holder, requestErr := admission.PreparePermissionRequest(created.AdmissionAuthority.Public, binding.Network, [32]byte{4}, 5,
			admission.AllocationUser, at, maxima)
		if requestErr != nil {
			return requestErr
		}
		defer zero(holder)
		raw, requestErr := admission.EncodePermissionRequest(request)
		if requestErr != nil {
			return requestErr
		}
		_, requestErr = vault.Execute(t.Context(), Operation{Kind: OperationIssueAdmissionPermission, RecordID: created.RecordID,
			Expected: created.Authority.Binding, AdmissionRequest: raw, AdmissionRequestCommitment: sha256.Sum256(raw)}, &sequenceSecrets{values: [][]byte{password}})
		return requestErr
	}
	if err := issue(now, [3]uint32{uint32(allocation.RoleLimit(admission.AllocationUser)), 0, 0}); err != nil {
		t.Fatalf("consume current hourly allocation: %v", err)
	}
	now = now.Add(-time.Hour)
	if err := issue(now, [3]uint32{1, 0, 0}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("clock rollback issuance = %v, want invalid", err)
	}
	now = now.Add(time.Hour)
	if err := issue(now, [3]uint32{1, 0, 0}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("recovered hour reallocated consumed budget: %v", err)
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
	request, holder, err := admission.PreparePermissionRequest(created.AdmissionAuthority.Public, binding.Network, [32]byte{4}, 5,
		admission.AllocationUser, now, [3]uint32{32, 0, 16})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for index := range holder {
			holder[index] = 0
		}
	})
	raw, err := admission.EncodePermissionRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	issue := Operation{Kind: OperationIssueAdmissionPermission, RecordID: created.RecordID, Expected: created.Authority.Binding,
		AdmissionRequest: raw, AdmissionRequestCommitment: sha256.Sum256(raw)}
	first, err := vault.Execute(t.Context(), issue, &sequenceSecrets{values: [][]byte{password}})
	if err != nil || first.RecordID != created.RecordID || len(first.AdmissionPermission) == 0 {
		t.Fatalf("issue admission permission = %+v / %v", first, err)
	}
	verified, err := vault.Execute(t.Context(), Operation{Kind: OperationVerifyVaultRecord, RecordID: created.RecordID,
		Expected: created.Authority.Binding}, &sequenceSecrets{values: [][]byte{password}})
	if err != nil || verified.Authority.Generation != 2 || verified.Authority.Revision != 1 {
		t.Fatalf("verify current admission ledger = %+v / %v", verified, err)
	}
	permission, err := admission.DecodePermission(first.AdmissionPermission)
	if err != nil || admission.VerifyPermission(permission, ed25519.PublicKey(created.AdmissionAuthority.Public[:]), binding.Network, request.Permission.IssuerNodeID, request.Permission.DutyGeneration, now) != nil {
		t.Fatalf("verify signed admission permission = %+v / %v", permission, err)
	}
	retry, err := vault.Execute(t.Context(), issue, &sequenceSecrets{values: [][]byte{password}})
	if err != nil || retry.RecordID != first.RecordID || string(retry.AdmissionPermission) != string(first.AdmissionPermission) {
		t.Fatalf("exact retry = %+v / %v", retry, err)
	}
	ledgerPath, err := admissionLedgerPath(vault.root, created.RecordID)
	if err != nil {
		t.Fatal(err)
	}
	previousLedger, err := readEnvelopeFile(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	secondRequest, secondHolder, err := admission.PreparePermissionRequest(created.AdmissionAuthority.Public, binding.Network, [32]byte{4}, 5,
		admission.AllocationPublisher, now, [3]uint32{0, 16, 0})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { zero(secondHolder) })
	secondRaw, err := admission.EncodePermissionRequest(secondRequest)
	if err != nil {
		t.Fatal(err)
	}
	secondIssue := Operation{Kind: OperationIssueAdmissionPermission, RecordID: created.RecordID, Expected: created.Authority.Binding,
		AdmissionRequest: secondRaw, AdmissionRequestCommitment: sha256.Sum256(secondRaw)}
	if _, err := vault.Execute(t.Context(), secondIssue, &sequenceSecrets{values: [][]byte{password}}); err != nil {
		t.Fatalf("advance admission ledger: %v", err)
	}
	changed := request
	changed.Permission.Maxima[0]++
	changed, err = admission.SealPermissionRequest(changed, holder)
	if err != nil {
		t.Fatal(err)
	}
	changedRaw, err := admission.EncodePermissionRequest(changed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := vault.Execute(t.Context(), Operation{Kind: OperationIssueAdmissionPermission, RecordID: first.RecordID,
		Expected: first.Authority.Binding, AdmissionRequest: changedRaw, AdmissionRequestCommitment: sha256.Sum256(changedRaw)},
		&sequenceSecrets{values: [][]byte{password}}); err == nil {
		t.Fatal("accepted changed allocation for one permission ID")
	}
	if err := writeAtomicPrivate(ledgerPath, previousLedger); err != nil {
		t.Fatal(err)
	}
	if _, err := vault.Execute(t.Context(), secondIssue, &sequenceSecrets{values: [][]byte{password}}); err == nil {
		t.Fatal("accepted restored allocation ledger behind durable floor")
	}

}
