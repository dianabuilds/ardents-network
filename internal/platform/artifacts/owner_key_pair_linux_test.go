//go:build linux

package artifacts

import (
	"bytes"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestOwnerPrivateKeyReadRefusesNonPrivateSubstitutions(t *testing.T) {
	base := t.TempDir()
	key := filepath.Join(base, "key")
	body := []byte("private fixture")
	if err := os.WriteFile(key, body, 0600); err != nil {
		t.Fatal(err)
	}
	raw, err := readOwnerPrivateKey(key)
	if err != nil || !bytes.Equal(raw, body) {
		t.Fatalf("opened key did not preserve bytes: %v", err)
	}
	clear(raw)

	link := filepath.Join(base, "alias")
	if err := os.Symlink(key, link); err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(base, "fifo")
	if err := syscall.Mkfifo(fifo, 0600); err != nil {
		t.Fatal(err)
	}
	groupKey := filepath.Join(base, "group-key")
	if err := os.WriteFile(groupKey, body, 0640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(groupKey, 0640); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{link, fifo, groupKey} {
		if raw, err := readOwnerPrivateKey(path); err == nil || len(raw) != 0 {
			clear(raw)
			t.Fatalf("private key accepted %s", filepath.Base(path))
		}
	}
}
