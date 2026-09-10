//go:build linux

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// A fresh command process is required: the test harness already owns poller
// descriptors. This verifies inherited authority, not installed confinement.
func TestWorkerEntrypointAuditsDescriptorsAndJoinsCancellation(t *testing.T) {
	binaryPath := filepath.Join(t.TempDir(), "ardents-text")
	build := exec.CommandContext(t.Context(), "go", "build", "-o", binaryPath, ".")
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build worker: %v\n%s", err, output)
	}
	for _, extra := range []bool{false, true} {
		name := "sole-local-attachment"
		if extra {
			name = "foreign-descriptor"
		}
		t.Run(name, func(t *testing.T) {
			fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM|syscall.SOCK_CLOEXEC, 0)
			if err != nil {
				t.Fatal(err)
			}
			child := os.NewFile(uintptr(fds[0]), "worker-attachment")
			defer child.Close()
			parent := os.NewFile(uintptr(fds[1]), "endpoint-attachment")
			peer, err := net.FileConn(parent)
			_ = parent.Close()
			if err != nil {
				t.Fatal(err)
			}
			defer peer.Close()
			if err := peer.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
				t.Fatal(err)
			}
			null, err := os.OpenFile("/dev/null", os.O_RDWR, 0)
			if err != nil {
				t.Fatal(err)
			}
			defer null.Close()
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, binaryPath, "worker-publisher")
			command.Stdin, command.Stdout, command.Stderr = child, child, null
			command.Env = []string{"GOMAXPROCS=2", "GOMEMLIMIT=96MiB"}
			if extra {
				command.ExtraFiles = []*os.File{null}
			}
			if err := command.Start(); err != nil {
				t.Fatal(err)
			}
			_ = child.Close()
			done := make(chan error, 1)
			joined := make(chan struct{})
			go func() { done <- command.Wait(); close(joined) }()
			t.Cleanup(func() {
				cancel()
				select {
				case <-joined:
				case <-time.After(time.Second):
					t.Error("worker process cleanup did not join")
				}
			})
			if extra {
				select {
				case err := <-done:
					if err == nil || command.ProcessState.ExitCode() != 2 {
						t.Fatalf("foreign descriptor exit: %v", err)
					}
				case <-time.After(time.Second):
					t.Fatal("foreign descriptor was not rejected at entry")
				}
				return
			}
			snapshot := []byte("one authorized immutable document")
			header := make([]byte, 77)
			copy(header, "ARDTWP01")
			header[8], header[9] = 2, 31
			binary.BigEndian.PutUint32(header[41:45], uint32(len(snapshot)))
			digest := sha256.Sum256(snapshot)
			copy(header[45:], digest[:])
			if _, err := peer.Write(append(header, snapshot...)); err != nil {
				t.Fatal(err)
			}
			var ready [72]byte
			if _, err := io.ReadFull(peer, ready[:]); err != nil {
				t.Fatalf("valid inherited attachment refused: %v", err)
			}
			if string(ready[:8]) != "ARDTWR01" || !bytes.Equal(ready[8:40], header[9:41]) || !bytes.Equal(ready[40:], digest[:]) {
				t.Fatal("invalid readiness binding")
			}
			if err := command.Process.Signal(syscall.SIGTERM); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-done:
				status, ok := command.ProcessState.Sys().(syscall.WaitStatus)
				if err == nil || !ok || !status.Exited() || status.Signaled() || status.ExitStatus() != 2 {
					t.Fatalf("worker did not cancel through its owned exit path: %v / %v", err, status)
				}
				if ctx.Err() != nil {
					t.Fatal("worker needed forced process termination")
				}
			case <-time.After(time.Second):
				t.Fatal("worker cancellation failed to join blocked socket read")
			}
		})
	}
}
