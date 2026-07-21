package daemon

import (
	"ardents/internal/diagnostics"
	transport "ardents/internal/network"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCompleteBootUsesDegradedLifecycleWhenHealthIsDegraded(t *testing.T) {
	diag := diagnostics.NewInDir(t.TempDir())
	life := diagnostics.NewMachine()
	require.NoError(t, life.Move(diagnostics.Starting))
	require.NoError(t, life.Move(diagnostics.Initializing))
	diag.SetSubsystem("transport", diagnostics.HealthDegraded, &diagnostics.Reason{
		Code:    "transport.bootstrap.degraded",
		Domain:  "transport",
		Summary: "transport degraded",
	})
	degraded := ""

	next := CompleteBoot(
		diag,
		transport.BootstrapStatus{State: "degraded", Reason: "bootstrap incomplete"},
		func(reason string) { degraded = reason },
		func(state string) { require.NoError(t, life.Move(state)) },
		diag.RetainCurrentHealth,
	)

	require.Equal(t, "bootstrap incomplete", degraded)
	require.Equal(t, diagnostics.Degraded, next)
	require.Equal(t, diagnostics.Degraded, life.State())
}

func TestCompleteBootRetainsHealthWhenReady(t *testing.T) {
	diag := diagnostics.NewInDir(t.TempDir())
	life := diagnostics.NewMachine()
	require.NoError(t, life.Move(diagnostics.Starting))
	require.NoError(t, life.Move(diagnostics.Initializing))

	next := CompleteBoot(
		diag,
		transport.BootstrapStatus{State: "ready"},
		func(string) {},
		func(state string) { require.NoError(t, life.Move(state)) },
		diag.RetainCurrentHealth,
	)

	require.Equal(t, diagnostics.Ready, next)
	require.Equal(t, diagnostics.Ready, life.State())
}
