//go:build linux

package endpoint_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

// liveEnrolledEndpoint owns one endpoint process used by the alpha-control
// process tests and reaps both its process and stderr drain.
type liveEnrolledEndpoint struct {
	command        *exec.Cmd
	scanner        *bufio.Scanner
	stderr         *processStderrBuffer
	attachment     string
	releaseOutcome string
	cancel         context.CancelFunc
	finished       bool
}

type endpointLifecycleEvent struct {
	Kind, State, Outcome, Cohort, Release string
	Attachment                            string
}

func startLiveEnrolledEndpoint(t *testing.T, artifact, input, root, expectedCohort, expectedRelease string) *liveEnrolledEndpoint {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	bundleRoot, manifestPin := manifestPinnedEnrollmentArguments(t, input)
	running := &liveEnrolledEndpoint{command: exec.CommandContext(ctx, artifact, "endpoint", "enroll", bundleRoot, manifestPin), cancel: cancel}
	running.command.WaitDelay = time.Second
	running.command.Env = endpointEnvironment(root)
	running.stderr = &processStderrBuffer{}
	running.command.Stderr = running.stderr
	stdout, err := running.command.StdoutPipe()
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	if err := running.command.Start(); err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := running.cleanup(); err != nil {
			t.Errorf("fresh Endpoint fallback cleanup: %v", err)
		}
	})
	running.scanner = bufio.NewScanner(stdout)
	events := [3]endpointLifecycleEvent{}
	for index := range events {
		if !running.scanner.Scan() {
			t.Fatalf("fresh Endpoint startup event %d is absent: %v; stderr=%s", index, running.scanner.Err(), running.stderr.String())
		}
		if err := json.Unmarshal(running.scanner.Bytes(), &events[index]); err != nil {
			t.Fatalf("decode fresh Endpoint startup event %d: %v; line=%q", index, err, running.scanner.Text())
		}
	}
	if !validFreshEndpointStartup(events, expectedCohort, expectedRelease) {
		t.Fatalf("fresh Endpoint startup events = %+v; stderr=%s", events, running.stderr.String())
	}
	running.attachment = events[2].Attachment
	running.releaseOutcome = events[1].Outcome
	if info, err := os.Lstat(running.attachment); err != nil || info.Mode()&os.ModeSocket == 0 {
		t.Fatalf("fresh Endpoint attachment = %v / %v", info, err)
	}
	return running
}

func validFreshEndpointStartup(events [3]endpointLifecycleEvent, expectedCohort, expectedRelease string) bool {
	return events[0].Kind == "endpoint-lifecycle" && events[0].State == "starting" &&
		events[1].Kind == "release-decision" && events[1].Outcome == "release-accepted" &&
		events[1].Cohort == expectedCohort && events[1].Release == expectedRelease &&
		events[2].Kind == "endpoint-lifecycle" && events[2].State == "ready" && events[2].Attachment != ""
}

func (running *liveEnrolledEndpoint) stop(t *testing.T) {
	t.Helper()
	if err := running.command.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	if !running.scanner.Scan() {
		t.Fatalf("fresh Endpoint did not report stopped: %v; stderr=%s", running.scanner.Err(), running.stderr.String())
	}
	var stopped endpointLifecycleEvent
	if err := json.Unmarshal(running.scanner.Bytes(), &stopped); err != nil || stopped.Kind != "endpoint-lifecycle" || stopped.State != "stopped" {
		t.Fatalf("fresh Endpoint stop event = %q / %+v / %v", running.scanner.Text(), stopped, err)
	}
	waitErr := running.command.Wait()
	running.finished = true
	running.cancel()
	if waitErr != nil {
		t.Fatalf("fresh Endpoint exit: %v; stderr=%s", waitErr, running.stderr.String())
	}
	if _, err := os.Lstat(running.attachment); !os.IsNotExist(err) {
		t.Fatalf("fresh Endpoint attachment remains after stop: %v", err)
	}
}

func (running *liveEnrolledEndpoint) cleanup() error {
	if running.finished {
		return nil
	}
	var failures []error
	pid := running.command.Process.Pid
	if err := running.command.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		failures = append(failures, fmt.Errorf("kill pid %d: %w", pid, err))
	}
	running.cancel()
	waitErr := running.command.Wait()
	running.finished = true
	state := running.command.ProcessState
	var exitError *exec.ExitError
	if state == nil {
		failures = append(failures, fmt.Errorf("pid %d has no reaped process state", pid))
	} else if status := state.Sys().(syscall.WaitStatus); waitErr != nil &&
		(!errors.As(waitErr, &exitError) || !status.Signaled() || status.Signal() != syscall.SIGKILL) {
		failures = append(failures, fmt.Errorf("wait pid %d: %w", pid, waitErr))
	}
	if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
		failures = append(failures, fmt.Errorf("pid %d remains after cleanup: %v", pid, err))
	}
	if running.attachment != "" {
		if _, err := os.Lstat(running.attachment); !os.IsNotExist(err) {
			failures = append(failures, fmt.Errorf("attachment remains after cleanup: %s: %v", running.attachment, err))
		}
	}
	return errors.Join(failures...)
}
