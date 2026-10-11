package custody

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

type advancingSecret struct {
	password []byte
	advance  func()
}

func (input advancingSecret) ReadSecret(context.Context, SecretPrompt) ([]byte, error) {
	input.advance()
	return append([]byte(nil), input.password...), nil
}
func (advancingSecret) Confirm(context.Context, ConfirmationPrompt) (bool, error) {
	return false, ErrInvalid
}

func TestAdmissionHourExpiryAfterUnlockAndEncryptionCreatesNoIssuance(t *testing.T) {
	for _, stage := range []string{"unlock", "open", "seal"} {
		t.Run(stage, func(t *testing.T) {
			fixture := newAllocationVaultFixture(t)
			now := fixture.now
			calls := 0
			fixture.vault.now = func() time.Time {
				calls++
				if stage == "open" && calls == 3 || stage == "seal" && calls == 4 {
					fixture.now = now.Add(time.Hour)
				}
				return fixture.now
			}
			request := fixture.request(t, 1)
			receipt, err := fixture.vault.Execute(t.Context(), request, advancingSecret{fixture.password, func() {
				if stage == "unlock" {
					fixture.now = now.Add(time.Hour)
				}
			}})
			if !errors.Is(err, ErrInvalid) || len(receipt.AdmissionPermission) != 0 {
				t.Fatalf("expiry returned issuance: %+v / %v", receipt, err)
			}
			path, err := admissionLedgerPath(fixture.vault.root, fixture.created.RecordID)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("expired first write: %v", err)
			}
			floors, err := fixture.vault.readFloors()
			floor, found := floorFor(floors, fixture.created.Authority.Binding)
			if err != nil || !found || floor.Generation != 1 || floor.Revision != 0 {
				t.Fatalf("expired floor advanced: %+v / %v", floor, err)
			}
		})
	}
}

func TestAdmissionExactRetryHasNoPostExpiryPrivilege(t *testing.T) {
	fixture := newAllocationVaultFixture(t)
	request := fixture.request(t, 1)
	if _, err := fixture.vault.Execute(t.Context(), request, &sequenceSecrets{values: [][]byte{fixture.password}}); err != nil {
		t.Fatal(err)
	}
	fixture.now = fixture.now.Add(time.Hour)
	if _, err := fixture.vault.Execute(t.Context(), request, &sequenceSecrets{}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expired retry unlocked or succeeded: %v", err)
	}
}
