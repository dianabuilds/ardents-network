//go:build linux

package selection

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
)

func TestInteriorBorrowRefusesWithoutChangingRetainedRoot(t *testing.T) {
	for _, scenario := range []string{"foreign", "old-marker", "marker-only", "missing-lock", "missing-floor", "bad-floor", "wrong-network", "wrong-domain"} {
		t.Run(scenario, func(t *testing.T) {
			base := t.TempDir()
			root := filepath.Join(base, "interior")
			if err := os.Mkdir(root, 0700); err != nil {
				t.Fatal(err)
			}
			write := func(name, value string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(root, name), []byte(value), 0600); err != nil {
					t.Fatal(err)
				}
			}
			switch scenario {
			case "foreign":
				write("legacy-evidence", "independent retained bytes\n")
			case "old-marker":
				write(".ardents-interior-set-v0", "ardents-interior-set-v0\n")
			case "marker-only":
				write(interiorMarker, "ardents-interior-set-v1\n")
			default:
				networkID, domain := byte(1), uint8(3)
				if scenario == "wrong-network" {
					networkID = 2
				}
				if scenario == "wrong-domain" {
					domain = 4
				}
				owner, err := openInteriorStore(root, [32]byte{networkID}, domain)
				if err != nil {
					t.Fatal(err)
				}
				if err := owner.close(); err != nil {
					t.Fatal(err)
				}
				if scenario == "missing-lock" || scenario == "missing-floor" {
					name := rootLockName
					if scenario == "missing-floor" {
						name = "watermark"
					}
					if err := os.Remove(filepath.Join(root, name)); err != nil {
						t.Fatal(err)
					}
				}
				if scenario == "bad-floor" {
					write("watermark", "retained corrupt floor\n")
				}
			}
			view := installationView(t, 1)
			installation, err := OpenInstallation(InstallationConfig{EntryRoot: filepath.Join(base, "entry"), Current: func() (network.RuntimeView, error) { return view, nil }})
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := installation.Close(); err != nil {
					t.Error(err)
				}
			}()
			before := interiorTree(t, root)
			for attempt := 0; attempt < 2; attempt++ {
				owner, err := installation.Borrow(RoleConfig{InteriorRoot: root, Domain: 3})
				if err == nil {
					_ = owner.Close()
					t.Fatal("invalid retained root accepted")
				}
				after := interiorTree(t, root)
				if len(before) != len(after) {
					t.Fatalf("refusal changed inventory: before %d, after %d", len(before), len(after))
				}
				for name, prior := range before {
					next, exists := after[name]
					if !exists || prior.bytes != next.bytes || !os.SameFile(prior.info, next.info) || prior.info.Mode() != next.info.Mode() || prior.info.Size() != next.info.Size() || !prior.info.ModTime().Equal(next.info.ModTime()) {
						t.Fatalf("refusal changed retained member %s", name)
					}
				}
			}
		})
	}
}

type interiorTreeMember struct {
	info  fs.FileInfo
	bytes string
}

func interiorTree(t *testing.T, root string) map[string]interiorTreeMember {
	t.Helper()
	result := make(map[string]interiorTreeMember)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		var raw []byte
		if info.Mode().IsRegular() {
			raw, err = os.ReadFile(path)
			if err != nil {
				return err
			}
		}
		result[path] = interiorTreeMember{info: info, bytes: string(raw)}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}
