//go:build linux

package document_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/connection"
	"github.com/dianabuilds/ardents-network/internal/application/document"
)

func TestDocumentReadCancellationDuringSetupAndTerminalWait(t *testing.T) {
	binary := buildArdentsNext(t)
	for _, phase := range []string{"setup", "terminal"} {
		t.Run(phase, func(t *testing.T) {
			ready := make(chan struct{})
			var peer connection.Interface
			if phase == "setup" {
				peer = &setupBlockingDocumentReadPeer{entered: ready}
			} else {
				peer = &documentReadPeer{response: documentResponse([]byte("secret")), class: connection.CleanClose,
					terminal: make(chan struct{}), responseSent: ready}
			}
			socket := documentReadSocket(t, peer)
			command := exec.CommandContext(t.Context(), binary, "read", socket)
			command.Stdin = strings.NewReader("fixture-link\n")
			var output, diagnostic bytes.Buffer
			command.Stdout, command.Stderr = &output, &diagnostic
			if err := command.Start(); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- command.Wait() }()
			joined := false
			defer func() {
				if !joined {
					_ = command.Process.Kill()
					<-done
				}
			}()
			select {
			case <-ready:
			case err := <-done:
				joined = true
				t.Fatalf("process exited before %s: %v", phase, err)
			case <-time.After(3 * time.Second):
				t.Fatalf("process did not reach %s", phase)
			}
			select {
			case err := <-done:
				joined = true
				t.Fatalf("process completed before %s was released: %v", phase, err)
			default:
			}
			if err := command.Process.Signal(syscall.SIGTERM); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-done:
				joined = true
				if err == nil || output.Len() != 0 || diagnostic.String() != "read cancelled\n" {
					t.Fatalf("cancellation: error=%v, output=%q, diagnostic=%q", err, &output, &diagnostic)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("cancellation did not join")
			}
		})
	}
}

func TestTrustedDocumentProcessCancellationInterruptsInheritedPipes(t *testing.T) {
	binary := buildArdentsNext(t)
	for _, stalledOutput := range []bool{false, true} {
		name := "pending-input"
		if stalledOutput {
			name = "stalled-output"
		}
		t.Run(name, func(t *testing.T) {
			peer := &documentReadPeer{response: documentResponse(bytes.Repeat([]byte("x"), document.MaximumBytes)), class: connection.CleanClose}
			socket := documentReadSocket(t, peer)
			ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, binary, "read", socket)
			input, err := command.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			defer input.Close()
			output, err := command.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			defer output.Close()
			var diagnostic bytes.Buffer
			command.Stderr = &diagnostic
			if err := command.Start(); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- command.Wait() }()
			joined := false
			defer func() {
				if !joined {
					_ = command.Process.Kill()
					<-done
				}
			}()
			if stalledOutput {
				if _, err := io.WriteString(input, "fixture-link\n"); err != nil {
					t.Fatal(err)
				}
				// The first byte proves the child completed the exchange and
				// entered presentation. Leave the rest of this real pipe unread.
				var first [1]byte
				if _, err := io.ReadFull(output, first[:]); err != nil {
					t.Fatal(err)
				}
				time.Sleep(100 * time.Millisecond)
			} else {
				// No line/EOF is supplied; allow the fresh process to install
				// signal handling and block in its inherited stdin read.
				time.Sleep(250 * time.Millisecond)
			}
			if err := command.Process.Signal(syscall.SIGTERM); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-done:
				joined = true
				if err == nil || diagnostic.String() != "read cancelled\n" {
					t.Fatalf("terminal result=%v, diagnostic=%q", err, diagnostic.String())
				}
			case <-time.After(2 * time.Second):
				t.Fatal("SIGTERM did not interrupt and join the inherited pipe")
			}
		})
	}
}

func TestDocumentReadPreservesInheritedPipeFlags(t *testing.T) {
	binary := buildArdentsNext(t)
	peer := &documentReadPeer{response: documentResponse([]byte("hello")), class: connection.CleanClose}
	socket := documentReadSocket(t, peer)
	input, inputWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	defer inputWriter.Close()
	outputReader, output, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer outputReader.Close()
	defer output.Close()
	flags := func(file *os.File) uintptr {
		t.Helper()
		value, _, errno := syscall.Syscall(syscall.SYS_FCNTL, file.Fd(), syscall.F_GETFL, 0)
		if errno != 0 {
			t.Fatal(errno)
		}
		return value
	}
	inputFlags, outputFlags := flags(input), flags(output)
	if _, err := io.WriteString(inputWriter, "fixture-link\n"); err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(t.Context(), binary, "read", socket)
	command.Stdin, command.Stdout = input, output
	var diagnostic bytes.Buffer
	command.Stderr = &diagnostic
	if err := command.Run(); err != nil {
		t.Fatalf("document read: %v; diagnostic: %s", err, diagnostic.String())
	}
	if flags(input) != inputFlags || flags(output) != outputFlags {
		t.Fatal("document read changed inherited pipe flags")
	}
}
