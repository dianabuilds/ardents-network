//go:build native_rendezvous_multihost

package state_test

import (
	"crypto/tls"
	"io"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// TestNativeRendezvousMultiHostTCPFaultRelay exercises three bounded TCP fault
// outcomes through a test-only transparent relay in front of the real remote
// product Rendezvous. It does not model packet loss, reordering, MTU, NAT,
// probing, host loss, recovery, or availability.
func TestNativeRendezvousMultiHostTCPFaultRelay(t *testing.T) {
	environment := requireNativeRendezvousMultiHostEnvironment(t)
	remoteEndpoint := net.JoinHostPort(environment.host, strconv.Itoa(environment.port))
	fixture := newRendezvousStateFixture(t, remoteEndpoint)
	stage := stageNativeRemoteRendezvous(t, fixture, environment)
	remote := nativeRendezvousMultiHostRemoteRendezvous{environment: environment}
	t.Cleanup(func() { remote.remove(t) })
	remote.start(t, stage)
	remote.waitReady(t)
	relay := startNativeRendezvousTCPFaultRelay(t, remoteEndpoint)

	t.Run("delayed exact carriage", func(t *testing.T) {
		initiator, responder := nativeRendezvousMultiHostOpenTCPFaultRelayPair(t, relay.Endpoint(), fixture, 0xc1)
		defer nativeRendezvousMultiHostCloseTCPFaultRelayPair(t, relay, initiator, responder)
		relay.Delay(200 * time.Millisecond)
		started := time.Now()
		nativeRendezvousMultiHostCarryTCPFaultRelayPayload(t, initiator, responder, "native Rendezvous delayed exact carriage")
		if elapsed := time.Since(started); elapsed < 350*time.Millisecond || elapsed > 3*time.Second {
			t.Fatalf("two-way 200ms relay delay elapsed %s, want [350ms,3s]", elapsed)
		}
		relay.Normal()
	})

	t.Run("reset closes pair and permits fresh pair", func(t *testing.T) {
		initiator, responder := nativeRendezvousMultiHostOpenTCPFaultRelayPair(t, relay.Endpoint(), fixture, 0xc2)
		nativeRendezvousMultiHostCarryTCPFaultRelayPayload(t, initiator, responder, "native Rendezvous reset baseline")
		relay.Reset(t)
		nativeRendezvousMultiHostRequireTerminalLegClose(t, initiator, "Initiator")
		nativeRendezvousMultiHostRequireTerminalLegClose(t, responder, "Responder")
		_ = initiator.Close()
		_ = responder.Close()
		time.Sleep(200 * time.Millisecond)
		freshInitiator, freshResponder := nativeRendezvousMultiHostOpenTCPFaultRelayPair(t, relay.Endpoint(), fixture, 0xc3)
		defer nativeRendezvousMultiHostCloseTCPFaultRelayPair(t, relay, freshInitiator, freshResponder)
		nativeRendezvousMultiHostCarryTCPFaultRelayPayload(t, freshInitiator, freshResponder, "native Rendezvous fresh pair after reset")
	})

	t.Run("blackhole obeys caller read budget", func(t *testing.T) {
		initiator, responder := nativeRendezvousMultiHostOpenTCPFaultRelayPair(t, relay.Endpoint(), fixture, 0xc4)
		defer nativeRendezvousMultiHostCloseTCPFaultRelayPair(t, relay, initiator, responder)
		relay.Drop()
		if _, err := initiator.Write([]byte("native Rendezvous intentionally dropped payload")); err != nil {
			t.Fatalf("write dropped payload: %v", err)
		}
		if err := responder.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
			t.Fatalf("set blackhole read deadline: %v", err)
		}
		_, err := responder.Read(make([]byte, 1))
		_ = responder.SetReadDeadline(time.Time{})
		if err == nil {
			t.Fatal("blackholed payload unexpectedly reached the Responder")
		}
		if networkError, ok := err.(net.Error); !ok || !networkError.Timeout() {
			t.Fatalf("blackhole result = %v, want caller read-budget timeout", err)
		}
		relay.Reset(t)
		relay.Normal()
	})
}

