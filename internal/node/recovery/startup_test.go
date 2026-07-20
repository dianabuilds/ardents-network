package recovery_test

import (
	"errors"
	"testing"

	"ardents/internal/diagnostics"
	nodelifecycle "ardents/internal/node/lifecycle"
	noderecovery "ardents/internal/node/recovery"

	"github.com/stretchr/testify/require"
)

func TestRunStartupStepUsesStateLoadFailureCode(t *testing.T) {
	diag := diagnostics.NewInDir(t.TempDir())
	var gotCode string

	ok := noderecovery.RunStartupStep(
		diag,
		nodelifecycle.StartupPhaseStateLoad,
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

	ok := noderecovery.RunStartupStep(
		diag,
		nodelifecycle.StartupPhaseIdentity,
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
