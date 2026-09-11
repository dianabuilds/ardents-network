//go:build linux && text_worker_installed

package endpoint

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
)

const (
	installedEscapeProbeFile = "/tmp/ardents-text-worker-escape-file"
	installedEscapeProbeIPC  = "/tmp/ardents-text-worker-escape.sock"
	installedEscapeProbeTCP4 = "127.0.0.1:45561"
	installedEscapeProbeTCP6 = "[::1]:45562"
	installedEscapeProbeUDP4 = "127.0.0.1:45563"
)

// TestInstalledTextWorkerEscapeMatrix uses a separately pinned adversarial
// worker. The Endpoint process owns real host listeners and a host-only file;
// a successful worker launch proves the artifact's pre-INIT probes did not
// reach them, while the listeners independently detect a forbidden packet or
// connection. This is installed P6/P7 evidence, not a Service journey.
func TestInstalledTextWorkerEscapeMatrix(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Second)
	defer cancel()
	if err := verifyTextEndpointService(ctx); err != nil {
		t.Fatalf("invalid installed environment: %v", err)
	}
	probes := startInstalledEscapeProbes(t)
	defer probes.close(t)
	for _, role := range []struct {
		name     string
		surface  broker.Surface
		snapshot []byte
	}{
		{"reader", broker.Connection, nil},
		{"publisher", broker.Administration, []byte("escape profile snapshot")},
	} {
		t.Run(role.name, func(t *testing.T) {
			endpoint, principal := textContextEndpoint(t)
			owner := admittedTextContext(t, endpoint, principal, role.surface)
			worker, err := owner.launchTextWorker(ctx, role.snapshot)
			if err != nil {
				t.Fatalf("escape artifact did not reach verified readiness: %v", err)
			}
			if worker.grant.Active() != 1 || worker.lease.Context().Err() != nil {
				t.Fatal("escape artifact did not receive the bounded verified Grant")
			}
			instance := installedTextWorkerInstance(t, ctx, worker, role.name)
			events, err := pinTextWorkerCgroup(instance)
			if err != nil {
				t.Fatal(err)
			}
			if err := worker.Close(); err != nil {
				_ = events.Close()
				t.Fatal(err)
			}
			gone, populated, err := readTextWorkerCgroup(events)
			closeErr := events.Close()
			if err != nil || closeErr != nil || !gone && populated {
				t.Fatalf("escape worker cleanup: populated=%v err=%v close=%v", populated, err, closeErr)
			}
			if worker.grant.Active() != 0 || worker.lease.Context().Err() == nil {
				t.Fatal("escape worker retained Grant after cleanup")
			}
			requireInstalledTextWorkerCollected(t, ctx, instance.name, role.name)
			probes.requireNoContact(t)
		})
	}
}

type installedEscapeProbes struct {
	tcp4 *net.TCPListener
	tcp6 *net.TCPListener
	udp4 *net.UDPConn
	ipc  *net.UnixListener
	seen chan string
}

func startInstalledEscapeProbes(t *testing.T) *installedEscapeProbes {
	t.Helper()
	if err := os.WriteFile(installedEscapeProbeFile, []byte("host-only escape sentinel"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(installedEscapeProbeFile) })
	if err := os.Remove(installedEscapeProbeIPC); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	ipc, err := net.ListenUnix("unix", &net.UnixAddr{Name: installedEscapeProbeIPC, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(installedEscapeProbeIPC) })
	tcp4, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 45561})
	if err != nil {
		_ = ipc.Close()
		t.Fatal(err)
	}
	tcp6, err := net.ListenTCP("tcp6", &net.TCPAddr{IP: net.ParseIP("::1"), Port: 45562})
	if err != nil {
		_ = tcp4.Close()
		_ = ipc.Close()
		t.Fatal(err)
	}
	udp4, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 45563})
	if err != nil {
		_ = tcp6.Close()
		_ = tcp4.Close()
		_ = ipc.Close()
		t.Fatal(err)
	}
	probes := &installedEscapeProbes{tcp4: tcp4, tcp6: tcp6, udp4: udp4, ipc: ipc, seen: make(chan string, 4)}
	probes.watchTCP("IPv4 TCP", tcp4)
	probes.watchTCP("IPv6 TCP", tcp6)
	probes.watchIPC(ipc)
	probes.watchUDP(udp4)
	return probes
}

func (probes *installedEscapeProbes) watchTCP(name string, listener *net.TCPListener) {
	go func() {
		if connection, err := listener.AcceptTCP(); err == nil {
			_ = connection.Close()
			probes.seen <- name
		}
	}()
}

func (probes *installedEscapeProbes) watchIPC(listener *net.UnixListener) {
	go func() {
		if connection, err := listener.AcceptUnix(); err == nil {
			_ = connection.Close()
			probes.seen <- "host IPC"
		}
	}()
}

func (probes *installedEscapeProbes) watchUDP(listener *net.UDPConn) {
	go func() {
		buffer := make([]byte, 128)
		if count, _, err := listener.ReadFromUDP(buffer); err == nil && string(buffer[:count]) == "ardents-text-worker-escape-probe" {
			probes.seen <- "IPv4 UDP"
		}
	}()
}

func (probes *installedEscapeProbes) requireNoContact(t *testing.T) {
	t.Helper()
	select {
	case name := <-probes.seen:
		t.Fatalf("confined worker reached %s probe", name)
	default:
	}
}

func (probes *installedEscapeProbes) close(t *testing.T) {
	t.Helper()
	for _, closer := range []interface{ Close() error }{probes.tcp4, probes.tcp6, probes.udp4, probes.ipc} {
		if err := closer.Close(); err != nil && !errorsIsClosedNetwork(err) {
			t.Errorf("close escape probe: %v", err)
		}
	}
	probes.requireNoContact(t)
}

func errorsIsClosedNetwork(err error) bool {
	return err != nil && (err == net.ErrClosed || fmt.Sprintf("%v", err) == "use of closed network connection")
}
