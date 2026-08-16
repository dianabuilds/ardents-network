//go:build linux

package camouflage

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestCandidateCleanupEscalatesThroughSIGTERMAndIsIdempotent(t *testing.T) {
	child := shellChild(t, "trap 'exit 0' TERM; while :; do sleep 1; done")
	started := time.Now()
	if err := child.closeBefore(time.Now().Add(6 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed < 1400*time.Millisecond || elapsed > 4*time.Second {
		t.Fatalf("SIGTERM cleanup took %s", elapsed)
	}
	if err := child.closeBefore(time.Now().Add(6 * time.Second)); err != nil {
		t.Fatalf("repeated cleanup: %v", err)
	}
}

func TestCandidateCleanupEscalatesThroughSIGKILL(t *testing.T) {
	child := shellChild(t, "trap '' TERM; while :; do sleep 1; done")
	started := time.Now()
	if err := child.closeBefore(time.Now().Add(6 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed < 2900*time.Millisecond || elapsed > 5*time.Second {
		t.Fatalf("SIGKILL cleanup took %s", elapsed)
	}
}

func shellChild(t *testing.T, script string) *candidateChild {
	t.Helper()
	command := exec.Command("/bin/sh", "-c", script)
	stateRoot := filepath.Join(t.TempDir(), "state")
	if err := os.Mkdir(stateRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := configureCandidateProcess(command, stateRoot); err != nil {
		t.Fatal(err)
	}
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	child := &candidateChild{command: command, stdin: stdin, wait: make(chan error, 1), drained: make(chan struct{})}
	close(child.drained)
	child.stdoutRest.limit, child.stderr.limit = maximumControlTranscript, maximumControlTranscript
	go func() { child.wait <- command.Wait() }()
	return child
}
