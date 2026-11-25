//go:build linux && endpoint_launch_native

package endpoint

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

const latencyEndpointUnit = "ardents-endpoint.service"

// StartLatencyUnit replaces the staged unit in the test-owned stand and
// registers the participant's stop and fixture removal before starting it.
func StartLatencyUnit(t testing.TB, unit, participantPath string) {
	t.Helper()
	const unitPath = "/etc/systemd/system/ardents-endpoint.service"
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		if output, err := exec.CommandContext(ctx, "systemctl", "stop", latencyEndpointUnit).CombinedOutput(); err != nil {
			t.Errorf("stop latency participant: %v: %s", err, output)
		}
		for _, path := range []string{unitPath, participantPath} {
			if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("remove latency fixture %s: %v", path, err)
			}
		}
		if output, err := exec.CommandContext(ctx, "systemctl", "daemon-reload").CombinedOutput(); err != nil {
			t.Errorf("reload manager after latency cleanup: %v: %s", err, output)
		}
		_ = exec.CommandContext(ctx, "systemctl", "reset-failed", latencyEndpointUnit).Run()
	})
	if err := os.MkdirAll(filepath.Dir(unitPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(unitPath, []byte(unit), 0644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.CommandContext(t.Context(), "systemctl", "daemon-reload").CombinedOutput(); err != nil {
		t.Fatalf("reload latency unit: %v: %s", err, out)
	}
	if out, err := exec.CommandContext(t.Context(), "systemctl", "start", latencyEndpointUnit).CombinedOutput(); err != nil {
		t.Fatalf("cold/warm participant start: %v: %s", err, out)
	}
}

// WaitLatencyReport observes the atomically published component report within
// the native driver budget and checks for early exit on the real manager.
func WaitLatencyReport(t testing.TB, control string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
	defer cancel()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	resultPath := filepath.Join(control, "result.json")
	for {
		data, err := os.ReadFile(resultPath)
		switch {
		case err == nil:
			return data
		case !errors.Is(err, fs.ErrNotExist):
			t.Fatalf("read cold/warm report: %v", err)
		}
		if current := StartupIdentity(t, ctx); current[2] != "active" && current[0] == "0" {
			// The report may have appeared between the first read and the
			// manager's observation of process exit.
			if data, err := os.ReadFile(resultPath); err == nil {
				return data
			} else {
				t.Fatalf("cold/warm participant exited without a readable verdict: %+v: %v", current, err)
			}
		}
		select {
		case <-ctx.Done():
			t.Fatalf("cold/warm participant did not report a verdict: %v", ctx.Err())
		case <-ticker.C:
		}
	}
}
