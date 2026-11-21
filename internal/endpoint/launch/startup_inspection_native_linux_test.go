//go:build installation_native

package launch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallationNativeStartupRefusesRootBeforeInstallationEffects(t *testing.T) {
	parent := os.Getenv("ARDENTS_INSTALLATION_NATIVE_ROOT")
	if os.Geteuid() != 0 || parent == "" {
		t.Fatal("invalid environment: installation_native requires root and an explicit trusted temporary parent")
	}
	if _, err := rootDirectoryAncestors(parent); err != nil {
		t.Fatalf("invalid environment: untrusted temporary parent: %v", err)
	}
	temporary, err := os.MkdirTemp(parent, "startup-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(temporary); err != nil {
			t.Error(err)
		}
	})
	directory := filepath.Join(temporary, "absent-installation")
	if owner, err := OpenStartup(t.Context(), directory); owner != nil || !errors.Is(err, ErrNativeUnavailable) {
		if owner != nil {
			owner.Close()
		}
		t.Fatal("public startup admitted root", err)
	}
	if owner, err := openStartupInspection(t.Context(), directory); owner != nil || !errors.Is(err, ErrNativeUnavailable) {
		if owner != nil {
			owner.close()
		}
		t.Fatal("root acquired service startup observations", err)
	}
	if _, err := os.Lstat(directory); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("startup refusal introduced filesystem effects", err)
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if owner, err := openStartupInspection(cancelled, directory); owner != nil || !errors.Is(err, context.Canceled) {
		if owner != nil {
			owner.close()
		}
		t.Fatal("startup renewed cancelled opening", err)
	}
	if owner, err := openStartupInspection(nil, directory); owner != nil || !errors.Is(err, ErrInput) {
		if owner != nil {
			owner.close()
		}
		t.Fatal("startup accepted missing caller", err)
	}
}
