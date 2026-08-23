package update

import (
	"context"
	"crypto/sha256"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/custody"
)

func oracleTreeSum(root string) [32]byte {
	hash := sha256.New()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		_, _ = hash.Write([]byte(filepath.ToSlash(relative)))
		if entry.IsDir() {
			_, _ = hash.Write([]byte{0})
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_, _ = hash.Write([]byte{1})
		_, _ = hash.Write(body)
		return nil
	})
	if err != nil {
		return [32]byte{}
	}
	var result [32]byte
	copy(result[:], hash.Sum(nil))
	return result
}

func oracleCreateCustodyVault(t *testing.T, root string) {
	t.Helper()
	vault, err := custody.Open(custody.VaultConfig{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	password := []byte("update D0 custody vault password")
	state := oracleCustodyAuthorityState()
	if _, err := vault.Execute(t.Context(), custody.Operation{Kind: custody.OperationCreateVaultRecord, Authority: state}, &oracleCustodySecrets{values: [][]byte{password, password}}); err != nil {
		t.Fatal(err)
	}
	if err := vault.Close(); err != nil {
		t.Fatal(err)
	}
}

func oracleCustodyAuthorityState() custody.AuthorityState {
	var binding custody.AuthorityBinding
	for index := range binding.Environment {
		binding.Environment[index] = byte(index + 1)
		binding.Network[index] = byte(index + 2)
		binding.Root[index] = byte(index + 3)
		binding.IDCommitment[index] = byte(index + 4)
	}
	binding.Kind = custody.AuthorityService
	return custody.AuthorityState{Binding: binding, RootMaterial: []byte("update D0 authority root material"), Generation: 3, Revision: 7, Watermarks: []custody.Watermark{{Domain: "credential-generation", Value: 3}}}
}

type oracleCustodySecrets struct {
	values [][]byte
}

func (input *oracleCustodySecrets) ReadSecret(context.Context, custody.SecretPrompt) ([]byte, error) {
	if len(input.values) == 0 {
		return nil, errors.New("unexpected custody secret read")
	}
	value := append([]byte(nil), input.values[0]...)
	input.values = input.values[1:]
	return value, nil
}
