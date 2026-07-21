package daemon

import (
	"ardents/internal/diagnostics"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDegradeTransportBootstrapMarksTransportAndMovesLifecycle(t *testing.T) {
	diag := diagnostics.NewInDir(t.TempDir())
	life := diagnostics.NewMachine()
	require.NoError(t, life.Move(diagnostics.Starting))
	require.NoError(t, life.Move(diagnostics.Initializing))

	DegradeTransportBootstrap(
		diag,
		"node-a",
		"transport.bootstrap.fetch_failed",
		"bootstrap peer records could not be retrieved",
		"profile tcp_wss, mode steady: fetch failed",
		"remote discovery is incomplete",
		map[string]any{"detail": "fetch failed"},
		func(domain string, state string, reason *diagnostics.Reason) {
			diag.SetPrimary(state, reason)
		},
		func(next string) { require.NoError(t, life.Move(next)) },
	)

	require.Equal(t, diagnostics.Degraded, life.State())
	require.Equal(t, "transport", diag.Health().PrimaryReason.Domain)
	require.Equal(t, "transport.bootstrap.fetch_failed", diag.Health().Subsystems[0].Reason.Code)
}

func TestDegradeDiscoveryImportMarksDiscovery(t *testing.T) {
	diag := diagnostics.NewInDir(t.TempDir())
	life := diagnostics.NewMachine()
	require.NoError(t, life.Move(diagnostics.Starting))
	require.NoError(t, life.Move(diagnostics.Initializing))
	called := ""

	DegradeDiscoveryImport(
		diag,
		"rec-1",
		"bad signature",
		func(detail string) { called = detail },
		func(domain string, state string, reason *diagnostics.Reason) {
			diag.SetPrimary(state, reason)
		},
		func(next string) { require.NoError(t, life.Move(next)) },
	)

	require.Equal(t, "bad signature", called)
	require.Equal(t, diagnostics.Degraded, life.State())
	require.Equal(t, "discovery.bootstrap.import_degraded", diag.Health().Subsystems[0].Reason.Code)
}
