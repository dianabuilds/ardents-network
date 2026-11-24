//go:build linux && node_process_native

package node

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

// PinnedCommand verifies the original executable supplied by the native runner.
// The root driver runs outside the measured cgroups.
func PinnedCommand(t testing.TB) string {
	t.Helper()
	path := os.Getenv("ARDENTS_NODE_NATIVE_COMMAND")
	want := os.Getenv("ARDENTS_NODE_NATIVE_COMMAND_SHA256")
	if os.Geteuid() != 0 {
		t.Fatal("selected Node native root driver and pinned command unavailable")
	}
	if err := verifyPinnedCommand(path, want); err != nil {
		t.Fatal(err)
	}
	return path
}

func verifyPinnedCommand(path, want string) error {
	if !filepath.IsAbs(path) || len(want) != 64 {
		return errors.New("selected Node native pinned command unavailable")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(raw)
	if hex.EncodeToString(hash[:]) != want {
		return errors.New("selected native command digest differs")
	}
	return nil
}

// DiagnosticBuffer captures an original child diagnostic during concurrent reads.
type DiagnosticBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (diagnostic *DiagnosticBuffer) Write(raw []byte) (int, error) {
	diagnostic.mu.Lock()
	defer diagnostic.mu.Unlock()
	return diagnostic.buffer.Write(raw)
}
func (diagnostic *DiagnosticBuffer) String() string {
	diagnostic.mu.Lock()
	defer diagnostic.mu.Unlock()
	return diagnostic.buffer.String()
}

// NativeProcess tracks a systemd-run child, its original MainPID and joined result.
type NativeProcess struct {
	Unit       string
	Command    *exec.Cmd
	Stdout     io.ReadCloser
	Diagnostic DiagnosticBuffer
	PID        int
	Joined     bool
	Signaled   bool
}

// Stop joins and checks the original measured process.
func (process *NativeProcess) Stop(t *testing.T, refusedExchange ...bool) {
	t.Helper()
	if process.Joined {
		return
	}
	if !process.Signaled {
		if err := exec.Command("systemctl", "kill", "--kill-whom=main", "--signal=SIGINT", process.Unit).Run(); err != nil {
			t.Fatal("original Node unavailable before explicit stop", err, process.Diagnostic.String())
		}
		process.Signaled = true
	}
	err := process.Command.Wait()
	process.Joined = true
	_ = process.Stdout.Close()
	var exit *exec.ExitError
	var event map[string]string
	decodeErr := json.NewDecoder(bytes.NewBufferString(process.Diagnostic.String())).Decode(&event)
	if !errors.As(err, &exit) || decodeErr != nil || event["phase"] != "joined" {
		t.Fatal("original Node did not report known joined retirement", err, decodeErr, process.Diagnostic.String())
	}
	if exit.ExitCode() != 130 || event["outcome"] != "canceled" {
		// The deliberate pre-spend refusal can leave an actual Route physical
		// cleanup failure. It must remain a failed joined result, never EXIT or
		// clean cancellation. All positive cancellation rows still require 130.
		if len(refusedExchange) == 0 || !refusedExchange[0] || exit.ExitCode() != 1 || event["outcome"] != "failed" || event["first_owner"] != "caller" || event["cleanup_owner"] != "route" {
			t.Fatal("original Node did not retain the expected cancellation or refusal cleanup", err, process.Diagnostic.String())
		}
	}
	if _, err := os.Stat(fmt.Sprintf("/proc/%d", process.PID)); !os.IsNotExist(err) {
		t.Fatal("original Node process remains after manager join", err)
	}
}

// FailedJoin joins and checks the original measured process.
func (process *NativeProcess) FailedJoin(t *testing.T, expectedOwner string) {
	t.Helper()
	err := process.Command.Wait()
	process.Joined = true
	_ = process.Stdout.Close()
	var exit *exec.ExitError
	var event map[string]string
	decodeErr := json.Unmarshal([]byte(process.Diagnostic.String()), &event)
	if !errors.As(err, &exit) || exit.ExitCode() != 1 || decodeErr != nil || event["phase"] != "joined" || event["outcome"] != "failed" || event["first_owner"] != expectedOwner {
		t.Fatal("original Node did not retain failed physical completion", err, process.Diagnostic.String())
	}
	if expectedOwner != "" {
		var detail struct {
			Kind, Phase, Cause string
			Resource           json.RawMessage
		}
		if err := json.Unmarshal([]byte(event["first_failure"]), &detail); err != nil || detail.Kind != "logical" || detail.Phase == "" || detail.Cause == "" {
			t.Fatal("original local first cause unavailable after actual join", err, process.Diagnostic.String())
		}
		if expectedOwner == "process" && (len(detail.Resource) == 0 || string(detail.Resource) == "null") {
			t.Fatal("original process refusal lost its actual measurement context", process.Diagnostic.String())
		}
	}
	if _, err := os.Stat(fmt.Sprintf("/proc/%d", process.PID)); !os.IsNotExist(err) {
		t.Fatal("failed original Node remains after manager join", err)
	}
}

// Launch starts the exact requested systemd child and registers joined cleanup.
func Launch(t testing.TB, unit, diagnosticLabel, stopErrorLabel string, args ...string) *NativeProcess {
	t.Helper()
	process := &NativeProcess{Unit: unit}
	prefix := []string{"--quiet", "--wait", "--pipe", "--collect", "--unit=" + unit}
	process.Command = exec.CommandContext(t.Context(), "systemd-run", append(prefix, args...)...)
	process.Command.Stderr = &process.Diagnostic
	var err error
	process.Stdout, err = process.Command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := process.Command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if !process.Joined {
			err := exec.Command("systemctl", "stop", unit).Run()
			if err != nil && stopErrorLabel != "" {
				t.Error(stopErrorLabel, err)
			}
			_ = process.Command.Wait()
			_ = process.Stdout.Close()
			process.Joined = true
		}
		if t.Failed() {
			t.Log(diagnosticLabel, process.Diagnostic.String())
		}
	})
	return process
}
