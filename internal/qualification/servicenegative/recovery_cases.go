package servicenegative

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"io"
	"math/big"
	"net"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/serviceconn"
)

func (value fixture) observeRecoveryTerminal(parent context.Context,
	opener func(context.Context, serviceconn.Recovery) (net.Conn, error), timeout time.Duration) RecoveryObservation {
	started := time.Now()
	client, publisher, publication, ok := value.connected(parent)
	if !ok {
		return RecoveryObservation{}
	}
	clientRoute, publisherRoute := net.Pipe()
	clientApplication, clientPeer := net.Pipe()
	publisherApplication, publisherPeer := net.Pipe()
	defer clientPeer.Close()
	defer publisherPeer.Close()
	var ctx context.Context
	var cancel context.CancelFunc
	if timeout == 0 {
		ctx, cancel = context.WithCancel(parent)
		time.AfterFunc(100*time.Millisecond, cancel)
	} else {
		ctx, cancel = context.WithTimeout(parent, timeout)
	}
	defer cancel()
	binding := serviceconn.Recovery{CandidateView: [32]byte{41}, IsolationContext: [32]byte{42},
		DestinationBinding: [32]byte{43}, RouteProfile: "h3-recovery-negative-v1",
		WorkSafetyNotAfter: value.first.NotAfter, WorkSafetyMaximum: value.first.NotAfter,
		NoNewRecoveryAfter: value.first.NotAfter}
	outcomes := make(chan streamOutcome, 2)
	go func() {
		result, err := client.Do(ctx, serviceconn.Request{Action: "connect", Principal: value.connection,
			Session: sessionFor(client, value, ctx), Target: value.first.Target, Publication: publication,
			Route: failAfterBytes(clientRoute, 96<<10), Application: clientApplication,
			OpenAttachment: opener, RecoveryBinding: binding,
			BytesEachDirection: 256 << 10, At: value.now})
		outcomes <- streamOutcome{result, err}
	}()
	go func() {
		result, err := publisher.Do(ctx, serviceconn.Request{Action: "accept", Principal: value.connection,
			Session: sessionFor(publisher, value, ctx), Route: publisherRoute, Application: publisherApplication,
			OpenAttachment: opener, RecoveryBinding: binding, BytesEachDirection: 256 << 10, At: value.now})
		outcomes <- streamOutcome{result, err}
	}()
	go writeAndDrain(clientPeer)
	go writeAndDrain(publisherPeer)
	var terminal uint32
	class := ""
	for range 2 {
		select {
		case outcome := <-outcomes:
			if outcome.err != nil && outcome.result.Class != "clean service connection close" {
				terminal++
				class = outcome.result.Class
			}
		case <-time.After(2 * time.Second):
			return RecoveryObservation{TerminalCount: terminal, Class: class, WithinNanos: time.Since(started).Nanoseconds()}
		}
	}
	passed := terminal == 2
	logical := uint32(0)
	if passed {
		logical = 1
	}
	return RecoveryObservation{TerminalCount: logical, EndpointTerminalCount: terminal, Class: class,
		WithinNanos: time.Since(started).Nanoseconds(), Passed: passed}
}

func sessionFor(endpoint endpointRunner, value fixture, ctx context.Context) [32]byte {
	return admit(ctx, endpoint, value.connection, "connection", value.now)
}

func writeAndDrain(connection net.Conn) {
	defer connection.Close()
	go func() { _, _ = io.Copy(io.Discard, connection) }()
	_, _ = connection.Write(make([]byte, 256<<10))
}

func unavailableRecovery(context.Context, serviceconn.Recovery) (net.Conn, error) {
	return nil, errors.New("no eligible recovery candidate")
}

func forgedRecovery(context.Context, serviceconn.Recovery) (net.Conn, error) {
	connection, peer := net.Pipe()
	go serveForgedAttachment(peer)
	return connection, nil
}

func serveForgedAttachment(connection net.Conn) {
	defer connection.Close()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(99), Subject: pkix.Name{CommonName: "forged-recovery"},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Minute),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		return
	}
	server := tls.Server(connection, &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
		Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: private}}})
	_ = server.Handshake()
}

func blockedRecovery(ctx context.Context, _ serviceconn.Recovery) (net.Conn, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

type byteFailure struct {
	net.Conn
	mu        sync.Mutex
	remaining int
}

func failAfterBytes(connection net.Conn, count int) net.Conn {
	return &byteFailure{Conn: connection, remaining: count}
}

func (connection *byteFailure) Write(value []byte) (int, error) {
	connection.mu.Lock()
	defer connection.mu.Unlock()
	if connection.remaining <= 0 {
		_ = connection.Conn.Close()
		return 0, net.ErrClosed
	}
	if len(value) > connection.remaining {
		value = value[:connection.remaining]
	}
	written, err := connection.Conn.Write(value)
	connection.remaining -= written
	return written, err
}
