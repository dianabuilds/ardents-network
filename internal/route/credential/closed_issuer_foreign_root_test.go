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
func TestClosedIssuerRefusesForeignMaterialBeforeLease(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{"issuer.pin": "ardents-issuer-key-material-v1\n", "issuer.lock": "", "issuer.keys": "retained"}
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

func TestClosedIssuerRefusesForeignPartialMaterialBeforeLease(t *testing.T) {
	for _, name := range []string{"issuer.keys", "issuer.lock", "issuer.pending"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, name)
			if e := os.WriteFile(path, []byte("partial"), 0600); e != nil {
				t.Fatal(e)
			}
			acquired := false
			_, _, err := openClosedIssuerRootWithLease(root, func(string) (issuerRootLease, error) { acquired = true; return issuerRootLease{}, nil })
			if err == nil || acquired {
				t.Fatal("foreign partial root claimed", err)
			}
			files, e := os.ReadDir(root)
			if e != nil || len(files) != 1 {
				t.Fatal("foreign effect", e)
			}
			raw, e := os.ReadFile(path)
			if e != nil || string(raw) != "partial" {
				t.Fatal("foreign bytes changed", e)
			}
		})
	}
}
