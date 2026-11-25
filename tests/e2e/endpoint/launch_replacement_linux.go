//go:build linux && endpoint_launch_native

package endpoint

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/platform/systemd"
)

// VerifyNativeEndpointReplacement exercises the original Endpoint and its signed replacement.
func VerifyNativeEndpointReplacement(t *testing.T, stand *LaunchStand, grantPermissions func(*testing.T, [3]string), activeWorker func(*testing.T, [3]string) string) {
	t.Helper()
	t.Cleanup(func() {
		if t.Failed() {
			journal, _ := exec.Command("journalctl", "--no-pager", "-u", "ardents-endpoint.service", "-n", "40", "-o", "cat").CombinedOutput()
			t.Logf("participant journal:\n%s", journal)
		}
	})
	original := StartOriginalEndpoint(t)
	grantPermissions(t, original)
	WaitOriginalReader(t, stand.ApplicationSock, original, stand.UID, stand.GID, 30*time.Second)
	stand.requireNoStartupExchange(t)
	t.Logf("platform-led Endpoint reached Service readiness; MainPID=%s InvocationID=%s", original[0], original[1])
	manager, err := systemd.RetainEndpoint(t.Context())
	if err != nil {
		t.Fatal("retain original Endpoint unit:", err)
	}
	defer func() {
		if err := manager.Close(); err != nil {
			t.Error("release Endpoint unit:", err)
		}
	}()
	stand.requireReplacementRefusal(t, "live predecessor")
	var worker string
	if activeWorker != nil {
		worker = activeWorker(t, original)
	}
	scopes := []*os.File{RetainLaunchScope(t, "/sys/fs/cgroup/system.slice/ardents-endpoint.service")}
	if worker != "" {
		scopes = append(scopes, RetainLaunchScope(t, worker))
	}
	if worker != "" {
		if !LiveSnapshotWorker(worker) {
			t.Fatal("Publisher worker ended before the original Endpoint Stop")
		}
		t.Logf("draining Endpoint with live Publisher worker in %s", filepath.Base(worker))
		stand.drainParticipant(t, original)
	} else {
		StopOriginalEndpoint(t, original)
	}
	RequireJoinedEndpoint(t, original)
	RequireStoppedEndpointScopes(t, scopes)
	if err := manager.Observe(t.Context()); err != nil {
		t.Fatal("observe original manager after Stop:", err)
	}
	stand.requireNoStartupExchange(t)
	guard := filepath.Join(stand.InstallationRoot, "start-guard.json")
	WriteLaunchFile(t, guard, []byte("interrupted transition fixture"), 0600)
	stand.requireReplacementRefusal(t, "uncertain predecessor")
	if err := os.Remove(guard); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(stand.SuccessorRoot, 0700); err != nil {
		t.Fatal(err)
	}
	partial := filepath.Join(stand.SuccessorRoot, "interrupted")
	WriteLaunchFile(t, partial, []byte("incomplete original candidate"), 0600)
	if out, err := exec.CommandContext(t.Context(), stand.Binary, "platform", "stage-endpoint", stand.SuccessorRequest, stand.Manifest).CombinedOutput(); err == nil {
		t.Fatalf("staging adopted interrupted candidate: %s", out)
	}
	if body, err := os.ReadFile(partial); err != nil || string(body) != "incomplete original candidate" {
		t.Fatalf("staging changed interrupted candidate: %v", err)
	}
	if err := os.RemoveAll(stand.SuccessorRoot); err != nil {
		t.Fatal(err)
	}
	stand.startReplacement(t, original, grantPermissions)
}

