//go:build linux

package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/interfacev2/connection"
	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
)

func TestTrustedTextProcessCancellationInterruptsInheritedPipes(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "ardents-text")
	build := exec.CommandContext(t.Context(), "go", "build", "-o", binary, ".")
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v: %s", err, output)
	}
	for _, stalledOutput := range []bool{false, true} {
		name := "pending-input"
		if stalledOutput {
			name = "stalled-output"
		}
		t.Run(name, func(t *testing.T) {
			peer := &textReadPeer{response: textResponse(bytes.Repeat([]byte("x"), textdocument.MaximumBytes)), class: connection.CleanClose}
			socket := textReadSocket(t, peer)
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
				if err == nil || diagnostic.String() != "text read cancelled\n" {
					t.Fatalf("terminal result=%v, diagnostic=%q", err, diagnostic.String())
				}
			case <-time.After(2 * time.Second):
				t.Fatal("SIGTERM did not interrupt and join the inherited pipe")
			}
		})
	}
}
