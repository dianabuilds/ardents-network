package instance

import (
	"context"
	"crypto/sha256"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestNativeInstanceRefusesIncompleteAndForeignRootsUnchanged(t *testing.T) {
	for _, scenario := range []string{"foreign", "old-marker", "unknown-marker", "marker-only", "lock-only", "state-only", "missing-lock", "missing-state", "bad-state", "retired-marker"} {
		t.Run(scenario, func(t *testing.T) {
			config := nativeConfig(t)
			if err := os.Mkdir(config.Root, 0700); err != nil {
				t.Fatal(err)
			}
			write := func(name, value string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(config.Root, name), []byte(value), 0600); err != nil {
					t.Fatal(err)
				}
			}
			switch scenario {
			case "foreign":
				write("legacy-evidence", "retained independent evidence\n")
			case "old-marker":
				write(markerName, "ardents-service-instance-root-v2\n")
			case "unknown-marker":
				write(markerName, "unknown-instance-root\n")
			case "marker-only":
				write(markerName, "ardents-service-instance-root-v3\n")
			case "lock-only":
				write(lockName, "")
			case "state-only":
				write(stateName, "retained damaged state")
			default:
				owner, err := Initialize(context.Background(), config)
				if err != nil {
					t.Fatal(err)
				}
				if err := owner.Close(); err != nil {
					t.Fatal(err)
				}
				switch scenario {
				case "missing-lock", "missing-state":
					name := lockName
					if scenario == "missing-state" {
						name = stateName
					}
					if err := os.Remove(filepath.Join(config.Root, name)); err != nil {
						t.Fatal(err)
					}
				case "bad-state":
					write(stateName, "retained damaged state")
				case "retired-marker":
					write(markerName, "ardents-service-instance-root-v2\n")
				}
			}
			before := instanceTree(t, config.Root)
			for _, initialize := range []bool{false, true, true} {
				var owner *Root
				var err error
				if initialize {
					owner, err = Initialize(context.Background(), config)
				} else {
					owner, err = Open(context.Background(), config.Root)
				}
				if err == nil {
					_ = owner.Close()
					t.Fatal("invalid root accepted or reconstructed")
				}
				after := instanceTree(t, config.Root)
				if len(before) != len(after) {
					t.Fatalf("refusal changed inventory: before %d, after %d", len(before), len(after))
				}
				for name, prior := range before {
					next, exists := after[name]
					if !exists || prior.digest != next.digest || !os.SameFile(prior.info, next.info) || prior.info.Mode() != next.info.Mode() || prior.info.Size() != next.info.Size() || !prior.info.ModTime().Equal(next.info.ModTime()) {
						t.Fatalf("refusal changed retained member %s", filepath.Base(name))
					}
				}
			}
		})
	}
}

type instanceTreeMember struct {
	info   fs.FileInfo
	digest [32]byte
}

func instanceTree(t *testing.T, root string) map[string]instanceTreeMember {
	t.Helper()
	result := make(map[string]instanceTreeMember)
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
		result[path] = instanceTreeMember{info: info, digest: sha256.Sum256(raw)}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}
