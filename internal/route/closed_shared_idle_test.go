//go:build linux

package route

import (
	"context"
	"crypto/ed25519"
	"net"
	"testing"
	"time"
)

type observedSharedAccept struct {
	net.Listener
	entered chan struct{}
}

func (listener observedSharedAccept) Accept() (net.Conn, error) {
	close(listener.entered)
	return listener.Listener.Accept()
}

func TestClosedSharedCarrierHandshakeBudgetStartsAtConnection(t *testing.T) {
	certificate := entryBindingCertificate(t, 191)
	key := identifierFromKey(certificate.Leaf.PublicKey.(ed25519.PublicKey))
	listener, err := ListenClosedSharedCarrier(ClosedCarrierTCP, closedRoleCarrierTestEndpoint(t, ClosedCarrierTCP), certificate, func([32]byte) bool { return false }, 1)
	if err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	concrete := listener.(*closedSharedTCPListener)
	concrete.listener = observedSharedAccept{Listener: concrete.listener, entered: entered}
	endpoint := concrete.listener.Addr().String()
	deadline := time.Now().Add(200 * time.Millisecond)
	accepted := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		carrier, acceptErr := listener.Accept(ctx, 200*time.Millisecond)
		if carrier.Connection != nil {
			acceptErr = carrier.Connection.Close()
		}
		accepted <- acceptErr
	}()
	t.Cleanup(func() {
		defer cancel()
		_ = listener.Close()
		select {
		case err := <-accepted:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(time.Second):
			t.Error("shared listener did not join")
		}
	})
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("shared listener did not enter socket acceptance")
	}
	// The real socket has entered Accept with no peer. Expire the old
	// pre-accept deadline before sending a valid direct TLS handshake.
	timer := time.NewTimer(time.Until(deadline) + 30*time.Millisecond)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-t.Context().Done():
		t.Fatal(t.Context().Err())
	}
	carrier, err := OpenClosedRoleCarrier(context.Background(), ClosedRoleCarrierRequest{
		CarrierProfile: ClosedCarrierTCP, Endpoint: endpoint, ExpectedServer: key,
		Deadline: time.Now().Add(time.Second),
	})
	if err != nil {
		t.Fatalf("new connection inherited listener idle time: %v", err)
	}
	if err := carrier.Close(); err != nil {
		t.Fatal(err)
	}
}
