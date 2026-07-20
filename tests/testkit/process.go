package testkit

import (
	"encoding/json"
	"fmt"
	"runtime"
	"testing"

	workloadexecution "ardents/internal/workload/execution"
)

func HelperProcessConfig(t *testing.T, mode string) string {
	t.Helper()
	return HelperProcessConfigWithMarker(t, mode, "")
}

func HelperProcessConfigWithMarker(t *testing.T, mode, marker string) string {
	t.Helper()
	var command string
	var args []string
	switch mode {
	case "sleep":
		if runtime.GOOS == "windows" {
			command = "powershell"
			script := "Start-Sleep -Seconds 30"
			if marker != "" {
				script = fmt.Sprintf("$ardentsMarker = %q; %s", marker, script)
			}
			args = []string{"-NoProfile", "-Command", script}
		} else {
			command = "sh"
			script := "sleep 30"
			if marker != "" {
				script = fmt.Sprintf("ARDENTS_MARKER=%q; export ARDENTS_MARKER; %s", marker, script)
			}
			args = []string{"-c", script}
		}
	default:
		t.Fatalf("unsupported helper mode %q", mode)
	}
	raw, err := json.Marshal(map[string]any{
		"command": command,
		"args":    args,
	})
	if err != nil {
		t.Fatalf("marshal helper config: %v", err)
	}
	return string(raw)
}

func ProcessRunning(t *testing.T, pid int) bool {
	t.Helper()
	return workloadexecution.ProcessRunning(pid)
}
