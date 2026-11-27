//go:build linux && node_process_native

package processfixture

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// WriteOperationMarker is shared by the original process and its independent native workload driver.
func WriteOperationMarker(t *testing.T, root, name string) {
	t.Helper()
	if err := writeOperationMarker(root, name); err != nil {
		t.Fatal(err)
	}
}

func writeOperationMarker(root, name string) error {
	return os.WriteFile(filepath.Join(root, name), []byte("1"), 0600)
}

// WaitOperationMarker is shared by the original process and its independent native workload driver.
func WaitOperationMarker(t *testing.T, root, name string, limit time.Duration) {
	t.Helper()
	deadline := time.NewTimer(limit)
	defer deadline.Stop()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			return
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		select {
		case <-ticker.C:
		case <-deadline.C:
			t.Fatal("native operation marker unavailable", name)
		case <-t.Context().Done():
			t.Fatal(t.Context().Err())
		}
	}
}

// IssuerStorageFault is shared by the original process and its independent native workload driver.
func IssuerStorageFault(mode string) bool {
	return mode == "quota-uncertain" || mode == "result-uncertain"
}

// DescriptorRecordCount is shared by the original process and its independent native workload driver.
func DescriptorRecordCount(t *testing.T, root string) int {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(root, "records"))
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && len(entry.Name()) == 64 {
			count++
		}
	}
	return count
}

// IntroductionRecordCount is shared by the original process and its independent native workload driver.
func IntroductionRecordCount(t *testing.T, root string) int {
	t.Helper()
	snapshot, err := os.ReadFile(filepath.Join(root, "closed-introduction-slots"))
	if err != nil || len(snapshot) < 120 || string(snapshot[:8]) != "ARDISL01" || (len(snapshot)-120)%40 != 0 {
		t.Fatal("native History snapshot unavailable", err)
	}
	return (len(snapshot) - 120) / 40
}

// StartOperationProcess starts a pinned Node test and joins it on early return.
func StartOperationProcess(t testing.TB, unit, testName, fixtureEnv, plan, signals string, runtimeSeconds int) *NativeProcess {
	t.Helper()
	command := OwnerTestCommand(t, unit, PinnedOwnerTest(t), testName, fixtureEnv, plan, runtimeSeconds)
	process := &NativeProcess{Unit: unit, Command: command}
	command.Stdout, command.Stderr = &process.Diagnostic, &process.Diagnostic
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if !process.Joined {
			for _, marker := range []string{"release", "stop"} {
				if err := writeOperationMarker(signals, marker); err != nil {
					t.Error("release original test process", err)
				}
			}
			if err := process.Wait(); err != nil {
				// Wait can join only systemd-run after its context is canceled.
				// Stop the unit as well so it cannot survive a failed workload.
				stopErr := exec.Command("systemctl", "stop", unit).Run()
				t.Error("original test process cleanup", errors.Join(err, stopErr))
			}
		}
		if t.Failed() {
			t.Log("original test diagnostics", process.Diagnostic.String())
		}
	})
	return process
}
