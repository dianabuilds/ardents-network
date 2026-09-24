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
)

// installedCommandNodeLiveness reports only fixture-local node index, process
// terminal class, and the latest fixed lifecycle state. It excludes process
// stderr, addresses, peer identities, and raw terminal errors.
func installedCommandNodeLiveness(t *testing.T, sourcePlan map[string]any) string {
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
		status := "running"
		select {
		case <-process.done:
			if process.terminalErr() == nil {
				status = "exited-0"
			} else {
				status = "exited-nonzero"
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
		states = append(states, "node-"+strconv.Itoa(index)+" "+status+" lifecycle="+state)
	}
	return strings.Join(states, "\n")
}

func TestInstalledCommandNodeLivenessReportsFixedSnapshot(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "lifecycle.json"), []byte(`{"kind":"lifecycle","state":"READY","reason":"ignored"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	process := &nodeProcess{done: make(chan struct{}), waitErr: errors.New("private terminal error")}
	close(process.done)
	got := installedCommandNodeLiveness(t, map[string]any{
		"diagnostic_node_processes": []*nodeProcess{process},
		"route_diagnostic_paths":    []string{directory},
	})
	if got != "node-0 exited-nonzero lifecycle=READY" {
		t.Fatalf("liveness = %q", got)
	}
}
