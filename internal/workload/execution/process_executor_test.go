package execution

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWaitForWorkloadStopReturnsErrorWhenProcessStaysRunning(t *testing.T) {
	err := WaitForWorkloadStop(func() (Instance, error) {
		return Instance{WorkloadID: "work.stuck", Running: true}, nil
	}, "work.stuck", time.Now().Add(30*time.Millisecond))
	require.Error(t, err, "expected stop deadline error")

}

func TestWaitForWorkloadStopReturnsInspectErrorWhenStatusUnavailable(t *testing.T) {
	err := WaitForWorkloadStop(func() (Instance, error) {
		return Instance{}, fmt.Errorf("inspect unavailable")
	}, "work.inspect.error", time.Now().Add(30*time.Millisecond))
	require.Error(t, err, "expected inspect status error")
	{

		got := err.Error()
		require.Falsef(t, got == "" || !strings.
			Contains(got, "inspect unavailable"), "expected inspect error to be preserved, got %q", got)
	}

}

func TestProcessRunningTracksStartedProcess(t *testing.T) {
	exec := NewLocalExecutor()
	prepared, err := exec.Prepare(context.Background(), Request{
		WorkloadID: "work.pidprobe",
		Config:     runningProcessConfig(),
	})
	require.NoErrorf(t, err, "prepare: %v", err)

	instance, err := exec.Start(context.Background(), prepared)
	require.NoErrorf(t, err, "start: %v", err)

	t.Cleanup(func() {
		err := exec.Stop(context.Background(), instance)
		require.NoErrorf(t, err, "stop: %v", err)
	})
	require.Truef(t, ProcessRunning(instance.
		PID), "expected running process for pid %d", instance.PID)

}

func runningProcessConfig() string {
	if runtime.GOOS == "windows" {
		return `{"command":"cmd","args":["/c","ping","127.0.0.1","-n","6"]}`
	}
	return `{"command":"sh","args":["-c","sleep 5"]}`
}
