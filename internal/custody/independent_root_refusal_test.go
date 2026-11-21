package custody

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAuthorityBirthRejectsForeignRootBeforeSecretsOrProvenance(t *testing.T) {
	for _, kind := range []AuthorityKind{AuthorityService, AuthorityAdmission} {
		t.Run(string(kind), func(t *testing.T) {
			root := t.TempDir()
			foreign := filepath.Join(root, "another-owner.state")
			if err := os.WriteFile(foreign, []byte("retained foreign owner evidence"), 0600); err != nil {
				t.Fatal(err)
			}
			vault, err := Open(VaultConfig{Root: root})
			if err != nil {
				t.Fatal(err)
			}
			defer vault.Close()
			operation := Operation{Kind: OperationCreateServiceAuthority, Binding: AuthorityBinding{
				Kind: kind, Environment: [32]byte{1}, Network: [32]byte{2}, Root: [32]byte{3}}}
			if kind == AuthorityAdmission {
				operation.Kind = OperationCreateAdmissionAuthority
			}
			input := &sequenceSecrets{values: [][]byte{[]byte("independent root test password"), []byte("independent root test password")}}
			receipt, err := vault.Execute(t.Context(), operation, input)
			if !errors.Is(err, ErrInvalid) || receipt.RecordID != "" || len(input.values) != 2 {
				t.Fatal("foreign root reached authority creation or secret input", err)
			}
			for _, path := range []string{filepath.Join(root, "independent-root"), vault.floors} {
				if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
					t.Fatal("foreign root gained authority history", err)
				}
			}
			entries, err := os.ReadDir(vault.records)
			if err != nil || len(entries) != 0 {
				t.Fatal("foreign root gained an encrypted Authority", err)
			}
			retained, err := os.ReadFile(foreign)
			if err != nil || string(retained) != "retained foreign owner evidence" {
				t.Fatal("foreign evidence changed", err)
			}
		})
	}
}
