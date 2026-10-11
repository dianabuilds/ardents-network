//go:build linux

package custody

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAuthorityBirthRejectsSymlinkedRecordDirectories(t *testing.T) {
	for _, directory := range []string{"records", "quarantine"} {
		t.Run(directory, func(t *testing.T) {
			root, foreign := t.TempDir(), t.TempDir()
			if err := os.Symlink(foreign, filepath.Join(root, directory)); err != nil {
				t.Fatal("selected native symlink environment", err)
			}
			vault, err := Open(VaultConfig{Root: root})
			if err != nil {
				t.Fatal(err)
			}
			defer vault.Close()
			input := &sequenceSecrets{values: [][]byte{[]byte("independent root test password"), []byte("independent root test password")}}
			_, err = vault.Execute(t.Context(), Operation{Kind: OperationCreateServiceAuthority,
				Binding: AuthorityBinding{Kind: AuthorityService, Environment: [32]byte{1}, Network: [32]byte{2}, Root: [32]byte{3}}}, input)
			if !errors.Is(err, ErrInvalid) || len(input.values) != 2 {
				t.Fatal("symlinked root reached authority creation or secrets", err)
			}
			if _, err := os.Lstat(filepath.Join(root, "independent-root")); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("symlinked root gained provenance", err)
			}
			entries, err := os.ReadDir(foreign)
			if err != nil || len(entries) != 0 {
				t.Fatal("foreign directory changed", err)
			}
		})
	}
}
