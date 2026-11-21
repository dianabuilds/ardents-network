//go:build linux

package selection

import (
	"maps"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/network"
)

// These supplied-fact storage units establish no authenticated intake or Route.
func TestInstallationFirstPreparationAndRetainedOpening(t *testing.T) {
	view := installationView(t, 1)
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	config := InstallationConfig{EntryRoot: root, Current: func() (network.RuntimeView, error) { return view, nil }}
	if owner, err := OpenRetainedInstallation(config); err == nil {
		_ = owner.Close()
		t.Fatal("empty retained root initialized")
	}
	if err := InitializeInstallation(config); err != nil {
		t.Fatal(err)
	}
	owner, err := OpenRetainedInstallation(config)
	if err != nil {
		t.Fatal(err)
	}
	selected, err := owner.entries.Members(3)
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	before := entryHistoryFiles(t, root)
	if err := InitializeInstallation(config); err == nil {
		t.Fatal("initial preparation adopted history")
	}
	if !maps.Equal(before, entryHistoryFiles(t, root)) {
		t.Fatal("repeat initialization changed history")
	}
	owner, err = OpenRetainedInstallation(config)
	if err != nil {
		t.Fatal(err)
	}
	if retained, err := owner.entries.Members(3); err != nil || retained != selected {
		t.Fatal("retained opening changed selection", err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
}

func entryHistoryFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	files := make(map[string]string)
	for _, entry := range entries {
		raw, err := os.ReadFile(filepath.Join(root, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		files[entry.Name()] = string(raw)
	}
	return files
}

func TestRetainedInstallationRefusesLostHistoryWithoutMutation(t *testing.T) {
	for _, lost := range []string{"root", "contents", "lease", "marker", "watermark"} {
		t.Run(lost, func(t *testing.T) {
			view := installationView(t, 1)
			root := filepath.Join(t.TempDir(), "entry")
			if err := os.Mkdir(root, 0700); err != nil {
				t.Fatal(err)
			}
			config := InstallationConfig{EntryRoot: root, Current: func() (network.RuntimeView, error) { return view, nil }}
			if err := InitializeInstallation(config); err != nil {
				t.Fatal(err)
			}
			original, err := os.Stat(root)
			if err != nil {
				t.Fatal(err)
			}
			switch lost {
			case "root":
				err = os.RemoveAll(root)
			case "contents":
				for name := range entryHistoryFiles(t, root) {
					if err := os.Remove(filepath.Join(root, name)); err != nil {
						t.Fatal(err)
					}
				}
			case "lease":
				err = os.Remove(filepath.Join(root, rootLockName))
			case "marker":
				err = os.Remove(filepath.Join(root, closedSetMarker))
			case "watermark":
				err = os.Remove(filepath.Join(root, "watermark"))
			}
			if err != nil {
				t.Fatal(err)
			}
			before := entryHistoryFiles(t, root)
			if owner, err := OpenRetainedInstallation(config); err == nil {
				_ = owner.Close()
				t.Fatal("lost history accepted")
			}
			if !maps.Equal(before, entryHistoryFiles(t, root)) {
				t.Fatal("refusal modified history")
			}
			current, err := os.Stat(root)
			if lost == "root" {
				if !os.IsNotExist(err) {
					t.Fatal("refusal recreated root", err)
				}
			} else if err != nil || !os.SameFile(original, current) {
				t.Fatal("refusal replaced original root", err)
			}
		})
	}
}

func TestRetainedEntryContentsLossCannotResampleBeforeExpiry(t *testing.T) {
	config, view := closedEntryFixture(t)
	owner, err := openClosedSets(config)
	if err != nil {
		t.Fatal(err)
	}
	selected, err := owner.Members(1)
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	var remaining []ClosedSetMember
	for _, candidate := range view.Candidates {
		if candidate.NodeID != selected[0].NodeID && candidate.NodeID != selected[1].NodeID {
			remaining = append(remaining, candidate)
		}
	}
	view.Candidates = remaining
	openRetained := func() (*ClosedSets, error) {
		return openClosedSetsMode(config, closedSetRetained, acquireRetainedRootLease)
	}
	owner, err = openRetained()
	if err != nil {
		t.Fatal(err)
	}
	if retained, err := owner.Members(1); err != nil || retained != selected {
		t.Fatal("ordinary retained reopen resampled", err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	for name := range entryHistoryFiles(t, config.Root) {
		if err := os.Remove(filepath.Join(config.Root, name)); err != nil {
			t.Fatal(err)
		}
	}
	if owner, err := openRetained(); err == nil {
		_ = owner.Close()
		t.Fatal("erased history permitted another draw")
	}
	if len(entryHistoryFiles(t, config.Root)) != 0 {
		t.Fatal("refusal recreated selection history")
	}
}

func TestRetainedInstallationCannotRecreateRoleHistory(t *testing.T) {
	for _, lost := range []string{"root", "contents", "lease"} {
		t.Run(lost, func(t *testing.T) {
			view := installationView(t, 1)
			root := t.TempDir()
			entry, interior := filepath.Join(root, "entry"), filepath.Join(root, "interior")
			for _, path := range []string{entry, interior} {
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			config := InstallationConfig{EntryRoot: entry, Current: func() (network.RuntimeView, error) { return view, nil }}
			role := RoleConfig{InteriorRoot: interior, Domain: 3}
			if err := InitializeInstallation(config, role); err != nil {
				t.Fatal(err)
			}
			owner, err := OpenRetainedInstallation(config)
			if err != nil {
				t.Fatal(err)
			}
			borrower, err := owner.Borrow(role)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := borrower.Select(); err != nil {
				t.Fatal(err)
			}
			if err := borrower.Close(); err != nil {
				t.Fatal(err)
			}
			if err := owner.Close(); err != nil {
				t.Fatal(err)
			}
			switch lost {
			case "root":
				err = os.RemoveAll(interior)
			case "contents":
				for name := range entryHistoryFiles(t, interior) {
					if err := os.Remove(filepath.Join(interior, name)); err != nil {
						t.Fatal(err)
					}
				}
			case "lease":
				err = os.Remove(filepath.Join(interior, rootLockName))
			}
			if err != nil {
				t.Fatal(err)
			}
			before := entryHistoryFiles(t, interior)
			owner, err = OpenRetainedInstallation(config)
			if err != nil {
				t.Fatal("Entry history was not lost", err)
			}
			if borrower, err := owner.Borrow(role); err == nil {
				_ = borrower.Close()
				t.Fatal("lost role history accepted")
			}
			if err := owner.Close(); err != nil {
				t.Fatal(err)
			}
			if !maps.Equal(before, entryHistoryFiles(t, interior)) {
				t.Fatal("refusal recreated role history")
			}
		})
	}
}
