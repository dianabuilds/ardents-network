//go:build linux && text_worker_installed

package runtime

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/execution"
)

// The separately pinned text_worker_escape artifact attempts forbidden host
// effects before HELLO. Real qualified launch plus independently observed host
// listeners are required; ordinary Text readiness cannot substitute for it.
func TestInstalledExecutionEscapeMatrix(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Fatal("invalid environment: original non-root Endpoint MainPID required")
	}
	probes := startExecutionEscapeProbes(t)
	defer probes.close(t)
	for _, surface := range []execution.Surface{execution.Connection, execution.Administration} {
		t.Run(string(surface), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
			defer cancel()
			principal := [32]byte{2}
			owner, err := New(execution.Config{ID: [32]byte{1}, Grants: []execution.Grant{{Principal: principal, Surface: surface}}})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := owner.Close(); err != nil {
					t.Errorf("escape owner cleanup: %v", err)
				}
			})
			invocation, err := owner.Launch(ctx, principal, surface)
			if err != nil {
				t.Fatalf("escape artifact did not reach qualified readiness: %v", err)
			}
			var operation *Operation
			t.Cleanup(func() {
				if operation != nil {
					operation.Close()
				}
				if err := invocation.Close(); err != nil {
					t.Errorf("escape invocation cleanup: %v", err)
				}
			})
			operation, err = invocation.BeginOperation(ctx)
			if err != nil || operation.Check() != nil {
				t.Fatalf("qualified escape operation unavailable: %v", err)
			}
			// Retain original kernel/proc descriptors independently of runtime
			// cleanup. Pathname disappearance or manager MainPID=0 is no join.
			group, err := os.OpenRoot("/sys/fs/cgroup" + invocation.activation.Instance.Cgroup)
			if err != nil {
				t.Fatal(err)
			}
			events, openErr := group.Open("cgroup.events")
			if err := errors.Join(openErr, group.Close()); err != nil {
				if events != nil {
					_ = events.Close()
				}
				t.Fatal(err)
			}
			observer := &hostileExecutionTree{events: events}
			t.Cleanup(func() {
				for _, process := range observer.processes {
					if err := process.Close(); err != nil {
						t.Error(err)
					}
				}
				if err := events.Close(); err != nil {
					t.Error(err)
				}
			})
			var filesystem syscall.Statfs_t
			if syscall.Fstatfs(int(events.Fd()), &filesystem) != nil || filesystem.Type != 0x63677270 {
				t.Fatal("escape observer is not an original kernel cgroup-v2 descriptor")
			}
			process, err := os.Open("/proc/" + strconv.FormatUint(uint64(invocation.activation.Instance.PID), 10))
			if err != nil {
				t.Fatal(err)
			}
			observer.processes = append(observer.processes, process)
			if gone, populated, err := observer.observe(); err != nil || gone || !populated {
				t.Fatalf("escape scope positive control unavailable: %v", err)
			}
			operation.Close()
			if err := invocation.Close(); err != nil {
				t.Fatal(err)
			}
			observer.requireJoined(t)
			if !invocation.CompletedCurrent() || operation.Check() == nil {
				t.Fatal("joined escape operation retained effects or lost exact completion")
			}
			probes.requireControls(t)
			probes.requireNoContact(t)
		})
	}
}

const executionEscapeControl = "execution-escape-positive-control"

type executionEscapeProbes struct {
	listeners []net.Listener
	packets   []*net.UDPConn
	control   chan string
	seen      chan string
	done      chan struct{}
}

