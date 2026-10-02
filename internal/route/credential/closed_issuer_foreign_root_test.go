package credential

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClosedIssuerRefusesForeignAdmissionBeforeLease(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{"admission.pin": "ardents-admission-ledger-v1\n", "admission.lock": "", "admission.journal": "retained", "admission.floor": "floor"}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	acquired := false
	_, _, err := openClosedIssuerRootWithLease(root, func(string) (issuerRootLease, error) { acquired = true; return issuerRootLease{}, nil })
	if err == nil || acquired {
		t.Fatalf("refusal %v acquired %t", err, acquired)
	}
	entries, e := os.ReadDir(root)
	if e != nil || len(entries) != len(files) {
		t.Fatal("foreign root changed", e)
	}
	for name, body := range files {
		raw, e := os.ReadFile(filepath.Join(root, name))
		if e != nil || string(raw) != body {
			t.Fatal("foreign bytes changed", name, e)
		}
	}
}
