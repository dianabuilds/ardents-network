//go:build linux && network_signing_native

package state

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// The Introduction repair root check binds the real root-owned
// /var/lib/ardents/roles/introduction parent chain, so its positive path runs
// under the selected root-native profile rather than the unprivileged unit
// container. The command supplies the complete ordered writable-root set and
// also sets Config.Root/Config.LocalRoleStateRoot to its first two entries; the
// adapter must validate that set without appending the State/local-role roots a
// second time (E1-1).
func TestIntroductionRepairRootsNative(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Fatal("selected Introduction repair root profile requires Linux root")
	}
	const roles = "/var/lib/ardents/roles"
	base := filepath.Join(roles, "introduction")
	baseExisted := true
	if _, err := os.Lstat(base); err != nil {
		baseExisted = false
		if err := os.MkdirAll(base, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(base, 0755); err != nil {
			t.Fatal(err)
		}
	}
	work := filepath.Join(base, "e11-native-"+strconv.Itoa(os.Getpid()))
	if err := os.MkdirAll(work, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(work, 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(work); err != nil {
			t.Error(err)
		}
		if !baseExisted {
			if err := os.Remove(base); err != nil {
				t.Error(err)
			}
		}
	})
	roots := make([]string, 5)
	for i, name := range []string{"network", "participation", "spend", "hosting", "introduction"} {
		root := filepath.Join(work, name)
		if err := os.MkdirAll(root, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(root, 0700); err != nil {
			t.Fatal(err)
		}
		roots[i] = root
	}
	// The exact command-derived shape: the full ordered set already contains the
	// State and local-role roots, so the check must proceed, not refuse.
	t.Run("command shape proceeds", func(t *testing.T) {
		if err := checkNodeDutyRepairRoots(introductionRepairNative, roots); err != nil {
			t.Fatal("complete ordered writable-root set refused:", err)
		}
	})

	// Causal anchor: the same real roots supplied without duplication proceed,
	// proving the roots themselves are valid and isolating the double-append as
	// the sole refusal cause on the unfixed adapter.
	t.Run("disjoint config roots proceed", func(t *testing.T) {
		if err := checkNodeDutyRepairRoots(introductionRepairNative, roots[2:]); err != nil {
			t.Fatal("disjoint root set refused:", err)
		}
	})

	t.Run("duplicate refuses", func(t *testing.T) {
		dup := append(append([]string(nil), roots...), roots[0])
		if err := checkNodeDutyRepairRoots(introductionRepairNative, dup); !errors.Is(err, ErrIntroductionRepairUnavailable) {
			t.Fatal("duplicate writable root accepted:", err)
		}
	})

	t.Run("nested refuses", func(t *testing.T) {
		nested := filepath.Join(roots[2], "sub")
		if err := os.MkdirAll(nested, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(nested, 0700); err != nil {
			t.Fatal(err)
		}
		set := []string{roots[0], roots[1], roots[2], nested, roots[4]}
		if err := checkNodeDutyRepairRoots(introductionRepairNative, set); !errors.Is(err, ErrIntroductionRepairUnavailable) {
			t.Fatal("nested writable root accepted:", err)
		}
	})
}
