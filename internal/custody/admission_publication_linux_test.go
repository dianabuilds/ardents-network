//go:build linux

package custody

import (
	"bytes"
	"os"
	"testing"
	"time"
)

func TestAdmissionInterruptedPublicationStates(t *testing.T) {
	for _, stage := range []string{"before_publication", "torn_before_readback", "missing_retained_journal"} {
		t.Run(stage, func(t *testing.T) {
			fixture := newAllocationVaultFixture(t)
			if _, err := fixture.vault.Execute(t.Context(), fixture.request(t, 1), &sequenceSecrets{values: [][]byte{fixture.password}}); err != nil {
				t.Fatal(err)
			}
			path, err := admissionLedgerPath(fixture.vault.root, fixture.created.RecordID)
			if err != nil {
				t.Fatal(err)
			}
			previousJournal, err := readEnvelopeFile(path)
			if err != nil {
				t.Fatal(err)
			}
			defer zero(previousJournal)
			previousFloor, err := readSmallFile(fixture.vault.floors)
			if err != nil {
				t.Fatal(err)
			}
			defer zero(previousFloor)
			request := fixture.request(t, 1)
			issued, err := fixture.vault.Execute(t.Context(), request, &sequenceSecrets{values: [][]byte{fixture.password}})
			if err != nil {
				t.Fatal(err)
			}
			if err := writeAtomicPrivate(fixture.vault.floors, previousFloor); err != nil {
				t.Fatal(err)
			}
			// Reconstruct the persisted interruption states, without claiming syscall
			// failpoint coverage or introducing a production storage override.
			switch stage {
			case "before_publication":
				if err := writeAtomicPrivate(path, previousJournal); err != nil {
					t.Fatal(err)
				}
			case "torn_before_readback":
				raw, err := readEnvelopeFile(path)
				if err != nil {
					t.Fatal(err)
				}
				defer zero(raw)
				if err := writeAtomicPrivate(path, raw[:len(raw)/2]); err != nil {
					t.Fatal(err)
				}
			case "missing_retained_journal":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			}
			cfg := VaultConfig{Root: fixture.vault.root, Now: func() time.Time { return fixture.now }}
			if err := fixture.vault.Close(); err != nil {
				t.Fatal(err)
			}
			fixture.vault, err = Open(cfg)
			if err != nil {
				t.Fatal(err)
			}
			retried, err := fixture.vault.Execute(t.Context(), request, &sequenceSecrets{values: [][]byte{fixture.password}})
			if stage == "before_publication" {
				if err != nil || !bytes.Equal(retried.AdmissionPermission, issued.AdmissionPermission) || retried.Authority.Generation != issued.Authority.Generation {
					t.Fatalf("uncommitted attempt could not retry exactly: %v", err)
				}
			} else {
				if err == nil || len(retried.AdmissionPermission) != 0 {
					t.Fatal("unverifiable retained allocation exposed a permission")
				}
				floor, readErr := readSmallFile(fixture.vault.floors)
				if readErr != nil || !bytes.Equal(floor, previousFloor) {
					t.Fatal("refusal changed committed floor")
				}
				zero(floor)
			}
		})
	}
}
