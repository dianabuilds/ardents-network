//go:build linux && (document_worker_installed || endpoint_launch_native)

package endpoint

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// GrantOriginalPermissions synchronizes the two responses with the original
// participant invocation and its reported acceptance of each role.
func GrantOriginalPermissions(t testing.TB, base, invocation string, uid, gid uint32, bound time.Duration, approve func(base, role string, request []byte) []byte) {
	t.Helper()
	for _, entry := range []struct{ role, directory string }{
		{"reader", "permission"},
		{"publisher", "publisher-permission"},
	} {
		waitPermissionEvent(t, invocation, "permission-pending", entry.role, bound)
		directory := filepath.Join(base, entry.directory)
		request, err := os.ReadFile(filepath.Join(directory, "request"))
		if err != nil {
			t.Fatalf("read original %s permission request: %v", entry.role, err)
		}
		response := approve(base, entry.role, request)
		if err := PublishPermissionResponse(t, filepath.Join(directory, "response"), response, uid, gid); err != nil {
			t.Fatal(err)
		}
		waitPermissionEvent(t, invocation, "permission-accepted", entry.role, bound)
	}
}

func waitPermissionEvent(t testing.TB, invocation, phase, role string, bound time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), bound)
	defer cancel()
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		raw, err := exec.CommandContext(ctx, "journalctl", "--no-pager", "-o", "cat", "_SYSTEMD_INVOCATION_ID="+invocation).CombinedOutput()
		if err != nil {
			t.Fatalf("observe original %s %s journal: %v: %s", role, phase, err, raw)
		}
		if hasPermissionEvent(raw, phase, role) {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("original %s %s observation exceeded the bound: %v", role, phase, ctx.Err())
		case <-tick.C:
		}
	}
}
