package custody

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestServiceRecordWrongPasswordAndAuthenticatedMutationHaveOneRefusal(t *testing.T) {
	f := newServiceVaultFixture(t)
	op := Operation{Kind: OperationVerifyVaultRecord, RecordID: f.created.RecordID, Expected: f.created.Authority.Binding}
	if _, err := f.vault.Execute(t.Context(), op, &sequenceSecrets{values: [][]byte{[]byte("wrong but bounded vault password")}}); !errors.Is(err, ErrUnlockFailed) {
		t.Fatal("wrong password exposed another result", err)
	}
	path := filepath.Join(f.vault.records, "record-"+f.created.RecordID+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var parsed envelope
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatal(err)
	}
	ciphertext, err := decodeRawURL(parsed.Ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	ciphertext[0] ^= 1
	parsed.Ciphertext = base64.RawURLEncoding.EncodeToString(ciphertext)
	zero(ciphertext)
	altered, err := marshalCanonical(parsed)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, altered, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := f.vault.Execute(t.Context(), op, &sequenceSecrets{values: [][]byte{f.password}}); !errors.Is(err, ErrUnlockFailed) {
		t.Fatal("ciphertext mutation exposed another result", err)
	}
}

func TestServiceRecordCrossHandleOperationRetainsExclusiveLease(t *testing.T) {
	f := newServiceVaultFixture(t)
	other, err := Open(VaultConfig{Root: f.root})
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	lease, err := acquireVaultOperationLock(f.root)
	if err != nil {
		t.Fatal(err)
	}
	op := Operation{Kind: OperationVerifyVaultRecord, RecordID: f.created.RecordID, Expected: f.created.Authority.Binding}
	input := &sequenceSecrets{values: [][]byte{f.password}}
	_, operationErr := other.Execute(t.Context(), op, input)
	if err := lease.release(); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(operationErr, ErrBusy) || len(input.values) != 1 {
		t.Fatal("cross-handle lease reached unlock", operationErr)
	}
}

func TestCustodyNonRegularAndUncertainRecordInputsCannotAcknowledge(t *testing.T) {
	f := newServiceVaultFixture(t)
	if _, err := readEnvelopeFile(f.root); !errors.Is(err, ErrInvalid) {
		t.Fatal("directory envelope", err)
	}
	if _, err := readSmallFile(f.root); !errors.Is(err, ErrInvalid) {
		t.Fatal("directory floor", err)
	}
	floor, err := os.ReadFile(f.vault.floors)
	if err != nil {
		t.Fatal(err)
	}
	const record = "00112233445566778899aabbccddeeff"
	_, finish, err := f.vault.begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := finish(f.vault.writeRecord(record, []byte("not a canonical envelope"))); !errors.Is(err, ErrInvalid) {
		t.Fatal("uncertain invalid record acknowledged", err)
	}
	persisted, err := os.ReadFile(filepath.Join(f.vault.records, "record-"+record+".json"))
	if err != nil || !bytes.Equal(persisted, []byte("not a canonical envelope")) {
		t.Fatal("control did not reach original native publication/readback", err)
	}
	// Uncertain publication remains evidence; neither it nor refusal can advance
	// the floor or become a valid record. Do not delete uncertain history.
	if _, err := f.vault.Execute(t.Context(), Operation{Kind: OperationVerifyVaultRecord, RecordID: record, Expected: f.created.Authority.Binding}, &sequenceSecrets{values: [][]byte{f.password}}); err == nil {
		t.Fatal("invalid record verified")
	}
	after, err := os.ReadFile(f.vault.floors)
	if err != nil || !bytes.Equal(floor, after) {
		t.Fatal("uncertain record advanced floor", err)
	}
}