func nativeRendezvousMultiHostOpenTCPFaultRelayPair(t *testing.T, endpoint string, fixture rendezvousStateFixture, marker byte) (*tls.Conn, *tls.Conn) {
	t.Helper()
	attachment := [32]byte{marker}
	initiator, err := openRendezvousProcessLeg(t.Context(), endpoint, fixture.initiator.certificate, fixture.rendezvous.public,
		fixture.leg(attachment, route.InitiatorRole))
	if err != nil {
		t.Fatalf("open fault-relay Initiator leg: %v", err)
	}
	responder, err := openRendezvousProcessLeg(t.Context(), endpoint, fixture.responder.certificate, fixture.rendezvous.public,
		fixture.leg(attachment, route.ResponderRole))
	if err != nil {
		_ = initiator.Close()
		t.Fatalf("open fault-relay Responder leg: %v", err)
	}
	return initiator, responder
}

func nativeRendezvousMultiHostCloseTCPFaultRelayPair(t *testing.T, relay *nativeRendezvousMultiHostTCPFaultRelay, initiator, responder *tls.Conn) {
	t.Helper()
	_ = initiator.Close()
	_ = responder.Close()
	relay.WaitIdle(t)
}

func nativeRendezvousMultiHostCarryTCPFaultRelayPayload(t *testing.T, initiator, responder *tls.Conn, payload string) {
	t.Helper()
	if _, err := initiator.Write([]byte(payload)); err != nil {
		t.Fatalf("write fault-relay payload: %v", err)
	}
	if received := readProcessExact(t, responder, len(payload)); string(received) != payload {
		t.Fatalf("fault-relay carriage = %q, want %q", received, payload)
	}
}

type nativeRendezvousMultiHostTCPFaultMode uint8

const (
	nativeRendezvousMultiHostTCPFaultNormal nativeRendezvousMultiHostTCPFaultMode = iota
	nativeRendezvousMultiHostTCPFaultDelay
	nativeRendezvousMultiHostTCPFaultDrop
)

// nativeRendezvousMultiHostTCPFaultRelay is test-only transparent byte forwarding. Its fault switch
// is local to this qualification process and is never a Route or Node input.
type nativeRendezvousMultiHostTCPFaultRelay struct {
	listener net.Listener
	target   string

	mu          sync.Mutex
	mode        nativeRendezvousMultiHostTCPFaultMode
	delay       time.Duration
	connections map[net.Conn]struct{}
	acceptDone  chan struct{}
	bridges     sync.WaitGroup
}

func startNativeRendezvousTCPFaultRelay(t *testing.T, target string) *nativeRendezvousMultiHostTCPFaultRelay {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	relay := &nativeRendezvousMultiHostTCPFaultRelay{listener: listener, target: target, connections: make(map[net.Conn]struct{}), acceptDone: make(chan struct{})}
	go relay.accept()
	t.Cleanup(func() { relay.Close(t) })
	return relay
}

func (relay *nativeRendezvousMultiHostTCPFaultRelay) Endpoint() string {
	return relay.listener.Addr().String()
}

func (relay *nativeRendezvousMultiHostTCPFaultRelay) Delay(value time.Duration) {
	relay.mu.Lock()
	relay.mode, relay.delay = nativeRendezvousMultiHostTCPFaultDelay, value
	relay.mu.Unlock()
}

func (relay *nativeRendezvousMultiHostTCPFaultRelay) Drop() {
	relay.mu.Lock()
	relay.mode, relay.delay = nativeRendezvousMultiHostTCPFaultDrop, 0
	relay.mu.Unlock()
}

