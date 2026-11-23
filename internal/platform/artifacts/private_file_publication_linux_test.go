package artifacts

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPrivateFilePublicationAndExactRetry(t *testing.T) {
	parent := t.TempDir()
	if err := os.Chmod(parent, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(parent, "inventory")
	raw := []byte("inventory")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := PublishPrivateFile(ctx, path, raw); !errors.Is(err, context.Canceled) {
		t.Fatal("canceled publication", err)
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("canceled publication left an output", err)
	}
	if err := PublishPrivateFile(t.Context(), path, raw); err != nil {
		t.Fatal(err)
	}
	first, err := os.Stat(path)
	if err != nil || first.Mode().Perm() != 0600 {
		t.Fatal("published file", err)
	}
	if err := PublishPrivateFile(t.Context(), path, raw); err != nil {
		t.Fatal("exact retry", err)
	}
	if err := PublishPrivateFile(t.Context(), path, []byte("different")); !errors.Is(err, ErrInput) {
		t.Fatal("different bytes accepted", err)
	}
	after, err := os.Stat(path)
	if err != nil || !os.SameFile(first, after) {
		t.Fatal("publication replaced the original", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(raw, got) {
		t.Fatal("publication changed the original bytes", err)
	}
	link := filepath.Join(parent, "link")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if err := PublishPrivateFile(t.Context(), link, raw); !errors.Is(err, ErrInput) {
		t.Fatal("symlink accepted", err)
	}
	if err := os.Link(path, link+"-hard"); err != nil {
		t.Fatal(err)
	}
	if err := PublishPrivateFile(t.Context(), path, raw); !errors.Is(err, ErrInput) {
		t.Fatal("hardlink accepted", err)
	}
}
