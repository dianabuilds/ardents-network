package endpoint

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	endpointapiection "github.com/dianabuilds/ardents-network/internal/service/connection"
)

func TestRouteAttachmentOpenerReusesListenerAndReleasesAcceptedIPC(t *testing.T) {
	path := filepath.Join(os.TempDir(), "ara-"+time.Now().Format("150405.000000")+".sock")
	defer os.Remove(path)
	listener, err := listenLocal(path, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	var mu sync.Mutex
	current, high, accepts := 0, 0, 0
	resources := func(kind string, delta int) uint32 {
		mu.Lock()
		defer mu.Unlock()
		if kind == "route-attachment-accept" {
			accepts += delta
			return uint32(accepts)
		}
		if kind != "accepted-ipc" {
			t.Fatalf("unexpected resource kind %q", kind)
		}
		current += delta
		if current > high {
			high = current
		}
		return uint32(high)
	}
	opener := routeAttachmentOpener(listener, resources)
	for generation := uint64(1); generation <= 2; generation++ {
		peer := dialRouteSocket(t, path)
		attachment, err := opener(context.Background(), endpointapiection.Recovery{
			Generation: generation, Deadline: time.Now().Add(time.Second)})
		if err != nil {
			t.Fatal(err)
		}
		if err := attachment.Close(); err != nil {
			t.Fatal(err)
		}
		_ = peer.Close()
	}
	mu.Lock()
	defer mu.Unlock()
	if current != 0 || high != 1 || accepts != 2 {
		t.Fatalf("accepted Route Attachments: current=%d high=%d count=%d", current, high, accepts)
	}
}

func TestRouteAttachmentOpenerStopsOnContextCancellation(t *testing.T) {
	path := filepath.Join(os.TempDir(), "arc-"+time.Now().Format("150405.000000")+".sock")
	defer os.Remove(path)
	listener, err := listenLocal(path, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	opener := routeAttachmentOpener(listener, func(string, int) uint32 { return 0 })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	_, err = opener(ctx, endpointapiection.Recovery{Generation: 2, Deadline: time.Now().Add(time.Second)})
	if err == nil || time.Since(started) > 250*time.Millisecond {
		t.Fatalf("cancelled attachment request did not stop promptly: %v", err)
	}
}

func dialRouteSocket(t *testing.T, path string) net.Conn {
	t.Helper()
	connection, err := net.DialTimeout("unix", path, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return connection
}
