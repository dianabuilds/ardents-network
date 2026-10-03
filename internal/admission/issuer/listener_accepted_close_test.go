package issuer

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/carrier"
)

type issuerCloseFailureConn struct {
	net.Conn
	closed chan struct{}
	err    error
	once   sync.Once
}

func (connection *issuerCloseFailureConn) Close() error {
	connection.once.Do(func() {
		_ = connection.Conn.Close()
		close(connection.closed)
	})
	return connection.err
}

func TestClosedTokenListenerDrainRetainsAcceptedCarrierCloseFailure(t *testing.T) {
	for _, test := range []struct {
		name     string
		kind     carrier.ClosedSharedCarrierKind
		capacity bool
	}{
		{name: "Node capacity refusal", kind: carrier.ClosedSharedNode, capacity: true},
		{name: "direct capacity refusal", kind: carrier.ClosedSharedDirect, capacity: true},
		{name: "admitted Node child", kind: carrier.ClosedSharedNode},
	} {
		t.Run(test.name, func(t *testing.T) {
			closeErr := errors.New("issuer accepted Carrier close failed")
			local, peer := net.Pipe()
			defer peer.Close()
			connection := &issuerCloseFailureConn{Conn: local, closed: make(chan struct{}), err: closeErr}
			shared := &closedIssuerPausedAccept{connection: connection, kind: test.kind, entered: make(chan struct{}),
				release: make(chan struct{}), closed: make(chan struct{})}
			listener, err := StartClosedTokenListener(t.Context(), ClosedTokenListenerConfig{Issuer: &ClosedTokenIssuer{}, SharedListener: shared,
				ConnectionLimit: 1, Clock: time.Now, NodeHandler: func(context.Context, carrier.ClosedSharedCarrier,
					func(context.Context, io.ReadWriter, [32]byte, ardp.Hello) error) {
				}})
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				_ = listener.Drain(ctx)
			}()
			<-shared.entered
			if test.capacity {
				listener.limit <- struct{}{}
			}
			close(shared.release)
			select {
			case <-connection.closed:
			case <-time.After(time.Second):
				t.Fatal("issuer did not close accepted Carrier")
			}
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			if err := listener.Drain(ctx); !errors.Is(err, closeErr) {
				t.Fatalf("Issuer Drain lost accepted Carrier close failure: %v", err)
			}
			if !tokenListenerJoined(listener) {
				t.Fatal("failed accepted Carrier close was mistaken for an incomplete join")
			}
		})
	}
}
