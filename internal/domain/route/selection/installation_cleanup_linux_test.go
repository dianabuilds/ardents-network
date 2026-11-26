//go:build linux

package selection

// This control corrupts the actual Unix lease file descriptor and observes the
// joined cleanup failure. The Windows lease holds a syscall.Handle rather than
// an *os.File, so the physical corruption seam is platform-bound; a portable
// mock close failure would not exercise the native lease or prove that its
// cleanup provenance survives the borrower and installation terminal returns.

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/domain/network"
)

func TestInstallationRetainsBorrowerCleanupFailure(t *testing.T) {
	root := t.TempDir()
	view := installationView(t, 1)
	installation, err := OpenInstallation(InstallationConfig{EntryRoot: filepath.Join(root, "entry"), Current: func() (network.RuntimeView, error) { return view, nil }})
	if err != nil {
		t.Fatal(err)
	}
	borrower, err := installation.Borrow(RoleConfig{InteriorRoot: filepath.Join(root, "interior"), Domain: 3})
	if err != nil {
		t.Fatal(err)
	}
	// A physically broken lease exercises cleanup provenance; it supplies no
	// accepting authority or transport behavior.
	if err := borrower.interior.lease.file.Close(); err != nil {
		t.Fatal(err)
	}
	terminal := borrower.Close()
	if terminal == nil {
		t.Fatal("lost Interior physical cleanup failure")
	}
	if repeated := borrower.Close(); repeated != terminal {
		t.Fatal("borrower terminal result changed")
	}
	installationTerminal := installation.Close()
	if installationTerminal == nil {
		t.Fatal("installation lost returned borrower cleanup failure")
	}
	if !errors.Is(installationTerminal, terminal) {
		t.Fatalf("installation lost cleanup provenance: %v", installationTerminal)
	}
	if repeated := installation.Close(); repeated != installationTerminal {
		t.Fatal("installation terminal result changed")
	}
}
