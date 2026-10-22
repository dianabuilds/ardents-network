package source

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// The failure wrapper first closes the actual original TCP socket. It supplies
// only a late cleanup failure, never successful TLS, State or serving admission.
type sourceLateClose struct {
	net.Conn
	once    sync.Once
	result  error
	failure error
	calls   atomic.Int32
}

func TestSourceTLSShutdownKeepsNativeCloseSeparate(t *testing.T) {
	for _, reset := range []bool{false, true} {
		t.Run(map[bool]string{false: "native-failure", true: "peer-reset"}[reset], func(t *testing.T) {
			now := time.Now()
			fixture := newSourceTLSFixture(t, now)
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			peer, err := net.Dial("tcp", listener.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			defer peer.Close()
			original, err := listener.Accept()
			if err != nil {
				t.Fatal(err)
			}
			defer original.Close()
			late := errors.New("native TCP Close failure after TLS")
			injected := late
			if reset {
				injected = nil
			}
			native := &sourceLateClose{Conn: original, failure: injected}
			socket := &sourceSocket{Conn: native}
			policy := server{certificate: fixture.server, clientRoots: fixture.authority.pool,
				clientDigests: map[[32]byte]bool{fixture.clientPin: true}, headerTimeout: time.Second, clock: func() time.Time { return now }}
			secured := &sourceServerConnection{Conn: tls.Server(socket, serverTLSConfig(policy)), socket: socket}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			requested, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			joined := make(chan error, 1)
			go func() {
				joined <- handleConnection(ctx, policy, nil, secured, func(context.Context, Message) Message {
					close(requested)
					<-release
					return Message{Status: "not-found"}
				}, nil)
			}()
			done := false
			defer func() {
				cancel()
				unblock()
				_ = peer.Close()
				if !done {
					<-joined
				}
			}()
			if err := peer.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
				t.Fatal(err)
			}
			client := tls.Client(peer, clientTLSConfig(client{certificate: fixture.client, roots: fixture.authority.pool,
				serverName: fixture.serverName, leafKeyDigest: fixture.serverPin, clock: func() time.Time { return now }}))
			if err := writeRequest(client, Message{Operation: "latest"}); err != nil {
				t.Fatal(err)
			}
			<-requested
			if reset {
				if err := peer.(*net.TCPConn).SetLinger(0); err != nil {
					t.Fatal(err)
				}
				if err := peer.Close(); err != nil {
					t.Fatal(err)
				}
			}
			unblock()
			if !reset {
				if response, err := readResponse(client); err != nil || response.Status != "not-found" {
					t.Fatal(response, err)
				}
			}
			err = <-joined
			done = true
			var physical *sourcePhysicalClose
			if reset && errors.As(err, &physical) {
				t.Fatal("peer TLS shutdown became native cleanup failure", err)
			}
			if !reset && (!errors.As(err, &physical) || !errors.Is(err, late)) {
				t.Fatal("TLS hid original TCP Close failure", err)
			}
			if native.calls.Load() != 1 {
				t.Fatal("TLS and native retirement closed original TCP more than once")
			}
		})
	}
}

func (c *sourceLateClose) Close() error {
	c.calls.Add(1)
	c.once.Do(func() { c.result = errors.Join(c.Conn.Close(), c.failure) })
	return c.result
}

func TestSourceHandlerRetainsOriginalPhysicalCloseFailure(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		t.Run(map[bool]string{false: "normal-response", true: "cancellation"}[canceled], func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			peer, err := net.Dial("tcp", listener.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			defer peer.Close()
			original, err := listener.Accept()
			if err != nil {
				t.Fatal(err)
			}
			defer original.Close()
			late := errors.New("original Source socket late close failure")
			connection := &sourceLateClose{Conn: original, failure: late}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			active := make(chan int, 2)
			joined := make(chan error, 1)
			go func() {
				joined <- handleConnection(ctx, server{headerTimeout: time.Second}, func(n int) { active <- n }, connection,
					func(context.Context, Message) Message { return Message{Status: "not-found"} }, nil)
			}()
			if <-active != 1 {
				t.Fatal("handler not retained")
			}
			if canceled {
				cancel()
			} else {
				if err := peer.SetDeadline(time.Now().Add(time.Second)); err != nil {
					t.Fatal(err)
				}
				if err := writeRequest(peer, Message{Operation: "latest"}); err != nil {
					t.Fatal(err)
				}
				if response, err := readResponse(peer); err != nil || response.Status != "not-found" {
					t.Fatal(response, err)
				}
			}
			err = <-joined
			if !errors.Is(err, late) {
				t.Fatalf("lost original physical Close failure: %v", err)
			}
			if canceled && !errors.Is(err, context.Canceled) {
				t.Fatalf("lost original cancellation: %v", err)
			}
			if <-active != -1 {
				t.Fatal("handler user not joined")
			}
			if connection.calls.Load() != 1 {
				t.Fatal("original socket Close invoked more than once")
			}
		})
	}
}
