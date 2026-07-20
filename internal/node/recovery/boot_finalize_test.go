package recovery_test

import (
	"testing"

	"ardents/internal/diagnostics"
	transport "ardents/internal/network/api"
	nodelifecycle "ardents/internal/node/lifecycle"
	noderecovery "ardents/internal/node/recovery"

	"github.com/stretchr/testify/require"
)

func TestCompleteBootUsesDegradedLifecycleWhenHealthIsDegraded(t *testing.T) {
	diag := diagnostics.NewInDir(t.TempDir())
	life := nodelifecycle.NewMachine()
	require.NoError(t, life.Move(nodelifecycle.Starting))
	require.NoError(t, life.Move(nodelifecycle.Initializing))
	diag.SetSubsystem("transport", diagnostics.HealthDegraded, &diagnostics.Reason{
		Code:    "transport.bootstrap.degraded",
		Domain:  "transport",
		Summary: "transport degraded",
	})
	degraded := ""

	next := noderecovery.CompleteBoot(
		diag,
		transport.BootstrapStatus{State: "degraded", Reason: "bootstrap incomplete"},
		func(reason string) { degraded = reason },
		func(state string) { _ = life.Move(state) },
		diag.RetainCurrentHealth,
	)

	require.Equal(t, "bootstrap incomplete", degraded)
	require.Equal(t, nodelifecycle.Degraded, next)
	require.Equal(t, nodelifecycle.Degraded, life.State())
}

func TestCompleteBootRetainsHealthWhenReady(t *testing.T) {
	diag := diagnostics.NewInDir(t.TempDir())
	life := nodelifecycle.NewMachine()
	require.NoError(t, life.Move(nodelifecycle.Starting))
	require.NoError(t, life.Move(nodelifecycle.Initializing))

	next := noderecovery.CompleteBoot(
		diag,
		transport.BootstrapStatus{State: "ready"},
		func(string) {},
		func(state string) { _ = life.Move(state) },
		diag.RetainCurrentHealth,
	)

	require.Equal(t, nodelifecycle.Ready, next)
	require.Equal(t, nodelifecycle.Ready, life.State())
}
