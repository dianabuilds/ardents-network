package daemon

import (
	"ardents/internal/diagnostics"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadDiagnosticsForStartupAcceptsCleanLedger(t *testing.T) {
	diag := diagnostics.NewInDir(t.TempDir())
	called := false

	ok := LoadDiagnosticsForStartup(diag, func(_, _, _, _, _, _ string) {
		called = true
	})

	require.True(t, ok)
	require.False(t, called)
}
