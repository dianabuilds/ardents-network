//go:build network_signing_native

package infrastructure

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSourceNamespaceNativeKeepsOriginalDirectoryAndAbsence(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Fatal("selected Linux owner profile requires root fixture ownership")
	}
	parent := t.TempDir()
	path := filepath.Join(parent, "units")
	if err := os.Mkdir(path, 0755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	original, err := openSourceNamespace(ctx, []string{path})
	if err != nil {
		t.Fatal(err)
	}
	defer original.close()
	if err := original.check(); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(parent, "original")
	if err := os.Rename(path, old); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0755); err != nil {
		t.Fatal(err)
	}
	refused := original.check()
	if !errors.Is(refused, ErrBinding) {
		t.Fatalf("replacement unit directory accepted: %v", refused)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(old, path); err != nil {
		t.Fatal(err)
	}
	if original.check() != refused {
		t.Fatal("restoration revived original namespace")
	}
	if err := original.close(); !errors.Is(err, ErrBinding) {
		t.Fatalf("Close lost original refusal: %v", err)
	}
}

func TestSourceNamespaceNativeNewOverrideAndCallerCancellationRefuse(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Fatal("selected Linux owner profile requires root fixture ownership")
	}
	for _, leaf := range []string{"ardents-source.service", "ardents-source.service.d", "ardents-source.socket", "ardents-.service.d", "service.d", "socket.d"} {
		t.Run(leaf, func(t *testing.T) {
			directory := t.TempDir()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			original, err := openSourceNamespace(ctx, []string{directory})
			if err != nil {
				t.Fatal(err)
			}
			defer original.close()
			if err := os.WriteFile(filepath.Join(directory, leaf), []byte("foreign override"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := original.check(); !errors.Is(err, ErrBinding) {
				t.Fatalf("new override accepted: %v", err)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if original, err := openSourceNamespace(ctx, []string{t.TempDir()}); original != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled constructor returned custody: %v %v", original, err)
	}
}
