package node

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// Delay only the return from Read after physical closure. This models a
// descheduled reader; closing a descriptor is not evidence that its owner ran.
type delayedForwardingRead struct {
	net.Conn
	gate        chan struct{}
	interrupted chan struct{}
	once        sync.Once
	closeErr    error
}

func (reader *delayedForwardingRead) Read(body []byte) (int, error) {
	n, err := reader.Conn.Read(body)
	if err != nil {
		reader.once.Do(func() { close(reader.interrupted) })
		<-reader.gate
	}
	return n, err
}

func (reader *delayedForwardingRead) Close() error {
	return errors.Join(reader.Conn.Close(), reader.closeErr)
}

type idleForwardingListener struct{}

func (idleForwardingListener) Accept(ctx context.Context, _ time.Duration) (route.ClosedSharedCarrier, error) {
	<-ctx.Done()
	return route.ClosedSharedCarrier{}, ctx.Err()
}
func (idleForwardingListener) Close() error { return nil }

func TestClosedForwardingDrainRetainsRootUntilOutgoingReaderJoins(t *testing.T) {
	for _, test := range []struct {
		name     string
		closeErr error
	}{
		{"clean", nil}, {"physical-close-failure", errors.New("fixture physical close failed")},
	} {
		t.Run(test.name, func(t *testing.T) { checkForwardingReaderShutdown(t, test.closeErr) })
	}
}

func checkForwardingReaderShutdown(t *testing.T, closeErr error) {
	t.Helper()
	pool, err := route.NewClosedCarrierPool(time.Now)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	binding := route.ClosedSpendBinding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2},
		ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 1}
	spends, err := route.OpenClosedSpendLedger(root, binding)
	if err != nil {
		t.Fatal(err)
	}
	server := newClosedForwardingServer(runtimeConfig{now: time.Now}, dutyFacts{}, tls.Certificate{},
		idleForwardingListener{}, spends, nil, pool, nil, 1)
	local, peer := net.Pipe()
	blocked := &delayedForwardingRead{Conn: local, gate: make(chan struct{}), interrupted: make(chan struct{}), closeErr: closeErr}
	var release sync.Once
	t.Cleanup(func() {
		server.Stop()
		_ = pool.Close()
		_ = peer.Close()
		release.Do(func() { close(blocked.gate) })
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Drain(ctx)
		_ = spends.Close()
	})
	key := route.ClosedCarrierKey{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, LocalNodeID: [32]byte{3},
		PeerNodeID: [32]byte{4}, PeerKey: [32]byte{5}, CarrierProfile: route.ClosedCarrierTCP}
	lease, err := pool.Acquire(key, func() error { return nil }, func() (route.Carrier, error) { return blocked, nil })
	if err != nil {
		t.Fatal(err)
	}
	handshake := make(chan error, 1)
	go func() {
		_, err := route.ReadClosedLaneFrame(peer)
		if err == nil {
			frame, encodeErr := route.ClosedAcceptFrame(0, 64<<10)
			err = encodeErr
			if err == nil {
				err = route.WriteClosedLaneFrame(peer, frame)
			}
		}
		handshake <- err
	}()
	end := time.Now().UTC().Truncate(time.Second).Add(time.Minute)
	_, err = server.sessions.acquire(key, lease, time.Now().Add(time.Second), func() (route.ClosedHello, error) {
		return route.ClosedHello{NetworkID: [32]byte{1}, StateGeneration: [32]byte{2}, StateDigest: [32]byte{3},
			ProfileDigest: [32]byte{4}, RecipientNodeID: [32]byte{5}, RecipientDutyGeneration: 1,
			Purpose: route.ClosedPurposeForwarding, ChannelNonce: [32]byte{6}, Deadline: end}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := <-handshake; err != nil {
		t.Fatal(err)
	}
	if err := lease.MarkUsed(); err != nil {
		t.Fatal(err)
	}
	if err := lease.Release(); err != nil {
		t.Fatal(err)
	}
	server.Stop()
	first, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	if err := server.Drain(first); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Drain completed without the retained reader: %v", err)
	}
	select {
	case <-blocked.interrupted:
	default:
		t.Fatal("Stop did not interrupt the outgoing Carrier")
	}
	if replacement, err := route.OpenClosedSpendLedger(root, binding); err == nil {
		_ = replacement.Close()
		t.Fatal("unjoined reader lost its root lease")
	}
	release.Do(func() { close(blocked.gate) })
	joined, cancelJoined := context.WithTimeout(t.Context(), time.Second)
	defer cancelJoined()
	for range 2 {
		err := server.Drain(joined)
		if closeErr == nil && err != nil || closeErr != nil && !errors.Is(err, closeErr) {
			t.Fatalf("joined cleanup changed its physical close result: %v", err)
		}
	}
	reopened, err := route.OpenClosedSpendLedger(root, binding)
	if err != nil {
		t.Fatalf("joined shutdown retained root: %v", err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
}