func startExecutionEscapeProbes(t *testing.T) *executionEscapeProbes {
	t.Helper()
	probes := &executionEscapeProbes{control: make(chan string, 16), seen: make(chan string, 16), done: make(chan struct{}, 5)}
	// World readability makes the file probe meaningful for DynamicUser if
	// the host tree were incorrectly exposed; UID permission is not the oracle.
	file, err := os.OpenFile("/tmp/ardents-text-worker-escape-file", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Remove("/tmp/ardents-text-worker-escape-file"); err != nil {
			t.Error(err)
		}
	})
	_, writeErr := file.WriteString("host-only escape sentinel")
	if err := errors.Join(writeErr, file.Chmod(0644), file.Close()); err != nil {
		t.Fatal(err)
	}
	for _, endpoint := range []struct{ network, address string }{
		{"tcp4", "127.0.0.1:45561"}, {"tcp6", "[::1]:45562"}, {"unix", "/tmp/ardents-text-worker-escape.sock"},
	} {
		listener, err := net.Listen(endpoint.network, endpoint.address)
		if err != nil {
			probes.close(t)
			t.Fatal(err)
		}
		probes.listeners = append(probes.listeners, listener)
		go probes.watchListener(endpoint.network, listener)
		if endpoint.network == "unix" {
			if err := os.Chmod(endpoint.address, 0666); err != nil {
				probes.close(t)
				t.Fatal(err)
			}
		}
	}
	for _, address := range []string{"127.0.0.1:45563", "127.0.0.1:45564"} {
		endpoint, err := net.ResolveUDPAddr("udp4", address)
		if err != nil {
			t.Fatal(err)
		}
		connection, err := net.ListenUDP("udp4", endpoint)
		if err != nil {
			probes.close(t)
			t.Fatal(err)
		}
		probes.packets = append(probes.packets, connection)
		go probes.watchPacket(address, connection)
	}
	probes.requireControls(t)
	return probes
}

func (probes *executionEscapeProbes) report(name string, body []byte) {
	if string(body) == executionEscapeControl {
		probes.control <- name
	} else {
		probes.seen <- name
	}
}

func (probes *executionEscapeProbes) watchListener(name string, listener net.Listener) {
	defer func() { probes.done <- struct{}{} }()
	for {
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		if err := connection.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
			_ = connection.Close()
			probes.report(name, nil)
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(connection, 128))
		if err := errors.Join(readErr, connection.Close()); err != nil {
			probes.report(name, nil)
		} else {
			probes.report(name, body)
		}
	}
}

func (probes *executionEscapeProbes) watchPacket(name string, connection *net.UDPConn) {
	defer func() { probes.done <- struct{}{} }()
	buffer := make([]byte, 512)
	for {
		count, _, err := connection.ReadFromUDP(buffer)
		if err != nil {
			return
		}
		probes.report(name, buffer[:count])
	}
}

func (probes *executionEscapeProbes) requireControls(t *testing.T) {
	t.Helper()
	for _, endpoint := range []struct{ network, address string }{
		{"tcp4", "127.0.0.1:45561"}, {"tcp6", "[::1]:45562"}, {"unix", "/tmp/ardents-text-worker-escape.sock"},
		{"udp4", "127.0.0.1:45563"}, {"udp4", "127.0.0.1:45564"},
	} {
		connection, err := net.DialTimeout(endpoint.network, endpoint.address, time.Second)
		if err != nil {
			t.Fatal(err)
		}
		_, writeErr := connection.Write([]byte(executionEscapeControl))
		if err := errors.Join(writeErr, connection.Close()); err != nil {
			t.Fatal(err)
		}
		select {
		case name := <-probes.control:
			want := endpoint.network
			if want == "udp4" {
				want = endpoint.address
			}
			if name != want {
				t.Fatalf("escape control observed by wrong listener: got %s want %s", name, want)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("escape listener positive control unavailable: %s %s", endpoint.network, endpoint.address)
		}
	}
}

func (probes *executionEscapeProbes) requireNoContact(t *testing.T) {
	t.Helper()
	select {
	case name := <-probes.seen:
		t.Fatalf("confined escape artifact reached host listener: %s", name)
	default:
	}
}

func (probes *executionEscapeProbes) close(t *testing.T) {
	t.Helper()
	for _, listener := range probes.listeners {
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Error(err)
		}
	}
	for _, connection := range probes.packets {
		if err := connection.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Error(err)
		}
	}
	for range len(probes.listeners) + len(probes.packets) {
		select {
		case <-probes.done:
		case <-time.After(2 * time.Second):
			t.Fatal("escape observer did not join after closing its listener")
		}
	}
	probes.requireNoContact(t)
}
