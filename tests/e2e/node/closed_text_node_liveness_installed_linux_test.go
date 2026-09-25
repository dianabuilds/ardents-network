//go:build linux && text_worker_installed

package state_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestInstalledCommandNodeLivenessReportsFixedSnapshot(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "lifecycle.json"), []byte(`{"kind":"lifecycle","state":"FAILED","reason":"node listener stopped"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	process := &nodeProcess{done: make(chan struct{}), waitErr: errors.New("private terminal error"), finished: time.Unix(100, 0)}
	close(process.done)
	got := installedCommandNodeLivenessAt(t, map[string]any{
		"diagnostic_node_processes": []*nodeProcess{process},
		"route_diagnostic_paths":    []string{directory},
	}, time.Unix(110, 0))
	if got != "node-0 exited-nonzero lifecycle=FAILED reason=listener-stopped resource=none exit-age=under-30s" {
		t.Fatalf("liveness = %q", got)
	}
}

func TestInstalledCommandLifecycleFailureReasonIsFixed(t *testing.T) {
	for _, test := range []struct {
		reason string
		want   string
	}{
		{"local Node identity or key does not match verified state", "identity-mismatch"},
		{"assignment lost readiness during quarantine: private State detail", "assignment-readiness-lost"},
		{"closed Route State is unavailable: private State detail", "closed-route-state-unavailable"},
		{"resource placement is not ready: private host detail", "placement-unavailable"},
		{"resource pressure evidence is unavailable: hosting-lock", "hosting-lock"},
		{"resource pressure evidence is unavailable: owner-resident-process-churn", "owner-resident-process-churn"},
		{"private failure detail", "other"},
	} {
		t.Run(test.want, func(t *testing.T) {
			if got := installedCommandLifecycleFailureReason(test.reason); got != test.want {
				t.Fatalf("reason %q = %q, want %q", test.reason, got, test.want)
			}
		})
	}
}
