//go:build linux && (document_worker_installed || endpoint_launch_native)

package endpoint

import (
	"context"
	"errors"
	"net"
	"os"
	"strconv"
	"syscall"
	"testing"
	"time"
)

// WaitOriginalReader waits for a Reader listener owned by the original Endpoint.
func WaitOriginalReader(t testing.TB, socket string, original [3]string, uid, gid uint32, bound time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), bound)
	defer cancel()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		conn, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: socket, Net: "unix"})
		if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ECONNREFUSED) {
			select {
			case <-ctx.Done():
				t.Fatalf("original Reader listener did not become ready: %v", ctx.Err())
			case <-ticker.C:
				continue
			}
		}
		if err != nil {
			t.Fatal("installed Reader listener", err)
		}
		defer conn.Close()
		raw, err := conn.SyscallConn()
		if err != nil {
			t.Fatal(err)
		}
		var peer *syscall.Ucred
		var peerErr error
		if err := raw.Control(func(fd uintptr) {
			peer, peerErr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
		}); err != nil || peerErr != nil {
			t.Fatal("installed listener peer", err, peerErr)
		}
		if peer == nil || strconv.FormatInt(int64(peer.Pid), 10) != original[0] || peer.Uid != uid || peer.Gid != gid {
			t.Fatal("Reader listener is not owned by the original installed process")
		}
		if err := conn.Close(); err != nil {
			t.Fatal("close Reader readiness probe:", err)
		}
		break
	}
	if current := StartupIdentity(t, ctx); current != original {
		t.Fatal("Service readiness lost original invocation", current)
	}
}
