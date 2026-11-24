//go:build linux

package administration_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/administration"
)

type documentLinkPeer struct {
	snapshotPublicationPeer
	requested chan struct{}
}

func (peer *documentLinkPeer) PublishedLink(context.Context) (string, error) {
	close(peer.requested)
	return "ardents://test-public-link", nil
}

func TestPublishedLinkPresentationAndCancellation(t *testing.T) {
	binary := buildArdentsNext(t)
	for _, blocked := range []bool{false, true} {
		name := "present"
		if blocked {
			name = "blocked-output"
		}
		t.Run(name, func(t *testing.T) {
			peer := &documentLinkPeer{requested: make(chan struct{})}
			socket := filepath.Join(t.TempDir(), "admin.sock")
			server, err := administration.Listen(socket, peer)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := server.Close(); err != nil {
					t.Error(err)
				}
			}()
			command := exec.CommandContext(t.Context(), binary, "link", socket)
			var diagnostic bytes.Buffer
			command.Stderr = &diagnostic
			if !blocked {
				output, err := command.Output()
				if err != nil || string(output) != "ardents://test-public-link\n" || diagnostic.Len() != 0 {
					t.Fatalf("link: error=%v, output=%q, diagnostic=%q", err, output, &diagnostic)
				}
			} else {
				reader, writer, err := os.Pipe()
				if err != nil {
					t.Fatal(err)
				}
				defer reader.Close()
				defer writer.Close()
				fd := int(writer.Fd())
				if err := syscall.SetNonblock(fd, true); err != nil {
					t.Fatal(err)
				}
				block := make([]byte, 4096)
				for {
					_, err := syscall.Write(fd, block)
					if errors.Is(err, syscall.EAGAIN) {
						break
					}
					if err != nil {
						t.Fatal(err)
					}
				}
				command.Stdout = writer
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
				case <-peer.requested:
				case err := <-done:
					joined = true
					t.Fatalf("process exited before Link request: %v", err)
				case <-time.After(3 * time.Second):
					t.Fatal("Link request did not arrive")
				}
				time.Sleep(100 * time.Millisecond)
				if err := command.Process.Signal(syscall.SIGTERM); err != nil {
					t.Fatal(err)
				}
				select {
				case err := <-done:
					joined = true
					if err == nil || diagnostic.String() != "read cancelled\n" {
						t.Fatalf("cancelled link: error=%v, diagnostic=%q", err, &diagnostic)
					}
				case <-time.After(3 * time.Second):
					t.Fatal("blocked output did not join")
				}
			}
			if calls, _ := peer.observed(); calls != 0 {
				t.Fatal("Link presentation published a snapshot")
			}
		})
	}
}
