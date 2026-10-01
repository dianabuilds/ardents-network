//go:build linux

package forwarding

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	routecarrier "github.com/dianabuilds/ardents-network/internal/route/carrier"
)

type retirementPipe struct {
	net.Conn
	armed       atomic.Bool
	writing     chan struct{}
	release     chan struct{}
	interrupted chan struct{}
}

func (pipe *retirementPipe) Write(value []byte) (int, error) {
	if pipe.armed.CompareAndSwap(true, false) {
		close(pipe.writing)
		<-pipe.release
	}
	return pipe.Conn.Write(value)
}

func (pipe *retirementPipe) SetDeadline(deadline time.Time) error {
	err := pipe.Conn.SetDeadline(deadline)
	pipe.signalInterrupt()
	return err
}

func (pipe *retirementPipe) SetReadDeadline(deadline time.Time) error {
	err := pipe.Conn.SetReadDeadline(deadline)
	pipe.signalInterrupt()
	return err
}

func (pipe *retirementPipe) signalInterrupt() {
	select {
	case <-pipe.writing:
		select {
		case pipe.interrupted <- struct{}{}:
		default:
		}
	default:
	}
}

func TestAcceptedTLSRetirementPreservesCloseNotify(t *testing.T) {
	certificate, _ := nodeCertificate(t, 1, "retirement")
	left, right := net.Pipe()
	t.Cleanup(func() { _ = left.Close(); _ = right.Close() })
	pipe := &retirementPipe{Conn: left, writing: make(chan struct{}), release: make(chan struct{}), interrupted: make(chan struct{}, 1)}
	secured := tls.Server(pipe, &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS13})
	peer := tls.Client(right, &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS13}) // Fixture certificate only.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	handshake := make(chan error, 1)
	go func() { handshake <- secured.HandshakeContext(ctx) }()
	if err := peer.HandshakeContext(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-handshake; err != nil {
		t.Fatal(err)
	}
	pipe.armed.Store(true)
	server := &forwardServer{limit: make(chan struct{}, 1)}
	server.limit <- struct{}{}
	server.active.Add(1)
	server.workers.Add(1)
	// An unclassified fixture exercises terminal carrier ownership without admitting a role.
	go server.serveAccepted(ctx, routecarrier.ClosedSharedCarrier{Connection: secured})
	select {
	case <-pipe.writing:
	case <-ctx.Done():
		t.Fatal("closeNotify did not start")
	}
	cancel()
	select {
	case <-pipe.interrupted:
	case <-time.After(time.Second):
		t.Fatal("cancellation did not interrupt the reader")
	}
	close(pipe.release)
	_ = peer.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := peer.Read(make([]byte, 1)); err != io.EOF {
		t.Errorf("peer closeNotify: %v", err)
	}
	server.workers.Wait()
	if server.acceptedCloseErr != nil {
		t.Fatalf("carrier retirement: %v", server.acceptedCloseErr)
	}
}
