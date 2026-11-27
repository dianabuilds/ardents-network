//go:build linux && endpoint_pair_native

package endpointinstalled_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	endpointfixture "github.com/dianabuilds/ardents-network/tests/e2e/endpoint"
)

// This native qualification proves the platform-led launch foundation on TWO
// separately admitted Ubuntu/systemd managers (#391 S2.1). One Endpoint per
// system manager is a fixed, multiply-enforced contract (dedicated account, unit
// names, fixed resource paths, preflight absence and DynamicUser name-hash UIDs),
// so the installed two-Endpoint oracle requires two independent managers. The
// controller provisions two fresh systemd-nspawn guests, each its own PID-1
// systemd with separate PID/mount/cgroup namespaces and a pivoted Ubuntu 24.04
// root, sharing the host network namespace (fixture loopback). Each guest passes
// the same manager-local platform admission the product enforces (Ubuntu
// 24.04/systemd 255, cgroup v2, root-owned os-release, reachable system bus) and
// independently runs the checked endpoint_launch_native full-launch qualification
// to protected document Service readiness.
//
// It is component/acceptance evidence with fixture Release/Network authority for
// two independently admitted managers reaching platform-led Service readiness.
// It is NOT the cross-Endpoint publish/link/read journey (#391 S2.2) and NOT the
// whole-host two-participant privacy qualification. The transitional
// guarded-start/ACK/recovery orchestration is neither exercised nor retired
// (ADR-0125 D6 gates retirement on proven replacement guarantees).

func TestNativeEndpointPlatformLedPair(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Fatal("endpoint platform-led pair qualification requires a root driver")
	}
	for _, tool := range []string{"systemd-nspawn", "machinectl", "debootstrap", "cp", "systemd-run"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Fatalf("invalid environment: %s unavailable: %v", tool, err)
		}
	}
	if out, err := exec.Command("stat", "-fc", "%T", "/sys/fs/cgroup").Output(); err != nil || strings.TrimSpace(string(out)) != "cgroup2fs" {
		t.Fatalf("invalid environment: host cgroup v2 required: %v: %s", err, out)
	}
	repoRoot := endpointfixture.RepositoryRoot(t)
	goRoot := endpointfixture.GoEnv(t, "GOROOT")
	modCache := endpointfixture.GoEnv(t, "GOMODCACHE")
	buildCache := endpointfixture.GoEnv(t, "GOCACHE")
	base := endpointfixture.EnsurePairBaseRootfs(t)

	runID := fmt.Sprintf("%d-%d", os.Getpid(), time.Now().UnixNano())
	var guests [2]*endpointfixture.PairGuest
	for i, label := range []string{"a", "b"} {
		guests[i] = endpointfixture.BootPairGuest(t, base, runID, label, goRoot, repoRoot, modCache, buildCache)
	}
	endpointfixture.VerifyDistinctManagers(t, guests[:])

	// Run the checked full-launch qualification independently in each guest. The
	// runs are serial so the two guests' ephemeral fixture Source ports on the
	// shared loopback cannot race; both managers remain admitted throughout.
	var invocations [2]string
	for i, guest := range guests {
		invocations[i] = pairRunLaunchQualification(t, guest, repoRoot)
	}
	if invocations[0] == "" || invocations[1] == "" || invocations[0] == invocations[1] {
		t.Fatal("two managers did not produce distinct original invocations", invocations)
	}
	t.Logf("two independently admitted managers reached platform-led Service readiness; InvocationIDs %s / %s", invocations[0], invocations[1])
}

// pairRunLaunchQualification runs the checked endpoint_launch_native full-launch
// qualification inside the guest against its own original manager, using the
// canonical repository and toolchain bound at their host paths. It returns the
// observed original InvocationID and refuses any skip.
func pairRunLaunchQualification(t *testing.T, guest *endpointfixture.PairGuest, repoRoot string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 6*time.Minute)
	defer cancel()
	script := fmt.Sprintf("cd %s && go test -tags=endpoint_launch_native -run '^TestNativeEndpointPlatformLedLaunch' -count=1 -v ./tests/e2e/endpointinstalled", repoRoot)
	cmd := exec.CommandContext(ctx, "systemd-run", "-M", guest.Name(), "--pipe", "--quiet",
		"--setenv=PATH="+filepath.Join(endpointfixture.GoEnv(t, "GOROOT"), "bin")+":/usr/sbin:/usr/bin:/sbin:/bin",
		"--setenv=GOTOOLCHAIN=local",
		"--setenv=GOMODCACHE="+endpointfixture.GoEnv(t, "GOMODCACHE"),
		"--setenv=GOCACHE="+endpointfixture.GoEnv(t, "GOCACHE"),
		"--setenv=GOFLAGS=-buildvcs=false",
		"/bin/bash", "-c", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("guest %s launch qualification failed: %v\n%s", guest.Name(), err, out)
	}
	if bytes.Contains(out, []byte("--- SKIP:")) {
		t.Fatalf("guest %s launch qualification passed by skip:\n%s", guest.Name(), out)
	}
	if !bytes.Contains(out, []byte("--- PASS: TestNativeEndpointPlatformLedLaunch ")) {
		t.Fatalf("guest %s lacks executed positive launch evidence:\n%s", guest.Name(), out)
	}
	if !bytes.Contains(out, []byte("--- PASS: TestNativeEndpointPlatformLedLaunchRefusals ")) {
		t.Fatalf("guest %s lacks executed causal refusal evidence:\n%s", guest.Name(), out)
	}
	for _, line := range bytes.Split(out, []byte{'\n'}) {
		if marker := []byte("InvocationID="); bytes.Contains(line, marker) {
			rest := line[bytes.Index(line, marker)+len(marker):]
			if fields := strings.Fields(string(rest)); len(fields) > 0 && len(fields[0]) == 32 {
				return fields[0]
			}
		}
	}
	t.Fatalf("guest %s launch qualification observed no original InvocationID:\n%s", guest.Name(), out)
	return ""
}