// A systemd stop transaction tears down dependent workers before their
// Endpoint can close them. Signal its main process first so its own runtime
// joins the original borrowers before the platform stops the fixed sockets.
func (s *LaunchStand) drainParticipant(t *testing.T, original [3]string) {
	t.Helper()
	if current := StartupIdentity(t, t.Context()); current != original {
		t.Fatalf("original Endpoint changed before drain: %v", current)
	}
	if output, err := exec.CommandContext(t.Context(), "systemctl", "kill", "--kill-who=main", "--signal=SIGTERM", "ardents-endpoint.service").CombinedOutput(); err != nil {
		t.Fatalf("signal original Endpoint: %v: %s", err, output)
	}
	launchWait(t, func() ([]byte, bool) {
		if _, err := os.Stat(filepath.Join("/proc", original[0])); !os.IsNotExist(err) {
			return nil, false
		}
		state := StartupIdentity(t, t.Context())
		return nil, state[2] == "inactive" || state[2] == "failed"
	})
	RequireJoinedEndpoint(t, original)
	if output, err := exec.CommandContext(t.Context(), "systemctl", "stop", "ardents-text-reader.socket", "ardents-text-publisher.socket").CombinedOutput(); err != nil {
		t.Fatalf("stop original activation sockets: %v: %s", err, output)
	}
}

func (s *LaunchStand) requireReplacementRefusal(t *testing.T, reason string) {
	t.Helper()
	if out, err := exec.CommandContext(t.Context(), s.Binary, "platform", "stage-endpoint", s.SuccessorRequest, s.Manifest).CombinedOutput(); err == nil {
		t.Fatalf("staging admitted %s: %s", reason, out)
	}
	if _, err := os.Lstat(s.SuccessorRoot); !os.IsNotExist(err) {
		t.Fatalf("staging %s touched the successor root: %v", reason, err)
	}
}

func (s *LaunchStand) startReplacement(t *testing.T, original [3]string, grantPermissions func(*testing.T, [3]string)) {
	t.Helper()
	if out, err := exec.CommandContext(t.Context(), s.Binary, "platform", "stage-endpoint", s.SuccessorRequest, s.Manifest).CombinedOutput(); err != nil {
		t.Fatalf("platform successor staging: %v: %s", err, out)
	}
	if out, err := exec.CommandContext(t.Context(), "systemctl", "start", "ardents-endpoint.service").CombinedOutput(); err == nil {
		t.Fatalf("superseded Release was still admitted: %s", out)
	}
	if err := os.Remove("/etc/systemd/system/ardents-endpoint.service"); err != nil {
		t.Fatal(err)
	}
	launch := NativeLaunch(t, s.SuccessorRequest, s.SuccessorDistribution)
	InstallNativeUnit(t, s.SuccessorRequest, s.SuccessorDistribution, launch.Program)
	next := StartOriginalEndpoint(t)
	if next[1] == original[1] {
		t.Fatal("replacement reused original invocation")
	}
	grantPermissions(t, next)
	WaitOriginalReader(t, s.ApplicationSock, next, s.UID, s.GID, 30*time.Second)
	output, err := os.Readlink(filepath.Join("/proc", next[0], "exe"))
	if err != nil || output != launch.Program {
		t.Fatalf("replacement executable = %q, want %q: %v", output, launch.Program, err)
	}
	StopOriginalEndpoint(t, next)
	t.Logf("different signed Endpoint artifact reached Service readiness; InvocationID=%s", next[1])
}

func (s *LaunchStand) requireNoStartupExchange(t *testing.T) {
	t.Helper()
	for _, name := range []string{"start-guard.json", "start-socket.json", "start-completion.socket"} {
		if _, err := os.Lstat(filepath.Join(s.InstallationRoot, name)); !os.IsNotExist(err) {
			t.Fatalf("platform-led Endpoint retained guarded-start artifact %s: %v", name, err)
		}
	}
}

func launchWait(t *testing.T, observe func() ([]byte, bool)) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		raw, done := observe()
		if done {
			return raw
		}
		select {
		case <-ctx.Done():
			t.Fatal("actual installed observation exceeded bound", ctx.Err())
		case <-tick.C:
		}
	}
}
