package hosting

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHostingFilesPreserveOriginalDirectoryAndRefuseReplay(t *testing.T) {
	path := t.TempDir()
	if err := os.Chmod(path, 0700); err != nil {
		t.Fatal(err)
	}
	before, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	now := time.Now().UTC().Truncate(time.Second)
	policy := Policy{Provider: "local counter fixture", Start: now.Add(-time.Minute), End: now.Add(time.Hour), Unit: "GiB", Quantity: 1, Directions: "tx+rx", Interfaces: []string{"lo"}, InitialUsedBytes: 100, LowWatermarkBytes: 1024}
	births, err := InitializeFiles(t.Context(), root, policy)
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.Lstat(path)
	if err != nil || !os.SameFile(before, after) {
		t.Fatal("initialization replaced original directory", err)
	}
	for _, name := range []string{"budget.pin", "budget.lock", "budget.json"} {
		current, err := root.Lstat(name)
		if err != nil || births.Original(name) == nil || !os.SameFile(births.Original(name), current) {
			t.Fatal("original file birth differs", name, err)
		}
	}
	if births.Original("budget.pending") != nil {
		t.Fatal("temporary file became a period birth")
	}
	original, err := os.ReadFile(filepath.Join(path, "budget.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := InitializeFiles(t.Context(), root, policy); err == nil {
		t.Fatal("existing period was initialized again")
	}
	unchanged, err := os.ReadFile(filepath.Join(path, "budget.json"))
	if err != nil || string(unchanged) != string(original) {
		t.Fatal("refused initialization changed original budget", err)
	}
	owner, err := Open(path)
	if err != nil {
		t.Fatal("ordinary owner cannot reopen initialized files", err)
	}
	view, err := owner.Observe(t.Context())
	if err != nil || view.UsedBytes < 100 || view.ReservedBytes != 0 {
		t.Fatal("original floor lost", view, err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestHostingFilesRefuseInvalidCanceledAndUnknownDirectoryWithoutEffects(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	policy := Policy{Provider: "local counter fixture", Start: now.Add(-time.Minute), End: now.Add(time.Hour), Unit: "GiB", Quantity: 1, Directions: "tx+rx", Interfaces: []string{"lo"}, LowWatermarkBytes: 1024}
	for _, scenario := range []string{"invalid", "canceled", "nonempty", "closed"} {
		t.Run(scenario, func(t *testing.T) {
			path := t.TempDir()
			if err := os.Chmod(path, 0700); err != nil {
				t.Fatal(err)
			}
			root, err := os.OpenRoot(path)
			if err != nil {
				t.Fatal(err)
			}
			defer root.Close()
			candidate := policy
			ctx := t.Context()
			switch scenario {
			case "invalid":
				candidate.LowWatermarkBytes = 0
			case "canceled":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "nonempty":
				if err := os.WriteFile(filepath.Join(path, "foreign"), []byte("retained"), 0600); err != nil {
					t.Fatal(err)
				}
			case "closed":
				if err := root.Close(); err != nil {
					t.Fatal(err)
				}
			}
			births, err := InitializeFiles(ctx, root, candidate)
			if err == nil || births.Original("budget.pin") != nil {
				t.Fatal("unusable initialization returned births", err)
			}
			entries, err := os.ReadDir(path)
			if err != nil {
				t.Fatal(err)
			}
			wanted := 0
			if scenario == "nonempty" {
				wanted = 1
			}
			if len(entries) != wanted {
				t.Fatal("refused input changed directory", entries)
			}
		})
	}
}
