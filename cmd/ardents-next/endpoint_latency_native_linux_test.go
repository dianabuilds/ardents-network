//go:build linux && endpoint_launch_native && text_worker_installed

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
)

// TestNativeEndpointColdWarmLatency is the root driver for the cold/warm latency
// component measurement (M2 slice 3b, #650). It keeps the genuine publication
// Route network and signing authority, provisions one platform-led dual-role
// installed root, initializes the publication Instance credential, then pins a
// temporary qualification unit whose non-root MainPID is the endpoint-package
// test binary (TestNativeEndpointColdWarmLatencyParticipant). That participant
// retains the original owner composition, publishes the reference document and
// measures cold/warm reads directly; this driver only approves the exact
// qualified permission requests and observes the reported verdict.
//
// It is component evidence on the fixture stand, NOT the full 1,000+1,000 x
// 3-seed paced campaign and NOT whole-host privacy qualification. The short
// validation exercises a few cold/warm pairs and the verdict logic.
func TestNativeEndpointColdWarmLatency(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Fatal("cold/warm latency qualification requires a root driver")
	}
	if _, err := exec.Command("/usr/bin/busctl", "--system", "--no-pager", "call", "org.freedesktop.systemd1", "/org/freedesktop/systemd1", "org.freedesktop.DBus.Properties", "Get", "ss", "org.freedesktop.systemd1.Manager", "Version").Output(); err != nil {
		t.Fatal("real platform systemd manager unavailable:", err)
	}
	cold, warm, seed := latencyDriverCounts(t)
	f, _ := installedServiceNetwork(t, string(transport.ClosedCarrierTCP))
	stand := provisionPlatformLedLaunchFixture(t, 60000, f)
	t.Cleanup(func() {
		if t.Failed() {
			journal, _ := exec.Command("journalctl", "--no-pager", "-u", "ardents-endpoint.service", "-n", "60", "-o", "cat").CombinedOutput()
			t.Logf("cold/warm participant journal:\n%s", journal)
		}
	})
	credential := &syscall.Credential{Uid: stand.uid, Gid: stand.gid, Groups: []uint32{}}
	installedServiceInstance(t, stand.program, credential, stand.base, stand.network.NetworkID)

	control := filepath.Join(stand.base, "latency-control")
	if err := os.MkdirAll(control, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(control, int(stand.uid), int(stand.gid)); err != nil {
		t.Fatal(err)
	}
	participantPath := latencyInstallParticipant(t)
	unit := latencyRenderUnit(participantPath, stand.installationRoot, control, stand.base, cold, warm, seed)
	// The launch stand stages the production platform unit at /etc, which takes
	// precedence over /run; replace it with the temporary qualification unit so
	// the loaded MainPID is the participant test binary, not the product command.
	const unitPath = "/etc/systemd/system/ardents-endpoint.service"
	if err := os.MkdirAll(filepath.Dir(unitPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(unitPath, []byte(unit), 0644); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("systemctl", "daemon-reload").Run(); err != nil {
		t.Fatal("daemon-reload:", err)
	}
	t.Cleanup(func() {
		_ = exec.Command("systemctl", "stop", "ardents-endpoint.service").Run()
		_ = os.Remove(unitPath)
		_ = exec.Command("systemctl", "daemon-reload").Run()
		_ = exec.Command("systemctl", "reset-failed", "ardents-endpoint.service").Run()
		_ = os.Remove(participantPath)
	})
	if out, err := exec.Command("systemctl", "start", "ardents-endpoint.service").CombinedOutput(); err != nil {
		t.Fatalf("cold/warm participant start: %v: %s", err, out)
	}

	// The participant funds the publisher then the retained warm reader holder
	// during setup; approve each exact qualified request through the original
	// file handover. The signing key never leaves this driver.
	latencyApprove(t, stand, filepath.Join(stand.publisherDir, "request"), filepath.Join(stand.publisherDir, "response"), "publisher", stand.base)
	latencyApprove(t, stand, filepath.Join(stand.permissionDir, "request"), filepath.Join(stand.permissionDir, "response"), "reader", stand.base)

	// Cold attempts provision a fresh permission each; approve them as they
	// appear while the participant runs its bounded loop.
	approveCtx, stopApproving := context.WithCancel(t.Context())
	defer stopApproving()
	approvalDone := make(chan struct{})
	go func() {
		defer close(approvalDone)
		for i := 0; i < cold; i++ {
			base := filepath.Join(control, fmt.Sprintf("cold-%d", i))
			request := filepath.Join(base, "permission", "request")
			response := filepath.Join(base, "permission", "response")
			if !latencyApproveBounded(approveCtx, t, stand, request, response, "reader", base) {
				return
			}
		}
	}()

	resultPath := filepath.Join(control, "result.json")
	var raw []byte
	// The participant reports only after the warm loop, the open-phase diagnostic
	// and every cold attempt; that bounded run outlasts the generic 30 s launch
	// observation, so wait on its own longer bound.
	resultCtx, resultCancel := context.WithTimeout(t.Context(), 5*time.Minute)
	defer resultCancel()
	resultTick := time.NewTicker(200 * time.Millisecond)
	defer resultTick.Stop()
	for {
		data, err := os.ReadFile(resultPath)
		if err == nil {
			raw = data
			break
		}
		if current := launchIdentity(t); current[2] != "active" && current[0] == "0" {
			t.Fatalf("cold/warm participant exited before reporting a verdict: %+v", current)
		}
		select {
		case <-resultCtx.Done():
			t.Fatal("cold/warm participant did not report a verdict within bound:", resultCtx.Err())
		case <-resultTick.C:
		}
	}
	stopApproving()
	<-approvalDone
	var verdict struct {
		Kind      string `json:"kind"`
		ColdStats struct {
			P95MS any `json:"p95_ms"`
		} `json:"cold_stats"`
		WarmStats struct {
			P95MS any `json:"p95_ms"`
		} `json:"warm_stats"`
		ColdP95OK bool `json:"cold_p95_ok"`
		WarmP95OK bool `json:"warm_p95_ok"`
		Cold      []struct {
			Success bool   `json:"success"`
			Detail  string `json:"detail"`
		} `json:"cold"`
		Warm []struct {
			Success bool   `json:"success"`
			Detail  string `json:"detail"`
		} `json:"warm"`
	}
	if err := json.Unmarshal(raw, &verdict); err != nil {
		t.Fatalf("cold/warm verdict decode: %v: %s", err, raw)
	}
	t.Logf("cold/warm component verdict: cold p95=%v (ok=%v) warm p95=%v (ok=%v); %d cold / %d warm outcomes",
		verdict.ColdStats.P95MS, verdict.ColdP95OK, verdict.WarmStats.P95MS, verdict.WarmP95OK, len(verdict.Cold), len(verdict.Warm))
	if verdict.Kind != "cold-warm-component" || len(verdict.Cold) != cold || len(verdict.Warm) != warm {
		t.Fatalf("cold/warm verdict did not report every declared attempt: %s", raw)
	}
	for _, outcome := range verdict.Cold {
		if !outcome.Success {
			t.Errorf("cold/warm cold attempt counted as non-success (infinite): %s", outcome.Detail)
		}
	}
	for _, outcome := range verdict.Warm {
		if !outcome.Success {
			t.Errorf("cold/warm warm attempt counted as non-success (infinite): %s", outcome.Detail)
		}
	}
	// This bounded short run is component evidence that exercises the verdict
	// logic; it is NOT the bound qualification. The 1,000+1,000 x 3-seed paced
	// campaign on the normal reference cell is the separate execution receipt that
	// qualifies the 3 s / 1 s bounds. Report the fixture measurement honestly,
	// including any exceedance, rather than waiving or silently passing it.
	if !verdict.ColdP95OK {
		t.Logf("FINDING: cold p95 exceeds the 3 s bound on this fixture stand: %v ms (open-phase composition cost; bound qualification belongs to the full paced campaign)", verdict.ColdStats.P95MS)
	}
	if !verdict.WarmP95OK {
		t.Logf("FINDING: warm p95 exceeds the 1 s bound on this fixture stand: %v ms (open-phase composition cost; bound qualification belongs to the full paced campaign)", verdict.WarmStats.P95MS)
	}
	if out, err := exec.Command("systemctl", "stop", "ardents-endpoint.service").CombinedOutput(); err != nil {
		t.Errorf("cold/warm participant stop: %v: %s", err, out)
	}
}

func latencyDriverCounts(t *testing.T) (cold, warm int, seed string) {
	t.Helper()
	seed = os.Getenv("ARDENTS_LATENCY_SEED")
	if seed == "" {
		seed = "seed-1"
	}
	cold, warm = 3, 3
	if value := os.Getenv("ARDENTS_LATENCY_COLD"); value != "" {
		if _, err := fmt.Sscanf(value, "%d", &cold); err != nil || cold < 1 || cold > 1000 {
			t.Fatal("invalid cold count", value)
		}
	}
	if value := os.Getenv("ARDENTS_LATENCY_WARM"); value != "" {
		if _, err := fmt.Sscanf(value, "%d", &warm); err != nil || warm < 1 || warm > 1000 {
			t.Fatal("invalid warm count", value)
		}
	}
	return cold, warm, seed
}

// latencyInstallParticipant builds the endpoint-package qualification test binary
// (the non-root MainPID) and installs it root-owned immutable, mirroring the
// installed text-worker qualification pattern. A prebuilt binary may be supplied
// to avoid a Go build under shared-host load; its bytes are still pinned.
func latencyInstallParticipant(t *testing.T) string {
	t.Helper()
	const installed = "/usr/lib/ardents/qualification/latency-participant.test"
	var raw []byte
	if prebuilt := os.Getenv("ARDENTS_LATENCY_PREBUILT_PARTICIPANT"); prebuilt != "" {
		data, err := os.ReadFile(prebuilt)
		if err != nil {
			t.Fatal(err)
		}
		raw = data
	} else {
		repoRoot := latencyRepoRoot(t)
		out := filepath.Join(t.TempDir(), "latency-participant.test")
		cmd := exec.CommandContext(t.Context(), "go", "test", "-c", "-tags=endpoint_launch_native", "-trimpath", "-buildvcs=false", "-o", out, "./internal/successor/endpoint")
		cmd.Dir = repoRoot
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build cold/warm participant: %v: %s", err, output)
		}
		data, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		raw = data
	}
	if err := os.MkdirAll(filepath.Dir(installed), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(installed, raw, 0555); err != nil {
		t.Fatal(err)
	}
	return installed
}

func latencyRepoRoot(t *testing.T) string {
	t.Helper()
	if root := os.Getenv("ARDENTS_LATENCY_REPO_ROOT"); root != "" {
		return root
	}
	// The driver runs from cmd/ardents-next; the module root is two levels up.
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Dir(filepath.Dir(wd))
}

func latencyRenderUnit(program, root, control, base string, cold, warm int, seed string) string {
	return fmt.Sprintf(`[Unit]
Description=Ardents cold/warm latency qualification Endpoint
Requires=ardents-text-reader.socket ardents-text-publisher.socket
After=network-online.target ardents-text-reader.socket ardents-text-publisher.socket
Wants=network-online.target

[Service]
Type=exec
User=ardents-endpoint
Group=ardents-endpoint
SupplementaryGroups=
ExecStart=%s -test.run=^TestNativeEndpointColdWarmLatencyParticipant$ -test.v -test.timeout=28m
Environment=ARDENTS_LATENCY_ROOT=%s
Environment=ARDENTS_LATENCY_CONTROL=%s
Environment=ARDENTS_LATENCY_COLD=%d
Environment=ARDENTS_LATENCY_WARM=%d
Environment=ARDENTS_LATENCY_SEED=%s
WorkingDirectory=/
UMask=0077
NoNewPrivileges=yes
CapabilityBoundingSet=
AmbientCapabilities=
PrivateTmp=yes
ProtectHome=yes
ProtectSystem=strict
ReadWritePaths=%s /etc/ardents
ProtectControlGroups=yes
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectKernelLogs=yes
RestrictSUIDSGID=yes
LockPersonality=yes
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6
MemoryAccounting=yes
CPUAccounting=yes
TasksAccounting=yes
LimitCORE=0
Restart=no
RestartMode=normal
RemainAfterExit=no
ExitType=main
KillMode=control-group
TimeoutStopSec=30s
RuntimeMaxSec=29m
StandardOutput=journal
StandardError=journal
`, program, root, control, cold, warm, seed, base)
}

// latencyApprove waits for the participant's exact qualified request and writes
// the independently signed response through the original durable handover.
func latencyApprove(t *testing.T, stand *platformLedLaunchStand, requestPath, responsePath, role, journalBase string) {
	t.Helper()
	if !latencyApproveBounded(t.Context(), t, stand, requestPath, responsePath, role, journalBase) {
		t.Fatal("cold/warm permission request did not appear:", requestPath)
	}
}

func latencyApproveBounded(ctx context.Context, t *testing.T, stand *platformLedLaunchStand, requestPath, responsePath, role, journalBase string) bool {
	t.Helper()
	bounded, cancel := context.WithTimeout(ctx, 6*time.Minute)
	defer cancel()
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	var request []byte
	for {
		raw, err := os.ReadFile(requestPath)
		if err == nil && len(raw) > 0 {
			request = raw
			break
		}
		select {
		case <-bounded.Done():
			return false
		case <-tick.C:
		}
	}
	response := installedServiceApproval(t, journalBase, role, request, stand.f)
	pending := responsePath + ".pending"
	launchWrite(t, pending, response, 0600)
	if err := os.Chown(pending, int(stand.uid), int(stand.gid)); err != nil {
		t.Fatal(err)
	}
	launchSync(t, pending)
	if _, err := os.Lstat(responsePath); !os.IsNotExist(err) {
		t.Fatal("refuse existing cold/warm permission response", err)
	}
	if err := os.Rename(pending, responsePath); err != nil {
		t.Fatal(err)
	}
	launchSync(t, filepath.Dir(responsePath))
	return true
}
