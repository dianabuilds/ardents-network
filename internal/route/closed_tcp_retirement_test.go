package route

import (
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

func TestClosedNodeTCPRetirementAfterPeerReset(t *testing.T) {
	certificate, clientCertificate := entryBindingCertificate(t, 151), entryBindingCertificate(t, 152)
	clientKey := identifierFromKey(clientCertificate.Leaf.PublicKey.(ed25519.PublicKey))
	listener, err := ListenClosedSharedCarrier(ClosedCarrierTCP, closedRoleCarrierTestEndpoint(t, ClosedCarrierTCP), certificate,
		func(key [32]byte) bool { return key == clientKey }, 16)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan Carrier, 1)
	failed := make(chan error, 1)
	go func() {
		carrier, err := listener.Accept(t.Context(), 5*time.Second)
		if err != nil {
			failed <- err
			return
		}
		if carrier.Kind != ClosedSharedNode || carrier.NodeKey != clientKey {
			failed <- errors.Join(errors.New("accepted Carrier lost Node authentication"), carrier.Connection.Close())
			return
		}
		accepted <- carrier.Connection
	}()
	client, err := OpenClosedNodeCarrier(t.Context(), ClosedNodeCarrierRequest{CarrierProfile: ClosedCarrierTCP, Endpoint: closedSharedCarrierEndpoint(t, listener),
		Certificate: clientCertificate, ExpectedPeerKey: identifierFromKey(certificate.Leaf.PublicKey.(ed25519.PublicKey)), Deadline: time.Now().Add(5 * time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	var server Carrier
	select {
	case server = <-accepted:
	case err := <-failed:
		t.Fatal(err)
	}
	raw := server.(interface{ NetConn() net.Conn }).NetConn().(*net.TCPConn)
	if err := raw.SetLinger(0); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}
	// Observe the real peer reset before intentional local transport retirement.
	var value [1]byte
	if _, err := io.ReadFull(client, value[:]); err == nil {
		t.Fatal("reset peer remained usable")
	}
	if err := client.Close(); err != nil {
		t.Fatalf("retired physical socket reported TLS notification as failed cleanup: %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("repeated retirement changed outcome: %v", err)
	}
}

type failingClosedSocket struct {
	net.Conn
	cause  error
	closes int
}

func (socket *failingClosedSocket) Close() error {
	socket.closes++
	return errors.Join(socket.Conn.Close(), socket.cause)
}

func TestClosedNodeTCPRetainsActualSocketCloseFailure(t *testing.T) {
	local, peer := net.Pipe()
	defer peer.Close()
	original := errors.New("physical socket close failed")
	socket := &failingClosedSocket{Conn: local, cause: original}
	carrier := &closedTCPNodeTransport{Conn: tls.Client(socket, &tls.Config{MinVersion: tls.VersionTLS13})}
	for range 2 {
		if err := carrier.Close(); !errors.Is(err, original) {
			t.Fatalf("physical close failure erased: %v", err)
		}
	}
	if socket.closes != 1 {
		t.Fatal("physical socket closed more than once")
	}
	if _, err := carrier.Write([]byte{1}); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("retired Carrier remained writable: %v", err)
	}
}
