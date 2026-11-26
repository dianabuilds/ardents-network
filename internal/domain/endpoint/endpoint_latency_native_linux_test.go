//go:build linux && endpoint_launch_native && document_worker_installed

package endpoint_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/route/transport"
	endpointfixture "github.com/dianabuilds/ardents-network/tests/e2e/endpoint"
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
	cold, warm, seed := endpointfixture.LatencyDriverCounts(t)
	f, _ := installedServiceNetwork(t, string(transport.ClosedCarrierTCP))
	stand := provisionPlatformLedLaunchFixture(t, 60000, f)
	t.Cleanup(func() {
		if t.Failed() {
			journal, _ := exec.Command("journalctl", "--no-pager", "-u", "ardents-endpoint.service", "-n", "60", "-o", "cat").CombinedOutput()
			t.Logf("cold/warm participant journal:\n%s", journal)
		}
	})
	credential := &syscall.Credential{Uid: stand.UID, Gid: stand.GID, Groups: []uint32{}}
	endpointfixture.InitializeInstalledServiceInstance(t, stand.Program, credential, stand.Base, stand.NetworkID)

	control := filepath.Join(stand.Base, "latency-control")
	if err := os.MkdirAll(control, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(control, int(stand.UID), int(stand.GID)); err != nil {
		t.Fatal(err)
	}
	participantPath := endpointfixture.InstallLatencyParticipant(t)
	unit := endpointfixture.RenderLatencyUnit(participantPath, stand.InstallationRoot, control, stand.Base, cold, warm, seed)
	endpointfixture.StartLatencyUnit(t, unit, participantPath)

	// The participant funds the publisher then the retained warm reader holder
	// during setup; approve each exact qualified request through the original
	// file handover. The signing key never leaves this driver.
	latencyApprove(t, stand, filepath.Join(stand.PublisherDir, "request"), filepath.Join(stand.PublisherDir, "response"), "publisher", stand.Base)
	latencyApprove(t, stand, filepath.Join(stand.PermissionDir, "request"), filepath.Join(stand.PermissionDir, "response"), "reader", stand.Base)

	// Cold attempts provision a fresh permission each; approve them as they
	// appear while the participant runs its bounded loop.
	approveCtx, stopApproving := context.WithCancel(t.Context())
	approvalDone := make(chan struct{})
	defer func() {
		stopApproving()
		<-approvalDone
	}()
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

	raw := endpointfixture.WaitLatencyReport(t, control)
	stopApproving()
	<-approvalDone
	endpointfixture.CheckLatencyVerdict(t, raw, cold, warm, seed)
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
	if err := endpointfixture.PublishPermissionResponse(t, responsePath, response, stand.UID, stand.GID); err != nil {
		t.Fatal(err)
	}
	return true
}
