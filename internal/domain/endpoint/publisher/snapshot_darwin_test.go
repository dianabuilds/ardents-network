//go:build darwin

package publisher

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// On macOS the native worker launch is a typed refusal (#883). LaunchSnapshotFile
// imports the caller's explicitly selected input read-only (localsnapshot.Read
// creates no filesystem state or authority) and then refuses at the native launch,
// so no temporary copy or other state is created and no unconfined worker is spawned.
// The root is resolved past the system /var symlink per the established portable
// fixture pattern (#863) so the read-only O_NOFOLLOW component open succeeds.
func TestLaunchSnapshotFileRefusesAfterReadOnlyInputOnDarwin(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "snapshot")
	if err := os.WriteFile(path, []byte("original snapshot"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err = LaunchSnapshotFile(context.Background(), nil, [32]byte{}, path)
	if err == nil || err.Error() != "ordinary Publisher native launch is unavailable" {
		t.Fatalf("LaunchSnapshotFile platform refusal = %v, want ordinary Publisher native launch is unavailable", err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "snapshot" {
		t.Fatalf("macOS snapshot refusal created state beyond the caller input: %d entries", len(entries))
	}
}
