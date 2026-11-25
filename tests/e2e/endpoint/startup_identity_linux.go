//go:build linux && (document_worker_installed || endpoint_launch_native)

package endpoint

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

// StartupIdentity reads the fixed Endpoint unit from the real manager.
func StartupIdentity(t testing.TB, ctx context.Context) [3]string {
	t.Helper()
	var result [3]string
	for i, property := range []string{"MainPID", "InvocationID", "ActiveState"} {
		out, err := exec.CommandContext(ctx, "systemctl", "show", "ardents-endpoint.service", "-p", property, "--value").CombinedOutput()
		if err != nil {
			t.Fatalf("actual %s: %v: %s", property, err, out)
		}
		result[i] = strings.TrimSpace(string(out))
	}
	return result
}
