package state_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

const installedCommandPrivateLifecycleDirectory = "ARDENTS_E2E_PRIVATE_NODE_LIFECYCLE_DIR"

// installedCommandNodeLiveness reports only fixture-local node index, process
// terminal class, and the latest fixed lifecycle state. It excludes process
// stderr, addresses, peer identities, and raw terminal errors.
func installedCommandNodeLiveness(t *testing.T, sourcePlan map[string]any) string {
	t.Helper()
	return installedCommandNodeLivenessAt(t, sourcePlan, time.Now())
}

// installedCommandExitedNodeLiveness returns the fixed snapshot only when a
// prepared Node has already exited. It lets setup failures use the same
// privacy-safe evidence path as participant-stage failures.
func installedCommandExitedNodeLiveness(t *testing.T, sourcePlan map[string]any) string {
	t.Helper()
	processes, present := sourcePlan["diagnostic_node_processes"].([]*nodeProcess)
	if !present || len(processes) == 0 {
		return "none configured"
	}
	for _, process := range processes {
		select {
		case <-process.done:
			return installedCommandNodeLiveness(t, sourcePlan)
		default:
		}
	}
	return ""
}

// installedCommandCapturePrivateNodeLifecycles retains failed lifecycle
// records only when the installed VM explicitly supplies a private, root-only
// evidence directory. It never returns record content to the test output.
func installedCommandCapturePrivateNodeLifecycles(t *testing.T, sourcePlan map[string]any) string {
	t.Helper()
	directory := os.Getenv(installedCommandPrivateLifecycleDirectory)
	if directory == "" {
		return "not-configured"
	}
	if !filepath.IsAbs(directory) || filepath.Clean(directory) != directory {
		return "unavailable"
	}
	info, err := os.Lstat(directory)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || runtime.GOOS == "linux" && info.Mode().Perm()&0o077 != 0 {
		return "unavailable"
	}
	directories, present := sourcePlan["route_diagnostic_paths"].([]string)
	if !present {
		return "unavailable"
	}
	processes, present := sourcePlan["diagnostic_node_processes"].([]*nodeProcess)
	if present && len(processes) != len(directories) {
		return "unavailable"
	}
	captured := 0
	for index, source := range directories {
		raw, readErr := os.ReadFile(filepath.Join(source, "lifecycle.json"))
		if readErr != nil || !installedCommandFailedLifecycle(raw) {
			continue
		}
		if !installedCommandWritePrivateNodeEvidence(directory, index, "lifecycle.json", raw) {
			return "unavailable"
		}
		if present {
			select {
			case <-processes[index].done:
				if !installedCommandWritePrivateNodeEvidence(directory, index, "stderr", processes[index].stderr.Bytes()) {
					return "unavailable"
				}
			default:
			}
		}
		captured++
	}
	return "captured-" + strconv.Itoa(captured)
}

func installedCommandWritePrivateNodeEvidence(directory string, index int, suffix string, content []byte) bool {
	name := filepath.Join(directory, "node-"+strconv.Itoa(index)+"-"+suffix)
	file, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return false
	}
	written, err := file.Write(content)
	err = errors.Join(err, file.Sync(), file.Close())
	return err == nil && written == len(content)
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
	return installedCommandLifecycleFailureReason(event.Reason)
}

func installedCommandFailedLifecycle(raw []byte) bool {
	var event struct {
		Kind  string `json:"kind"`
		State string `json:"state"`
	}
	return json.Unmarshal(raw, &event) == nil && event.Kind == "lifecycle" && event.State == "FAILED"
}

