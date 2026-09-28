package administration

import (
	"context"
	"io"
	"net"
	"testing"
	"time"
)

// Exercise shutdown after a handler finishes but before its worker retires
// the accepted connection from the server's ownership set.
func TestAdministrationCloseAfterCompletedHandlerBeforeRetirement(t *testing.T) {
	path, listening := snapshotTestServer(t, testInterface{})
	if err := listening.Close(); err != nil {
		t.Fatal(err)
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	listener.SetUnlinkOnClose(false)
	ctx, cancel := context.WithCancel(t.Context())
	owner := &server{path: path, listener: listener, ctx: ctx, cancel: cancel, clients: make(map[*net.UnixConn]struct{})}
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
	owner.handle(accepted)
	if err := owner.Close(); err != nil {
		t.Fatalf("shutdown after completed handler: %v", err)
	}
	if err := owner.Close(); err != nil {
		t.Fatalf("repeated shutdown: %v", err)
	}
}
