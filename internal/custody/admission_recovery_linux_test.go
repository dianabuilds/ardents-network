//go:build linux

package custody

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission"
)

// Reconstruct the exact on-disk interruption state from
// genuine encrypted issuance and floor snapshots, then reopen the real Vault.
func TestAdmissionRecoveryAfterPublication(t *testing.T) {
	for _, completed := range []int{0, 1, 3} {
		name := fmt.Sprintf("after_%d_committed", completed)
		t.Run(name, func(t *testing.T) {
			now := time.Unix(1_800_000_000, 0).UTC()
			cfg := VaultConfig{Root: t.TempDir(), Now: func() time.Time { return now }}
			vault, err := Open(cfg)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = vault.Close() }()
			password := []byte("review-only admission recovery password")
			binding := AuthorityBinding{Environment: [32]byte{1}, Network: [32]byte{2}, Root: [32]byte{3}, Kind: AuthorityAdmission}
			created, err := vault.Execute(t.Context(), Operation{Kind: OperationCreateAdmissionAuthority, Authority: AuthorityState{Binding: binding}}, &sequenceSecrets{values: [][]byte{password, password}})
			if err != nil {
				t.Fatal(err)
			}
			prepare := func() Operation {
				request, holder, err := admission.PreparePermissionRequest(created.AdmissionAuthority.Public, binding.Network, [32]byte{4}, 5, admission.AllocationUser, now, [3]uint32{1, 0, 0})
				if err != nil {
					t.Fatal(err)
				}
				defer zero(holder)
				raw, err := admission.EncodePermissionRequest(request)
				if err != nil {
					t.Fatal(err)
				}
				return Operation{Kind: OperationIssueAdmissionPermission, RecordID: created.RecordID, Expected: created.Authority.Binding, AdmissionRequest: raw, AdmissionRequestCommitment: sha256.Sum256(raw)}
			}
			for range completed {
				if _, err := vault.Execute(t.Context(), prepare(), &sequenceSecrets{values: [][]byte{password}}); err != nil {
					t.Fatal(err)
				}
			}
			previousFloors, err := readSmallFile(vault.floors)
			if err != nil {
				t.Fatal(err)
			}
			defer zero(previousFloors)
			request := prepare()
			issued, err := vault.Execute(t.Context(), request, &sequenceSecrets{values: [][]byte{password}})
			if err != nil {
				t.Fatal(err)
			}
			if issued.Authority.Generation != uint64(completed+2) {
				t.Fatal("unexpected baseline generation")
			}
			// Keep the real new encrypted envelope, but the last committed floor:
			// the same disk pair left by interruption before floor advancement.
			if err := writeAtomicPrivate(vault.floors, previousFloors); err != nil {
				t.Fatal(err)
			}
			if err := vault.Close(); err != nil {
				t.Fatal(err)
			}
			vault, err = Open(cfg)
			if err != nil {
				t.Fatal(err)
			}
			retried, err := vault.Execute(t.Context(), request, &sequenceSecrets{values: [][]byte{password}})
			if err != nil {
				t.Fatalf("exact next successor after %d completed allocations refused on reopen: %v", completed, err)
			}
			if !bytes.Equal(issued.AdmissionPermission, retried.AdmissionPermission) || retried.Authority.Generation != issued.Authority.Generation {
				t.Fatal("recovery changed signed result or charged another allocation")
			}
		})
	}
}
