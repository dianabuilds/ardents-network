package tls

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	stdtls "crypto/tls"
	"crypto/x509"
	"errors"
	"math/big"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// These barriers surround actual TCP I/O and its first physical Close. They
// supply no successful TLS classification or Route authority.
type heldHandshakeListener struct {
	net.Listener
	read, closing, release chan struct{}
	closes                 atomic.Int32
	cause                  error
}

func (l *heldHandshakeListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return &heldHandshakeSocket{Conn: c, owner: l}, nil
}

type heldHandshakeSocket struct {
	net.Conn
	owner *heldHandshakeListener
	once  sync.Once
}

func (c *heldHandshakeSocket) Read(p []byte) (int, error) {
	c.once.Do(func() { close(c.owner.read) })
	return c.Conn.Read(p)
}

func (c *heldHandshakeSocket) Close() error {
	first := c.owner.closes.Add(1) == 1
	err := c.Conn.Close()
	if first {
		close(c.owner.closing)
		<-c.owner.release
		err = errors.Join(err, c.owner.cause)
	}
	return err
}

func TestSharedListenerCanceledHandshakeJoinsOneOriginalSocketClose(t *testing.T) {
	for _, cause := range []error{nil, errors.New("late original handshake socket close failure"), net.ErrClosed} {
		name := "clean"
		if cause != nil {
			name = cause.Error()
		}
		t.Run(name, func(t *testing.T) {
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
			reserved, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			endpoint := reserved.Addr().String()
			if err := reserved.Close(); err != nil {
				t.Fatal(err)
			}
			listener, err := ListenShared(endpoint, stdtls.Certificate{Certificate: [][]byte{der}, PrivateKey: private}, func([32]byte) bool { return false }, 1)
			if err != nil {
				t.Fatal(err)
			}
			original := listener.(*sharedListener)
			observed := &heldHandshakeListener{Listener: original.listener, read: make(chan struct{}), closing: make(chan struct{}), release: make(chan struct{}), cause: cause}
			original.listener = observed
			var once sync.Once
			release := func() { once.Do(func() { close(observed.release) }) }
			accepted := make(chan observedAcceptance, 1)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			go func() { c, e := listener.Accept(ctx, time.Second); accepted <- observedAcceptance{c, e} }()
			t.Cleanup(func() { release(); _ = listener.Close() })
			peer, err := net.DialTimeout("tcp", endpoint, time.Second)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = peer.Close() })
			await := func(ready <-chan struct{}) {
				t.Helper()
				select {
				case <-ready:
				case <-ctx.Done():
					t.Fatal("original handshake barrier unavailable", ctx.Err())
				}
			}
			await(observed.read)
			closed := make(chan error, 1)
			go func() { closed <- listener.Close() }()
			await(observed.closing)
			select {
			case err := <-closed:
				t.Fatal("listener returned before original TLS cancellation Close joined", err)
			default:
			}
			release()
			result := <-closed
			arrival := <-accepted
			if arrival.err == nil || arrival.carrier.Connection != nil {
				t.Fatal("interrupted unauthenticated socket acquired a carrier")
			}
			if observed.closes.Load() != 1 {
				t.Fatal("cleanup bypassed original socket Close ownership", observed.closes.Load())
			}
			if cause == nil && result != nil || cause != nil && !errors.Is(result, cause) {
				t.Fatal("original physical Close result changed", result, cause)
			}
			if listener.Close() != result {
				t.Fatal("repeated listener Close lost its retained result")
			}
		})
	}
}
