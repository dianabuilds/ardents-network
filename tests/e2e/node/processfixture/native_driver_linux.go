//go:build linux && node_process_native

package processfixture

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

// Wait joins the systemd-run process and records that cleanup no longer owns it.
// exec.Cmd.Wait closes the caller's stdout pipe after the process exits.
func (process *NativeProcess) Wait() error {
	err := process.Command.Wait()
	process.Joined = true
	return err
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
	err := process.Wait()
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
	err := process.Wait()
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
			_ = process.Wait()
		}
		if t.Failed() {
			t.Log(diagnosticLabel, process.Diagnostic.String())
		}
	})
	return process
}

// LaunchFreshRole starts the original role in its dedicated native unit.
func LaunchFreshRole(t testing.TB, role, binary, plan string, index byte) *NativeProcess {
	t.Helper()
	unit := fmt.Sprintf("ardents-%s-%d-%d.service", role, os.Getpid(), index)
	if role == "issuer" {
		unit = fmt.Sprintf("ardents-issuer-%d.service", os.Getpid())
	}
	return launchRole(t, unit, "original "+role+" diagnostic", binary, role, "--config", plan)
}

// StartFreshRole starts a fresh role and waits for its listening report.
func StartFreshRole(t testing.TB, role, binary, plan string, index byte) *NativeProcess {
	t.Helper()
	return waitForRole(t, LaunchFreshRole(t, role, binary, plan, index), role)
}

// LaunchRetainedRole starts an original Node process with the retained profile.
func LaunchRetainedRole(t testing.TB, role, binary, plan string, index byte) *NativeProcess {
	t.Helper()
	unit := fmt.Sprintf("ardents-%s-retained-%d-%d.service", role, os.Getpid(), index)
	return launchRole(t, unit, "retained "+role+" original diagnostic", binary, role, "--retained-config", plan)
}

func launchRole(t testing.TB, unit, diagnosticLabel, binary, role, flag, plan string) *NativeProcess {
	t.Helper()
	return Launch(t, unit, diagnosticLabel, "",
		"--property=CPUQuota=100%", "--property=CPUQuotaPeriodSec=100ms", "--property=MemoryMax=1G", "--property=MemoryHigh=768M", "--property=TasksMax=256", "--property=LimitNOFILE=1024", "--property=RuntimeMaxSec=120s",
		"--setenv=GOMAXPROCS=1", "--setenv=GOMEMLIMIT=512MiB", "--", binary, "node", role, flag, plan)
}

// StartRetainedRole waits for the listening report and records the original PID.
func StartRetainedRole(t testing.TB, role, binary, plan string, index byte) *NativeProcess {
	t.Helper()
	return waitForRole(t, LaunchRetainedRole(t, role, binary, plan, index), role)
}

func waitForRole(t testing.TB, process *NativeProcess, role string) *NativeProcess {
	t.Helper()
	var event map[string]string
	if err := json.NewDecoder(process.Stdout).Decode(&event); err != nil || event["operation"] != "node."+role || event["phase"] != "listening" {
		t.Fatal(role+" startup refused", event, err, process.Diagnostic.String())
	}
	raw, err := exec.CommandContext(t.Context(), "systemctl", "show", process.Unit, "-p", "MainPID", "--value").Output()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Sscanf(string(raw), "%d", &process.PID); err != nil || process.PID < 2 {
		t.Fatal(role+" MainPID unavailable", err)
	}
	return process
}

// JoinRetainedRole signals (or waits for a self-refusing invocation) and
// joins the actual command, returning the exit code and the stdout report.
func JoinRetainedRole(t testing.TB, p *NativeProcess, signal string) (int, map[string]string) {
	t.Helper()
	if signal != "" {
		if err := exec.Command("systemctl", "kill", "--kill-whom=main", "--signal="+signal, p.Unit).Run(); err != nil {
			t.Fatal("retained signal", signal, err)
		}
	}
	stdout, readErr := io.ReadAll(p.Stdout)
	err := p.Wait()
	if readErr != nil {
		t.Fatal("retained stdout", readErr)
	}
	code := 0
	var exited *exec.ExitError
	if err != nil {
		if !errors.As(err, &exited) {
			t.Fatal("retained wait", err)
		}
		code = exited.ExitCode()
	}
	var event map[string]string
	if raw := bytes.TrimSpace(stdout); len(raw) > 0 {
		if json.Unmarshal(raw, &event) != nil {
			t.Fatal("retained stdout report", string(raw))
		}
	}
	return code, event
}

// RequireStartupFailure checks the finite first-cause report of a refused startup.
func RequireStartupFailure(t testing.TB, raw string) {
	t.Helper()
	var fact struct {
		Kind, Phase, Cause string
		ElapsedMS          *int64 `json:"elapsed_ms"`
	}
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&fact); err != nil || fact.Kind != "logical" || fact.Phase != "startup" || fact.Cause != "owner-refusal" || fact.ElapsedMS == nil || *fact.ElapsedMS < 0 {
		t.Fatal("partial startup lost closed original first-cause projection", raw, err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		t.Fatal("startup diagnostic contains trailing data", err)
	}
}
