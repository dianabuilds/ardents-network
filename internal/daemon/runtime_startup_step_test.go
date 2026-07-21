package daemon

import (
	"ardents/internal/diagnostics"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunStartupStepUsesStateLoadFailureCode(t *testing.T) {
	diag := diagnostics.NewInDir(t.TempDir())
	var gotCode string

	ok := RunStartupStep(
		diag,
		StartupPhaseStateLoad,
		"node",
		"state",
		false,
		"",
		func(code, _, _, _, _, _ string) { gotCode = code },
		func() error { return errors.New("load failed") },
	)

	require.False(t, ok)
	require.Equal(t, "node.state.load_failed", gotCode)
}

func TestRunStartupStepCompletesOperationOnSuccess(t *testing.T) {
	diag := diagnostics.NewInDir(t.TempDir())

	ok := RunStartupStep(
		diag,
		StartupPhaseIdentity,
		"identity",
		"local",
		false,
		"",
		func(_, _, _, _, _, _ string) {},
		func() error { return nil },
	)

	require.True(t, ok)
	require.Empty(t, diag.PendingOperations())
}
