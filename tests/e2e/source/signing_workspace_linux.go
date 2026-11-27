//go:build linux && network_signing_native

package source

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"

	artifactio "github.com/dianabuilds/ardents-network/internal/platform/artifacts"
)

// SigningWorkspace creates a root-controlled temporary directory on ext4.
func SigningWorkspace(t *testing.T) string {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Fatal("invalid environment: Linux root required")
	}
	p := t.TempDir()
	if err := os.Chmod(p, 0700); err != nil {
		t.Fatal(err)
	}
	ancestors, err := artifactio.OpenDirectoryChain(t.Context(), p, nil)
	if err != nil {
		t.Fatal("invalid environment", err)
	}
	if err = errors.Join(ancestors.CheckRootAccess(), ancestors.Close()); err != nil {
		t.Fatal("invalid environment: root-controlled direct ancestors required", err)
	}
	mountType, err := exec.Command("findmnt", "--kernel", "--noheadings", "--output", "FSTYPE", "--target", p).Output()
	if err != nil || strings.TrimSpace(string(mountType)) != "ext4" {
		t.Fatal("invalid environment: native ext4 required", err)
	}
	return p
}