// installedCommandLifecycleFailureReason maps only fixed Node lifecycle
// reasons written by the installed command. It never returns the underlying
// reason because that can contain State or local-host detail.
func installedCommandLifecycleFailureReason(reason string) string {
	switch {
	case reason == "node listener stopped":
		return "listener-stopped"
	case reason == "Node listener failed":
		return "listener-start-failed"
	case reason == "persistent Network State is unavailable":
		return "state-unavailable"
	case reason == "local role state is unavailable":
		return "role-state-unavailable"
	case reason == "local Node identity or key does not match verified state":
		return "identity-mismatch"
	case reason == "role-probe listener does not match the accepted Node Record":
		return "listener-record-mismatch"
	case reason == "assignment changed during quarantine":
		return "assignment-changed"
	case strings.HasPrefix(reason, "assignment lost readiness during quarantine:"):
		return "assignment-readiness-lost"
	case strings.HasPrefix(reason, "closed Route State is unavailable:"):
		return "closed-route-state-unavailable"
	case strings.HasPrefix(reason, "resource placement is not ready:"):
		return "placement-unavailable"
	case reason == "closed Route assignment is not locally implemented":
		return "closed-route-assignment-unavailable"
	case reason == "profile or deterministic assignment is inactive":
		return "assignment-inactive"
	case reason == "freshness or validity is not satisfied":
		return "time-validity-unavailable"
	case reason == "freshness, validity, or terminal duty bound is not satisfied":
		return "time-duty-bound-unavailable"
	case reason == "external evidence channel failed":
		return "evidence-unavailable"
	case reason == "Node role cleanup failed":
		return "role-cleanup-failed"
	case reason == "shutdown before assignment admission":
		return "shutdown-before-admission"
	case strings.HasPrefix(reason, "resource pressure evidence is unavailable: "):
		return installedCommandResourceFailureReason(reason)
	default:
		return "other"
	}
}

func installedCommandResourceFailureReason(reason string) string {
	switch reason {
	case "resource pressure evidence is unavailable: canceled",
		"resource pressure evidence is unavailable: deadline",
		"resource pressure evidence is unavailable: hosting-lock",
		"resource pressure evidence is unavailable: hosting-state",
		"resource pressure evidence is unavailable: hosting-observation",
		"resource pressure evidence is unavailable: hosting-interface",
		"resource pressure evidence is unavailable: owner-cgroup",
		"resource pressure evidence is unavailable: other":
		return strings.TrimPrefix(reason, "resource pressure evidence is unavailable: ")
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

func TestInstalledCommandExitedNodeLivenessReportsFixedSnapshot(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "lifecycle.json"), []byte(`{"kind":"lifecycle","state":"FAILED","reason":"local Node identity or key does not match verified state"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	process := &nodeProcess{done: make(chan struct{}), waitErr: errors.New("private terminal error"), finished: time.Now().Add(-time.Second)}
	close(process.done)
	got := installedCommandExitedNodeLiveness(t, map[string]any{
		"diagnostic_node_processes": []*nodeProcess{process},
		"route_diagnostic_paths":    []string{directory},
	})
	if !strings.Contains(got, "node-0 exited-nonzero lifecycle=FAILED reason=identity-mismatch") {
		t.Fatalf("liveness = %q", got)
	}
}

func TestInstalledCommandPrivateLifecycleCaptureRetainsOnlyFailedRecords(t *testing.T) {
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "lifecycle.json"), []byte(`{"kind":"lifecycle","state":"FAILED","reason":"private lifecycle detail"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(destination, 0o700); err != nil {
		t.Fatal(err)
	}
	ready := t.TempDir()
	if err := os.WriteFile(filepath.Join(ready, "lifecycle.json"), []byte(`{"kind":"lifecycle","state":"READY"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	failedProcess := &nodeProcess{done: make(chan struct{}), stderr: bytes.NewBufferString("private stderr detail")}
	close(failedProcess.done)
	t.Setenv(installedCommandPrivateLifecycleDirectory, destination)
	if got := installedCommandCapturePrivateNodeLifecycles(t, map[string]any{
		"route_diagnostic_paths":    []string{source, ready},
		"diagnostic_node_processes": []*nodeProcess{failedProcess, {done: make(chan struct{})}},
	}); got != "captured-1" {
		t.Fatalf("capture = %q", got)
	}
	raw, err := os.ReadFile(filepath.Join(destination, "node-0-lifecycle.json"))
	if err != nil || string(raw) != `{"kind":"lifecycle","state":"FAILED","reason":"private lifecycle detail"}` {
		t.Fatalf("private lifecycle record = %q, %v", raw, err)
	}
	if _, err := os.Stat(filepath.Join(destination, "node-1-lifecycle.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("READY lifecycle was captured: %v", err)
	}
	stderr, err := os.ReadFile(filepath.Join(destination, "node-0-stderr"))
	if err != nil || string(stderr) != "private stderr detail" {
		t.Fatalf("private stderr = %q, %v", stderr, err)
	}
}
