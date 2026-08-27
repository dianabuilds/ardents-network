package custody

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPurgeRequiresConfirmationAndRetainsAuthorityFloor(t *testing.T) {
	vault, err := Open(VaultConfig{Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = vault.Close() })
	state := testAuthorityState()
	password := []byte("purge test vault password")
	created, err := vault.Execute(t.Context(), Operation{Kind: OperationCreateVaultRecord, Authority: state}, &sequenceSecrets{values: [][]byte{password, password}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := vault.Execute(t.Context(), Operation{Kind: OperationPurgeVaultRecord, RecordID: created.RecordID, Expected: state.Binding}, &sequenceSecrets{values: [][]byte{password}, confirmations: []bool{false}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unconfirmed purge error = %v", err)
	}
	path := filepath.Join(vault.records, "record-"+created.RecordID+".json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("unconfirmed purge removed record: %v", err)
	}
	if _, err := vault.Execute(t.Context(), Operation{Kind: OperationPurgeVaultRecord, RecordID: created.RecordID, Expected: state.Binding}, &sequenceSecrets{values: [][]byte{password}, confirmations: []bool{true}}); err != nil {
		t.Fatalf("confirmed purge: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("purged record remains: %v", err)
	}
	floors, err := vault.readFloors()
	if err != nil || len(floors) != 1 || floorBinding(floors[0]) != state.Binding {
		t.Fatalf("authority floor after purge = %+v, %v", floors, err)
	}
}
