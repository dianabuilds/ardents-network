//go:build linux && text_worker_installed

package state_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// installedCommandNodeLiveness reports only fixture-local node index, process
// terminal class, and the latest fixed lifecycle state. It excludes process
// stderr, addresses, peer identities, and raw terminal errors.
func installedCommandNodeLiveness(t *testing.T, sourcePlan map[string]any) string {
	t.Helper()
	return installedCommandNodeLivenessAt(t, sourcePlan, time.Now())
}

func installedCommandNodeLivenessAt(t *testing.T, sourcePlan map[string]any, observedAt time.Time) string {
	t.Helper()
	processes, present := sourcePlan["diagnostic_node_processes"].([]*nodeProcess)
	if !present || len(processes) == 0 {
		return "none configured"
	}
	directories, present := sourcePlan["route_diagnostic_paths"].([]string)
	if !present || len(directories) != len(processes) {
		return "invalid configured paths"
	}
	states := make([]string, 0, len(processes))
	for index, process := range processes {
		status, exitAge := "running", "active"
		select {
		case <-process.done:
			if process.terminalErr() == nil {
				status, exitAge = "exited-0", installedCommandExitAge(observedAt, process.finished)
			} else {
				status, exitAge = "exited-nonzero", installedCommandExitAge(observedAt, process.finished)
			}
		default:
		}
		state := "unavailable"
		raw, err := os.ReadFile(filepath.Join(directories[index], "lifecycle.json"))
		if err == nil {
			var event struct {
				Kind  string `json:"kind"`
				State string `json:"state"`
			}
			if json.Unmarshal(raw, &event) == nil && event.Kind == "lifecycle" && event.State != "" {
				state = event.State
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			state = "unreadable"
		}
		states = append(states, "node-"+strconv.Itoa(index)+" "+status+" lifecycle="+state+" reason="+installedCommandLifecycleReason(raw)+" resource="+installedCommandResourceState(directories[index])+" exit-age="+exitAge)
	}
	return strings.Join(states, "\n")
}

func installedCommandLifecycleReason(raw []byte) string {
	var event struct {
		State  string `json:"state"`
		Reason string `json:"reason"`
	}
	if json.Unmarshal(raw, &event) != nil || event.State != "FAILED" {
		return "none"
	}
	switch event.Reason {
	case "node listener stopped":
		return "listener-stopped"
	case "Node listener failed":
		return "listener-start-failed"
	case "persistent Network State is unavailable":
		return "state-unavailable"
	case "local role state is unavailable":
		return "role-state-unavailable"
	case "resource pressure evidence is unavailable":
		return "resource-evidence-unavailable"
	case "external evidence channel failed":
		return "evidence-unavailable"
	case "Node role cleanup failed":
		return "role-cleanup-failed"
	default:
		return "other"
	}
}

func installedCommandResourceState(directory string) string {
	raw, err := os.ReadFile(filepath.Join(directory, "resource.json"))
	if errors.Is(err, os.ErrNotExist) {
		return "none"
	}
	if err != nil {
		return "unreadable"
	}
	var event struct {
		Kind  string `json:"kind"`
		State string `json:"state"`
	}
	if json.Unmarshal(raw, &event) != nil || (event.Kind != "resource" && event.Kind != "resource-sample") {
		return "invalid"
	}
	switch event.State {
	case "OBSERVED", "PROTECT", "DRAIN", "EXIT", "NORMAL":
		return event.State
	default:
		return "invalid"
	}
}
func installedCommandExitAge(observedAt, finished time.Time) string {
	if finished.IsZero() {
		return "unknown"
	}
	switch elapsed := observedAt.Sub(finished); {
	case elapsed < 30*time.Second:
		return "under-30s"
	case elapsed < 2*time.Minute:
		return "under-2m"
	case elapsed < 5*time.Minute:
		return "under-5m"
	default:
		return "5m-or-more"
	}
}
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
