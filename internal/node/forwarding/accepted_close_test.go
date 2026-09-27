package forwarding

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	routecarrier "github.com/dianabuilds/ardents-network/internal/route/carrier"
	"github.com/dianabuilds/ardents-network/internal/route/replay"
)

type refusedForwardingCarrier struct {
	net.Conn
	closed   chan struct{}
	closeErr error
	once     sync.Once
}

func (carrier *refusedForwardingCarrier) Close() error {
	carrier.once.Do(func() {
		_ = carrier.Conn.Close()
		close(carrier.closed)
	})
	return carrier.closeErr
}

type oneForwardingCarrierListener struct {
	ready      chan struct{}
	accepted   chan struct{}
	connection net.Conn
	kind       routecarrier.ClosedSharedCarrierKind
	served     bool
}

func (listener *oneForwardingCarrierListener) Accept(ctx context.Context, _ time.Duration) (routecarrier.ClosedSharedCarrier, error) {
	if !listener.served {
		select {
		case <-listener.ready:
		case <-ctx.Done():
			return routecarrier.ClosedSharedCarrier{}, ctx.Err()
		}
		listener.served = true
		close(listener.accepted)
		return routecarrier.ClosedSharedCarrier{Kind: listener.kind, Connection: listener.connection}, nil
	}
	<-ctx.Done()
	return routecarrier.ClosedSharedCarrier{}, ctx.Err()
}

func (*oneForwardingCarrierListener) Close() error { return nil }

func TestClosedForwardingDrainRetainsAcceptedConnectionCloseFailure(t *testing.T) {
	for _, test := range []struct {
		name     string
		capacity bool
	}{
		{name: "capacity refusal", capacity: true},
		{name: "admitted direct child"},
	} {
		t.Run(test.name, func(t *testing.T) { checkForwardingAcceptedCloseFailure(t, test.capacity) })
	}
}

func checkForwardingAcceptedCloseFailure(t *testing.T, capacity bool) {
	t.Helper()
	closeErr := errors.New("accepted carrier close failed")
	local, peer := net.Pipe()
	defer peer.Close()
	carrier := &refusedForwardingCarrier{Conn: local, closed: make(chan struct{}), closeErr: closeErr}
	listener := &oneForwardingCarrierListener{ready: make(chan struct{}), accepted: make(chan struct{}), connection: carrier,
		kind: routecarrier.ClosedSharedDirect}
	pool, err := routecarrier.NewClosedCarrierPool(time.Now)
	if err != nil {
		t.Fatal(err)
	}
	spends, err := replay.Open(t.TempDir(), replay.Binding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 1})
	if err != nil {
		t.Fatal(err)
	}
	server := newClosedForwardingServerWithHost(closedForwardingDependencies{}, tls.Certificate{}, listener,
		&closedForwardingReceivingResources{spends: spends}, pool, nil, 1, time.Now)
	defer func() {
		server.Stop()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Drain(ctx)
	}()
	if capacity {
		server.limit <- struct{}{} // Force the accepted Carrier through the capacity-refusal path.
	}
	close(listener.ready)
	select {
	case <-listener.accepted:
	case <-time.After(time.Second):
		t.Fatal("listener did not accept the Carrier")
	}
	select {
	case <-carrier.closed:
	case <-time.After(time.Second):
		t.Fatal("accepted child did not close the Carrier")
	}
	server.Stop()
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := server.Drain(ctx); !errors.Is(err, closeErr) {
		t.Fatalf("Drain lost accepted Carrier close failure: %v", err)
	}
	if err := server.Drain(ctx); !errors.Is(err, closeErr) {
		t.Fatalf("repeated Drain lost accepted Carrier close failure: %v", err)
	}
}
