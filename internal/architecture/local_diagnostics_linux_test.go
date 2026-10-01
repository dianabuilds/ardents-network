package architecture

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestLocalDiagnosticsCollector(t *testing.T) {
	root := repositoryRoot(t)
	args := []string{"test"}
	for _, name := range []string{"diagnostic-command.go", "diagnostic-capture.go", "diagnostic-view.go", "diagnostic-report.go", "diagnostic-monitor.go", "diagnostic-monitor-view.go", "diagnostic-monitor-collector.go", "diagnostic-monitor_test.go", "diagnostic-log-retention.go", "diagnostic-log-retention_test.go", "diagnostic-capture_test.go", "diagnostic-report_test.go", "diagnostic-evidence.go", "diagnostic-evidence_test.go"} {
		args = append(args, filepath.Join(root, "scripts", "diagnostics", name))
	}
	args = append(args, "-count=1", "-timeout=1m")
	command := exec.Command("go", args...)
	command.Dir = root
	if body, err := command.CombinedOutput(); err != nil {
		t.Fatalf("local diagnostic collector: %v\n%s", err, body)
	}
}