func (relay *nativeRendezvousMultiHostTCPFaultRelay) Normal() {
	relay.mu.Lock()
	relay.mode, relay.delay = nativeRendezvousMultiHostTCPFaultNormal, 0
	relay.mu.Unlock()
}

func (relay *nativeRendezvousMultiHostTCPFaultRelay) Reset(t *testing.T) {
	t.Helper()
	relay.closeConnectionsWithReset()
	relay.WaitIdle(t)
}

func (relay *nativeRendezvousMultiHostTCPFaultRelay) WaitIdle(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		relay.mu.Lock()
		idle := len(relay.connections) == 0
		relay.mu.Unlock()
		if idle {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("TCP fault relay retained a connection after cleanup")
}

func (relay *nativeRendezvousMultiHostTCPFaultRelay) Close(t *testing.T) {
	t.Helper()
	_ = relay.listener.Close()
	<-relay.acceptDone
	relay.closeConnectionsWithReset()
	done := make(chan struct{})
	go func() { relay.bridges.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Error("TCP fault relay retained bridge work after cleanup")
	}
}

func (relay *nativeRendezvousMultiHostTCPFaultRelay) accept() {
	defer close(relay.acceptDone)
	for {
		client, err := relay.listener.Accept()
		if err != nil {
			return
		}
		relay.bridges.Add(1)
		go relay.bridge(client)
	}
}

func (relay *nativeRendezvousMultiHostTCPFaultRelay) bridge(client net.Conn) {
	defer relay.bridges.Done()
	server, err := net.DialTimeout("tcp", relay.target, 3*time.Second)
	if err != nil {
		_ = client.Close()
		return
	}
	relay.mu.Lock()
	relay.connections[client] = struct{}{}
	relay.connections[server] = struct{}{}
	relay.mu.Unlock()
	defer func() {
		_ = client.Close()
		_ = server.Close()
		relay.mu.Lock()
		delete(relay.connections, client)
		delete(relay.connections, server)
		relay.mu.Unlock()
	}()
	var forwards sync.WaitGroup
	forwards.Add(2)
	go func() { defer forwards.Done(); relay.forward(server, client) }()
	go func() { defer forwards.Done(); relay.forward(client, server) }()
	forwards.Wait()
}

func (relay *nativeRendezvousMultiHostTCPFaultRelay) forward(destination, source net.Conn) {
	buffer := make([]byte, 32<<10)
	for {
		count, readErr := source.Read(buffer)
		if count > 0 {
			mode, delay := relay.fault()
			if mode != nativeRendezvousMultiHostTCPFaultDrop {
				if mode == nativeRendezvousMultiHostTCPFaultDelay {
					time.Sleep(delay)
				}
				if nativeRendezvousMultiHostTCPFaultWriteAll(destination, buffer[:count]) != nil {
					return
				}
			}
		}
		if readErr != nil {
			return
		}
	}
}

func (relay *nativeRendezvousMultiHostTCPFaultRelay) fault() (nativeRendezvousMultiHostTCPFaultMode, time.Duration) {
	relay.mu.Lock()
	defer relay.mu.Unlock()
	return relay.mode, relay.delay
}

func (relay *nativeRendezvousMultiHostTCPFaultRelay) closeConnectionsWithReset() {
	relay.mu.Lock()
	connections := make([]net.Conn, 0, len(relay.connections))
	for connection := range relay.connections {
		connections = append(connections, connection)
	}
	relay.mu.Unlock()
	for _, connection := range connections {
		if tcp, ok := connection.(*net.TCPConn); ok {
			_ = tcp.SetLinger(0)
		}
		_ = connection.Close()
	}
}

func nativeRendezvousMultiHostTCPFaultWriteAll(writer net.Conn, value []byte) error {
	for len(value) > 0 {
		count, err := writer.Write(value)
		if err != nil {
			return err
		}
		if count <= 0 || count > len(value) {
			return io.ErrShortWrite
		}
		value = value[count:]
	}
	return nil
}
