package node

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/replay"
)

func TestClosedIntroductionDrainRetainsAcceptedCarrierCloseFailure(t *testing.T) {
	for _, test := range []struct {
		name     string
		capacity bool
		kind     route.ClosedSharedCarrierKind
	}{
		{name: "capacity refusal", capacity: true, kind: route.ClosedSharedNode},
		{name: "direct refusal", kind: route.ClosedSharedDirect},
		{name: "admitted Node child", kind: route.ClosedSharedNode},
	} {
		t.Run(test.name, func(t *testing.T) { checkIntroductionAcceptedCloseFailure(t, test.capacity, test.kind) })
	}
}

func checkIntroductionAcceptedCloseFailure(t *testing.T, capacity bool, kind route.ClosedSharedCarrierKind) {
	t.Helper()
	closeErr := errors.New("accepted introduction Carrier close failed")
	local, peer := net.Pipe()
	defer peer.Close()
	connection := &acceptedCloseFailureConn{Conn: local, closed: make(chan struct{}), err: closeErr}
	listener := &oneAcceptedCarrierListener{ready: make(chan struct{}), connection: connection, kind: kind}
	spends, err := replay.Open(t.TempDir(), replay.Binding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 1})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	server := &closedIntroductionServer{config: runtimeConfig{Config: Config{Current: func() (state.NodeDuty, error) {
		return state.NodeDuty{}, errors.New("current State is unavailable")
	}}}, listener: listener, spends: spends, capacity: make(chan struct{}, 1),
		cancel: cancel, done: make(chan error, 1), drained: make(chan struct{})}
	if capacity {
		server.capacity <- struct{}{} // Force capacity refusal after the first authenticated accept.
	}
	go server.run(ctx)
	t.Cleanup(func() {
		_ = server.stop()
		select {
		case <-server.drained:
		case <-time.After(time.Second):
			t.Error("introduction server did not drain")
		}
	})
	close(listener.ready)
	select {
	case <-connection.closed:
	case <-time.After(time.Second):
		t.Fatal("introduction did not close accepted Carrier")
	}
	_ = server.stop()
	select {
	case <-server.drained:
	case <-time.After(time.Second):
		t.Fatal("introduction server did not drain")
	}
	if !errors.Is(server.drainErr, closeErr) {
		t.Fatalf("Introduction drain lost accepted Carrier close failure: %v", server.drainErr)
	}
}
