//go:build linux

package state_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type processCert struct {
	certificate string
	key         string
	root        string
	pin         [32]byte
	sourcePin   [32]byte
	private     ed25519.PrivateKey
}

type nodeProcess struct {
	command         *exec.Cmd
	verifyPlacement func(*testing.T)
	events          chan nodeEvent
	done            chan struct{}
	stderr          *bytes.Buffer
	waitMu          sync.Mutex
	waitErr         error
}

type nodeEvent struct {
	Kind             string   `json:"kind"`
	Schema           string   `json:"schema"`
	State            string   `json:"state"`
	Epoch            uint64   `json:"epoch"`
	Assignment       string   `json:"assignment"`
	AssignmentDigest [32]byte `json:"assignment_digest"`
}

func startNodeCommand(t *testing.T, binary string, arguments ...string) *nodeProcess {
	t.Helper()
	command, verifyPlacement := nodeProcessCommand(t, binary, arguments...)
	command.Env = append(os.Environ(), "GOMAXPROCS=1", "GOMEMLIMIT=320MiB")
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	process := &nodeProcess{command: command, verifyPlacement: verifyPlacement, events: make(chan nodeEvent, 32), done: make(chan struct{}), stderr: new(bytes.Buffer)}
	command.Stderr = process.stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	go collectNodeEvents(stdout, process)
	go func() {
		err := command.Wait()
		process.waitMu.Lock()
		process.waitErr = err
		process.waitMu.Unlock()
		close(process.done)
	}()
	return process
}

func waitNodeState(t *testing.T, process *nodeProcess, state string, timeout time.Duration) nodeEvent {
	t.Helper()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case event, open := <-process.events:
			if !open {
				select {
				case <-process.done:
					t.Fatalf("Node output closed before %s: %v stderr=%s", state, process.terminalErr(), process.stderr)
				case <-timer.C:
					t.Fatalf("Node output closed before %s; stderr=%s", state, process.stderr)
				}
			}
			if event.Schema != "ardents-node-event-v1" {
				t.Fatalf("Node event schema = %q, want ardents-node-event-v1", event.Schema)
			}
			t.Logf("Node %d event: state=%s epoch=%d assignment=%s", process.command.Process.Pid, event.State, event.Epoch, event.Assignment)
			if event.State == state {
				if state == "READY" && process.verifyPlacement != nil {
					process.verifyPlacement(t)
				}
				return event
			}
		case <-process.done:
			for event := range process.events {
				if event.Schema != "ardents-node-event-v1" {
					t.Fatalf("Node event schema = %q, want ardents-node-event-v1", event.Schema)
				}
				t.Logf("Node %d event: state=%s epoch=%d assignment=%s", process.command.Process.Pid, event.State, event.Epoch, event.Assignment)
				if event.State == state {
					if state == "READY" && process.verifyPlacement != nil {
						process.verifyPlacement(t)
					}
					return event
				}
			}
			t.Fatalf("Node exited before %s: %v stderr=%s", state, process.terminalErr(), process.stderr)
		case <-timer.C:
			t.Fatalf("Node did not reach %s; stderr=%s", state, process.stderr)
		}
	}
}

func (process *nodeProcess) terminalErr() error {
	process.waitMu.Lock()
	defer process.waitMu.Unlock()
	return process.waitErr
}

func stopProcess(process *nodeProcess) {
	if process.command.Process != nil {
		_ = process.command.Process.Kill()
	}
	select {
	case <-process.done:
	case <-time.After(time.Second):
	}
}

func startClockObserver(t *testing.T, path string) func() {
	t.Helper()
	if err := os.WriteFile(path, []byte("external clock observation\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			now := time.Now()
			_ = os.Chtimes(path, now, now)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}

func writeJSON(t *testing.T, name string, value any) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	raw, _ := json.Marshal(value)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
func freeAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	_ = listener.Close()
	return address
}
