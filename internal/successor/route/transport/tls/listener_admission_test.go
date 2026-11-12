package tls

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	stdtls "crypto/tls"
	"crypto/x509"
	"math/big"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
)

// The barrier observes the first real TLS socket read, after its original
// deadline and capacity admission. It supplies no authentication result.
type observedHandshakeListener struct {
	net.Listener
	first atomic.Bool
	read  chan time.Time
}

func (l *observedHandshakeListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err == nil && !l.first.Swap(true) {
		return &observedHandshakeConnection{Conn: c, read: l.read}, nil
	}
	return c, err
}

type observedHandshakeConnection struct {
	net.Conn
	read     chan time.Time
	once     sync.Once
	deadline time.Time
}

type observedAcceptance struct {
	carrier transport.ClosedSharedCarrier
	err     error
}

func (c *observedHandshakeConnection) SetDeadline(deadline time.Time) error {
	if err := c.Conn.SetDeadline(deadline); err != nil {
		return err
	}
	c.deadline = deadline
	return nil
}

func (c *observedHandshakeConnection) Read(p []byte) (int, error) {
	c.once.Do(func() { c.read <- c.deadline })
	return c.Conn.Read(p)
}

func TestSharedTCPListenerAuthenticatesConcurrentlyWithinCapacity(t *testing.T) {
	observeOriginalHandshakeCapacity(t, 2)
}

func TestSharedTCPListenerRefusesArrivalsBeyondFiniteCapacity(t *testing.T) {
	observeOriginalHandshakeCapacity(t, 1)
}

func observeOriginalHandshakeCapacity(t *testing.T, capacity uint16) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { clear(private) })
	now := time.Now()
	leaf := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Minute), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, leaf, leaf, public, private)
	if err != nil {
		t.Fatal(err)
	}
	reservation, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	endpoint := reservation.Addr().String()
	if err := reservation.Close(); err != nil {
		t.Fatal(err)
	}
	listener, err := ListenShared(endpoint, stdtls.Certificate{Certificate: [][]byte{der}, PrivateKey: private}, func([32]byte) bool { return false }, capacity)
	if err != nil {
		t.Fatal(err)
	}
	read := make(chan time.Time, 1)
	original := listener.(*sharedListener)
	original.listener = &observedHandshakeListener{Listener: original.listener, read: read}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	finished := make(chan struct{})
	result := make(chan observedAcceptance, 1)
	go func() {
		defer close(finished)
		carrier, err := listener.Accept(ctx, 2*time.Second)
		result <- observedAcceptance{carrier: carrier, err: err}
	}()
	stalled, err := net.Dial("tcp", endpoint)
	if err != nil {
		cancel()
		_ = listener.Close()
		<-finished
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); _ = stalled.Close(); _ = listener.Close(); <-finished })
	var stalledDeadline time.Time
	select {
	case stalledDeadline = <-read:
	case <-ctx.Done():
		t.Fatal("first real TLS read did not reach original admission", ctx.Err())
	}
	var key [32]byte
	copy(key[:], public)
	end, _ := ctx.Deadline()
	client, err := OpenEndpoint(ctx, transport.ClosedRoleCarrierRequest{CarrierProfile: transport.ClosedCarrierTCP, Endpoint: endpoint, ExpectedServer: key, Deadline: end})
	if client != nil {
		t.Cleanup(func() { _ = client.Close() })
	}
	if capacity == 1 {
		if err == nil || !time.Now().Before(stalledDeadline) {
			t.Fatal("excess arrival did not refuse before original stalled peer expired", err)
		}
		return
	}
	if err != nil {
		t.Fatal("one admitted stalled handshake blocked the independent slot", err)
	}
	select {
	case accepted := <-result:
		if accepted.carrier.Connection != nil {
			defer accepted.carrier.Connection.Close()
		}
		if accepted.err != nil || accepted.carrier.Kind != transport.ClosedSharedDirect || !time.Now().Before(stalledDeadline) {
			t.Fatal("independent genuine TLS handoff did not precede stalled expiry", accepted.err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}
