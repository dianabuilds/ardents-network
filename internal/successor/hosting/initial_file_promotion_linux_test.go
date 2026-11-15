package hosting

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestInitialFilePromotionRetainsOriginalAndRejectsSubstitution(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Fatal("root-owned initial-file fixture requires root")
	}
	for _, scenario := range []string{"original", "replaced", "modified", "hardlink", "symlink", "unknown", "absent-birth"} {
		t.Run(scenario, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "budget.pin")
			if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
				t.Fatal(err)
			}
			original, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			births := FileBirths{originals: map[string]os.FileInfo{"budget.pin": original}}
			name := "budget.pin"
			switch scenario {
			case "replaced", "symlink":
				if err := os.Rename(path, filepath.Join(dir, "retained")); err != nil {
					t.Fatal(err)
				}
				if scenario == "replaced" {
					err = os.WriteFile(path, []byte("original"), 0600)
				} else {
					err = os.Symlink("retained", path)
				}
			case "modified":
				err = os.WriteFile(path, []byte("changed"), 0600)
			case "hardlink":
				err = os.Link(path, filepath.Join(dir, "extra"))
			case "unknown":
				name = "other"
			case "absent-birth":
				births = FileBirths{}
			}
			if err != nil {
				t.Fatal(err)
			}
			root, err := os.OpenRoot(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			got, err := births.Promote(root, name, 1234, 1234)
			if scenario == "original" {
				if err != nil || !os.SameFile(original, got) {
					t.Fatal("original file not retained", err)
				}
				stat, ok := got.Sys().(*syscall.Stat_t)
				if !ok || stat.Uid != 1234 || stat.Gid != 1234 {
					t.Fatal("runtime principal was not applied")
				}
			} else if !errors.Is(err, ErrInitialFileChanged) || got != nil {
				t.Fatal("foreign or changed file accepted", err)
			}
		})
	}
}
