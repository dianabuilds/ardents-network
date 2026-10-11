package custody

import (
	"bytes"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
)

func TestAdmissionRefusalCannotPublishOrChangeFloor(t *testing.T) {
	f := newAllocationVaultFixture(t)
	original := f.request(t, 1)
	floor, err := readSmallFile(f.vault.floors)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"purpose", "authority", "network", "holder", "commitment", "password", "oversized", "zero", "unsupported"} {
		t.Run(name, func(t *testing.T) {
			op := original
			op.AdmissionRequest = bytes.Clone(original.AdmissionRequest)
			password := f.password
			switch name {
			case "purpose":
				op.Expected.Kind = AuthorityName
			case "authority":
				op.Expected.IDCommitment[0] ^= 1
			case "network":
				op.Expected.Network[0] ^= 1
			case "holder":
				op.AdmissionRequest[len(op.AdmissionRequest)-1] ^= 1
				op.AdmissionRequestCommitment = sha256.Sum256(op.AdmissionRequest)
			case "commitment":
				op.AdmissionRequestCommitment[0] ^= 1
			case "password":
				password = []byte("a different custody password")
			case "oversized":
				op.AdmissionRequest = make([]byte, 513)
			case "zero":
				op = Operation{}
			case "unsupported":
				op.Kind = "sign"
			}
			r, err := f.vault.Execute(t.Context(), op, &sequenceSecrets{values: [][]byte{password}})
			if err == nil || len(r.AdmissionPermission) != 0 || r.RecordID != "" {
				t.Fatal("refusal exposed result")
			}
			after, err := readSmallFile(f.vault.floors)
			if err != nil || !bytes.Equal(after, floor) {
				t.Fatal("refusal changed floor")
			}
			ledger, _ := admissionLedgerPath(f.vault.root, f.created.RecordID)
			if _, err := os.Stat(ledger); !os.IsNotExist(err) {
				t.Fatal("refusal published successor")
			}
		})
	}
	// Existing encrypted records cannot acquire signing provenance by deleting
	// the new owner's independent-root marker or asking it to recreate one.
	if err := os.Remove(filepath.Join(f.vault.root, "independent-root")); err != nil {
		t.Fatal(err)
	}
	if _, err := f.vault.Execute(t.Context(), original, &sequenceSecrets{values: [][]byte{f.password}}); err == nil {
		t.Fatal("foreign root signed")
	}
	if err := f.vault.admitIndependentRoot(); err == nil {
		t.Fatal("foreign root adopted")
	}
}
