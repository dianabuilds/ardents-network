//go:build linux && endpoint_launch_native

package endpointinstalled_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	endpointfixture "github.com/dianabuilds/ardents-network/tests/e2e/endpoint"
)

// NET-32 user-idle component measurement (M2 slice 3a, #652). It provisions ONE
// platform-led successor Endpoint to Service readiness and leaves it idle — no
// publication, no Application job — then observes the required State/time refresh
// traffic and whole-client RSS over a bounded ten-minute window with a conservative
// 24-hour projection (privacy-qualification.md:80,112-132). This is component
// evidence for one fixture operator, NOT a 24-hour observed run and NOT whole-host
// privacy qualification. It reuses the #391 platform-led launch stand; the product
// Hosting API is unchanged — the harness reads the persisted Hosting reading and
// the unit cgroups directly for the interface/resource reconciliation.
const net32IdleWindow = 10 * time.Minute

func TestNativeEndpointPlatformLedIdle(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Fatal("NET-32 idle qualification requires a root driver")
	}
	if _, err := exec.Command("/usr/bin/busctl", "--system", "--no-pager", "call", "org.freedesktop.systemd1", "/org/freedesktop/systemd1", "org.freedesktop.DBus.Properties", "Get", "ss", "org.freedesktop.systemd1.Manager", "Version").Output(); err != nil {
		t.Fatal("real platform systemd manager unavailable:", err)
	}
	// Normal idle refresh cadence (60 s), not the launch fixture's fast 1000 ms
	// polling: NET-32 measures ordinary user-idle refresh traffic.
	stand, grant := provisionLaunchWithPermissions(t, 60000, false, nil)
	t.Cleanup(func() {
		if t.Failed() {
			journal, _ := exec.Command("journalctl", "--no-pager", "-u", "ardents-endpoint.service", "-n", "40", "-o", "cat").CombinedOutput()
			t.Logf("participant journal:\n%s", journal)
		}
	})
	original := endpointfixture.StartOriginalEndpoint(t)
	grant(t, original)
	endpointfixture.WaitOriginalReader(t, stand.ApplicationSock, original, stand.UID, stand.GID, 30*time.Second)
	t.Logf("NET-32 idle Endpoint ready; MainPID=%s InvocationID=%s", original[0], original[1])

	hostingRoot := filepath.Join(stand.Base, "entry", "hosting")
	verdict, err := endpointfixture.MeasureNET32Idle(t.Context(), hostingRoot, original[0], filepath.Join(stand.Base, "state"), net32IdleWindow)
	if err != nil {
		t.Fatalf("NET-32 idle observation: %v", err)
	}
	raw, _ := json.MarshalIndent(verdict, "", "  ")
	t.Logf("NET-32 short projection verdict:\n%s", raw)
	for _, criterion := range verdict.Criteria {
		if !criterion.Passed {
			t.Errorf("NET-32 criterion failed: %s observed=%v %s bound=%v", criterion.Name, criterion.Observed, criterion.Relation, criterion.Bound)
		}
	}
	endpointfixture.StopOriginalEndpoint(t, original)
}
