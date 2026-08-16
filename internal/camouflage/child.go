package camouflage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"
)

type candidateChild struct {
	command    *exec.Cmd
	stdin      io.WriteCloser
	wait       chan error
	stdoutRest boundedOutput
	stderr     boundedOutput
	drained    chan struct{}
	mu         sync.Mutex
	closed     bool
	closeErr   error
}

type readinessParser func(io.Reader) (string, []byte, error)

func startClientProcess(ctx context.Context, binary, stateRoot string, deadline, parent time.Time) (*candidateChild, string, error) {
	environment := []string{
		"TOR_PT_MANAGED_TRANSPORT_VER=1",
		"TOR_PT_CLIENT_TRANSPORTS=webtunnel",
		"TOR_PT_STATE_LOCATION=" + stateRoot,
		"TOR_PT_EXIT_ON_STDIN_CLOSE=1",
	}
	return startCandidateProcess(ctx, binary, environment, deadline, parent, readClientReadiness)
}

func startCandidateProcess(ctx context.Context, binary string, environment []string, deadline, parent time.Time,
	parse readinessParser,
) (*candidateChild, string, error) {
	if !time.Now().Before(deadline) {
		return nil, "", errors.New("startup deadline expired")
	}
	command := exec.Command(binary)
	configureCandidateProcess(command)
	command.Env = environment
	stdin, err := command.StdinPipe()
	if err != nil {
		return nil, "", err
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, "", err
	}
	child := &candidateChild{command: command, stdin: stdin, wait: make(chan error, 1), drained: make(chan struct{})}
	child.stdoutRest.limit = maximumControlTranscript
	child.stderr.limit = maximumControlTranscript
	command.Stderr = &child.stderr
	if err := command.Start(); err != nil {
		_ = stdin.Close()
		return nil, "", err
	}
	go func() { child.wait <- command.Wait() }()
	type readyResult struct {
		address    string
		transcript []byte
		err        error
	}
	ready := make(chan readyResult, 1)
	go func() {
		address, transcript, err := parse(stdout)
		ready <- readyResult{address: address, transcript: transcript, err: err}
		if err == nil {
			_, _ = io.Copy(&child.stdoutRest, stdout)
		}
		close(child.drained)
	}()
	select {
	case result := <-ready:
		if result.err != nil || child.stderr.tooLarge() ||
			len(result.transcript)+len(child.stderr.bytes()) > maximumControlTranscript {
			cleanupErr := child.closeBefore(cleanupDeadline(parent))
			return nil, "", errors.Join(result.err, errors.New("client control transcript invalid"),
				cleanupFailure(cleanupErr))
		}
		return child, result.address, nil
	case <-ctx.Done():
		cleanupErr := child.closeBefore(cleanupDeadline(parent))
		return nil, "", errors.Join(ctx.Err(), cleanupFailure(cleanupErr))
	case <-time.After(time.Until(deadline)):
		cleanupErr := child.closeBefore(cleanupDeadline(parent))
		return nil, "", errors.Join(errors.New("client readiness timed out"), cleanupFailure(cleanupErr))
	}
}

func (child *candidateChild) closeBefore(deadline time.Time) error {
	child.mu.Lock()
	defer child.mu.Unlock()
	if child.closed {
		return child.closeErr
	}
	child.closed = true
	_ = child.stdin.Close()
	if child.waitUntil(deadline, 1500*time.Millisecond) {
		child.closeErr = errors.Join(cleanupProcessGroup(child.command.Process.Pid, deadline),
			child.waitForDrain(deadline), child.outputError())
		return child.closeErr
	}
	if err := signalTerminate(child.command.Process); err != nil {
		child.closeErr = err
		return child.closeErr
	}
	if child.waitUntil(deadline, 1500*time.Millisecond) {
		child.closeErr = errors.Join(cleanupProcessGroup(child.command.Process.Pid, deadline),
			child.waitForDrain(deadline), child.outputError())
		return child.closeErr
	}
	if err := signalKill(child.command.Process); err != nil {
		child.closeErr = err
		return child.closeErr
	}
	if !child.waitUntil(deadline, 500*time.Millisecond) {
		child.closeErr = fmt.Errorf("candidate process %d was not reaped", child.command.Process.Pid)
		return child.closeErr
	}
	child.closeErr = errors.Join(cleanupProcessGroup(child.command.Process.Pid, deadline),
		child.waitForDrain(deadline), child.outputError())
	return child.closeErr
}

func (child *candidateChild) waitForDrain(deadline time.Time) error {
	wait := time.Until(deadline)
	if wait <= 0 {
		return errors.New("candidate pipe drain missed cleanup deadline")
	}
	select {
	case <-child.drained:
		return nil
	case <-time.After(wait):
		return errors.New("candidate pipe drain missed cleanup deadline")
	}
}

func (child *candidateChild) waitUntil(deadline time.Time, maximum time.Duration) bool {
	wait := min(maximum, time.Until(deadline))
	if wait <= 0 {
		return false
	}
	select {
	case <-child.wait:
		return true
	case <-time.After(wait):
		return false
	}
}

func (child *candidateChild) outputError() error {
	if child.stderr.tooLarge() || child.stdoutRest.tooLarge() {
		return errors.New("candidate output limit exceeded")
	}
	return nil
}
