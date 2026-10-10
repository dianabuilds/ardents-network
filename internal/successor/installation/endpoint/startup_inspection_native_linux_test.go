//go:build installation_native

package endpoint

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallationNativeStartupRefusesRootBeforeInstallationEffects(t *testing.T) {
	directory := filepath.Join(nativeRequestDirectory(t), "absent-installation")
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
