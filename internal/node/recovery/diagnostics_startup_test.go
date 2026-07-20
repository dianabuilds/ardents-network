package recovery_test

import (
	"testing"

	"ardents/internal/diagnostics"
	noderecovery "ardents/internal/node/recovery"

	"github.com/stretchr/testify/require"
)

func TestLoadDiagnosticsForStartupAcceptsCleanLedger(t *testing.T) {
	diag := diagnostics.NewInDir(t.TempDir())
	called := false

	ok := noderecovery.LoadDiagnosticsForStartup(diag, func(_, _, _, _, _, _ string) {
		called = true
	})

	require.True(t, ok)
	require.False(t, called)
}
