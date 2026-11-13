//go:build installation_native

package fixedfile

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallationNativeFixedFilePrivateBirthAndSecretErasure(t *testing.T) {
	directory, parent := mutationDirectory(t)
	path := filepath.Join(directory, "pending")
	secret := []byte("private original pending material\n")
	owned, err := Create(t.Context(), path, parent, secret, 0600, 0)
	if owned != nil {
		defer owned.Close()
	}
	if err != nil {
		t.Fatal(err)
	}
	birth := owned.Identity()
	retained := owned.body
	secret[0] = '!'
	info, err := owned.Commit()
	if err != nil || !os.SameFile(birth, info) || info.Mode() != 0600 {
		t.Fatal("private same-inode commit failed", err)
	}
	actual, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(actual, retained) {
		t.Fatal("input alias changed private bytes", err)
	}
	if err := owned.Close(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(retained, make([]byte, len(retained))) {
		t.Fatal("joined owner retained private body")
	}
	if _, err := Replace(t.Context(), path, parent, info, actual, actual, 0600, 0); err == nil {
		t.Fatal("private creation enabled complete-file replacement")
	}
	foreign, err := Create(t.Context(), filepath.Join(directory, "foreign-group"), parent, actual, 0600, 65534)
	if foreign != nil {
		defer foreign.Close()
	}
	if err == nil {
		t.Fatal("foreign group admitted to private root file")
	}
	if _, err := os.Lstat(filepath.Join(directory, "foreign-group")); !os.IsNotExist(err) {
		t.Fatal("refused group had birth effect", err)
	}
}
