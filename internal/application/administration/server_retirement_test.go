package administration

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/internal/socketfile"
)

func TestAdministrationClosePreservesReplacementFile(t *testing.T) {
	parent, err := os.MkdirTemp("", "aa-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(parent); err != nil {
			t.Error(err)
		}
	})
	path := filepath.Join(parent, "service.sock")
	server, err := Listen(path, testInterface{})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	original := filepath.Join(parent, "old.sock")
	if err := os.Rename(path, original); err != nil {
		t.Fatal(err)
	}
	const replacement = "a separate application entry"
	if err := os.WriteFile(path, []byte(replacement), 0600); err != nil {
		t.Fatal(err)
	}
	closeErr := server.Close()
	if got, err := os.ReadFile(path); err != nil || string(got) != replacement {
		t.Fatal("Administration cleanup removed or changed the replacement", err)
	}
	if !errors.Is(closeErr, socketfile.ErrChanged) || server.Close() != closeErr {
		t.Fatal("Administration cleanup did not retain the original identity refusal", closeErr)
	}
}

// Exercise shutdown after a handler finishes but before its worker retires
// the accepted connection from the server's ownership set.
func TestAdministrationCloseAfterCompletedHandlerBeforeRetirement(t *testing.T) {
	path, listening := snapshotTestServer(t, testInterface{})
	if err := listening.Close(); err != nil {
		t.Fatal(err)
	}
	listener, entry, err := socketfile.Listen(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	owner := &server{entry: entry, listener: listener, ctx: ctx, cancel: cancel, clients: make(map[*net.UnixConn]struct{})}
	t.Cleanup(func() {
		if err := owner.Close(); err != nil {
			t.Error(err)
		}
	})
	peer, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	accepted, err := listener.AcceptUnix()
	if err != nil {
		t.Fatal(err)
	}
	owner.clients[accepted] = struct{}{}
	if err := peer.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(peer, "invalid\n"); err != nil {
		t.Fatal(err)
	}
	if err := peer.CloseWrite(); err != nil {
		t.Fatal(err)
	}
	owner.handle(accepted, time.Now())
	if err := owner.Close(); err != nil {
		t.Fatalf("shutdown after completed handler: %v", err)
	}
	if err := owner.Close(); err != nil {
		t.Fatalf("repeated shutdown: %v", err)
	}
}
