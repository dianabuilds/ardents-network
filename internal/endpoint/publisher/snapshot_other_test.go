//go:build !linux && !darwin

package publisher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// LaunchSnapshotFile freezes a bounded document input before launching its
// qualified worker. On Linux and macOS (#883) it imports the caller input
// read-only and the native owner supplies the launch refusal (macOS workers are
// typed-unavailable). Windows keeps the typed refusal before any snapshot file
// I/O, so no temporary copy is created.
func TestLaunchSnapshotFileRefusesWithoutStateOnUnsupportedHost(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "snapshot")
	_, err := LaunchSnapshotFile(context.Background(), nil, [32]byte{}, path)
	if err == nil || err.Error() != "ordinary Publisher native launch is unavailable" {
		t.Fatalf("LaunchSnapshotFile platform refusal = %v, want ordinary Publisher native launch is unavailable", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("unsupported-host snapshot refusal created %d state entries", len(entries))
	}
}
