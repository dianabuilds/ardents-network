package testkit

import (
	"testing"
	"time"

	transport "ardents/internal/network/api"
	nodeapi "ardents/internal/node/api"

	"github.com/stretchr/testify/require"
)

func StartTransport(t *testing.T) transport.Service {
	t.Helper()

	ConfigureLoopbackTransport(t)
	svc := transport.New()
	require.NoError(t, svc.Start(t.Context()))
	t.Cleanup(func() {
		_ = svc.Stop(t.Context())
	})
	return svc
}

func StartTransportWithConfig(t *testing.T, cfg transport.Config) transport.Service {
	t.Helper()

	ConfigureLoopbackTransport(t)
	svc := transport.New(cfg)
	require.NoError(t, svc.Start(t.Context()))
	t.Cleanup(func() {
		_ = svc.Stop(t.Context())
	})
	return svc
}

func StartBootstrappedTransport(t *testing.T, remote transport.Service) transport.Service {
	t.Helper()

	local := transport.New()
	local.SetBootstrapNodes(remote.Endpoints())
	require.NoError(t, local.Start(t.Context()))
	t.Cleanup(func() {
		_ = local.Stop(t.Context())
	})
	return local
}

func WaitForRelayReadiness(t *testing.T, svc transport.Service) {
	t.Helper()

	WaitForCondition(t, 5*time.Second, "relay peer readiness", func() (bool, string) {
		if svc.RelayPeerCount(transport.DefaultPubsubTopic) > 0 {
			return true, ""
		}
		return false, "relay peers unavailable"
	})
}

func WaitForBootstrapReady(t *testing.T, svc transport.Service) transport.BootstrapStatus {
	t.Helper()

	var status transport.BootstrapStatus
	WaitForCondition(t, 5*time.Second, "transport bootstrap readiness", func() (bool, string) {
		status = svc.BootstrapStatus()
		if status.Joined && status.State == "ready" {
			return true, ""
		}
		return false, status.State
	})
	return status
}

func WaitForTransportDegradedAfterPeerLoss(t *testing.T, svc transport.Service) transport.BootstrapStatus {
	t.Helper()

	var status transport.BootstrapStatus
	WaitForCondition(t, 5*time.Second, "transport degradation after peer loss", func() (bool, string) {
		status = svc.BootstrapStatus()
		if status.State == "degraded" && !status.Joined && svc.State() == "degraded" {
			return true, ""
		}
		return false, status.State + "/" + svc.State()
	})
	return status
}

func WaitForNodeBootstrapReady(t *testing.T, n interface{ Snapshot() nodeapi.Snapshot }) {
	t.Helper()

	WaitForCondition(t, 5*time.Second, "node join bootstrap peer", func() (bool, string) {
		snap := n.Snapshot()
		if snap.Boot.Joined && snap.Boot.State == "ready" && snap.Trans.State == "ready" {
			return true, ""
		}
		return false, snap.Boot.State + "/" + snap.Trans.State
	})
}

func WaitForNodeDegradedAfterPeerLoss(t *testing.T, n interface{ Snapshot() nodeapi.Snapshot }) {
	t.Helper()

	WaitForCondition(t, 5*time.Second, "node degrade after peer loss", func() (bool, string) {
		snap := n.Snapshot()
		if snap.Boot.State != "degraded" || snap.Trans.State != "degraded" {
			return false, snap.Boot.State + "/" + snap.Trans.State
		}
		if snap.Diag.Health.PrimaryReason == nil {
			return false, "missing primary reason"
		}
		if snap.Diag.Health.PrimaryReason.Domain != "boot" && snap.Diag.Health.PrimaryReason.Domain != "transport" {
			return false, snap.Diag.Health.PrimaryReason.Domain
		}
		return true, ""
	})
}

func WaitForNodeBootstrapRecovery(t *testing.T, n interface{ Snapshot() nodeapi.Snapshot }) nodeapi.Snapshot {
	t.Helper()

	return WaitForSnapshot(t, 45*time.Second, n, "node recovery after bootstrap restart", func(snap nodeapi.Snapshot) (bool, string) {
		if snap.Boot.Joined && snap.Boot.State == "ready" && snap.Trans.State == "ready" && snap.Diag.Health.State == "ready" {
			return true, ""
		}
		reason := ""
		if snap.Diag.Health.PrimaryReason != nil {
			reason = snap.Diag.Health.PrimaryReason.Code + ":" + snap.Diag.Health.PrimaryReason.Domain
		}
		return false, snap.Boot.State + "/" + snap.Trans.State + "/" + snap.Diag.Health.State + "/" + reason
	})
}
