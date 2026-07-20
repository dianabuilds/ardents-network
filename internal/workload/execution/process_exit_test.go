package execution

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLocalExecutorCapturesNaturalExitCodes(t *testing.T) {
	for _, exitCode := range []int{0, 7} {
		t.Run(fmt.Sprintf("exit_%d", exitCode), func(t *testing.T) {
			executor := NewLocalExecutor()
			workloadID := fmt.Sprintf("work.exit.%d", exitCode)
			prepared, err := executor.Prepare(context.Background(), Request{
				WorkloadID: workloadID,
				Config:     exitingProcessConfig(exitCode),
			})
			require.NoError(t, err)

			_, err = executor.Start(context.Background(), prepared)
			require.NoError(t, err)

			instance := waitForExitedInstance(t, executor, workloadID)
			require.NotNil(t, instance.ExitCode)
			require.Equal(t, exitCode, *instance.ExitCode)
		})
	}
}

func waitForExitedInstance(t *testing.T, executor *LocalExecutor, workloadID string) Instance {
	t.Helper()
	var instance Instance
	require.Eventually(t, func() bool {
		current, err := executor.Inspect(context.Background(), workloadID)
		if err != nil {
			return false
		}
		instance = current
		return !current.Running && current.ExitCode != nil
	}, 3*time.Second, 10*time.Millisecond)
	return instance
}

func exitingProcessConfig(exitCode int) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf(`{"command":"powershell","args":["-NoProfile","-Command","exit %d"]}`, exitCode)
	}
	return fmt.Sprintf(`{"command":"sh","args":["-c","exit %d"]}`, exitCode)
}
