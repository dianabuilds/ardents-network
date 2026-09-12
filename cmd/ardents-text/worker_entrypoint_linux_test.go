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
	"strconv"
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
			harnessRead, harnessWrite, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer harnessRead.Close()
			defer harnessWrite.Close()
			harnessPipe, err := syscall.Dup(int(harnessRead.Fd()))
			if err != nil {
				t.Fatal(err)
			}
			defer syscall.Close(harnessPipe)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, binaryPath, "worker-publisher")
			command.Stdin, command.Stdout, command.Stderr = child, child, null
			if extra {
				command.ExtraFiles = []*os.File{null}
			}
			markHarnessDescriptorsCloseOnExec(t)
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

// GitHub's go test executor can leave its result pipe without close-on-exec.
// The installed Endpoint does not grant that pipe, so the harness must not
// accidentally turn it into worker authority. ExtraFiles remains intentionally
// inherited and proves that the worker rejects a real foreign descriptor.
func markHarnessDescriptorsCloseOnExec(t *testing.T) {
	t.Helper()
	directory, err := syscall.Open("/proc/self/fd", syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(directory)
	var buffer [4096]byte
	for {
		n, err := syscall.ReadDirent(directory, buffer[:])
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			return
		}
		_, _, entries := syscall.ParseDirent(buffer[:n], -1, nil)
		for _, entry := range entries {
			fd, err := strconv.Atoi(entry)
			if err != nil || fd <= 2 || fd == directory {
				continue
			}
			flags, _, errno := syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), syscall.F_GETFD, 0)
			if errno != 0 {
				continue
			}
			if flags&syscall.FD_CLOEXEC != 0 {
				continue
			}
			if _, _, errno = syscall.Syscall(syscall.SYS_FCNTL, uintptr(fd), syscall.F_SETFD, flags|syscall.FD_CLOEXEC); errno != 0 {
				t.Fatal(errno)
			}
		}
	}
}
