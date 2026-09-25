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
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
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

func TestClosedForwardingDrainJoinsLateSessionProducerBeforeReader(t *testing.T) {
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
	server := newClosedForwardingServerWithHost(runtimeConfig{now: time.Now}, dutyFacts{}, tls.Certificate{},
		idleForwardingListener{}, &closedForwardingReceivingResources{spends: spends}, pool, nil, 1)
	local, peer := net.Pipe()
	blocked := &delayedForwardingRead{Conn: local, gate: make(chan struct{}), interrupted: make(chan struct{}), closeErr: closeErr}
	allowAccept := make(chan struct{})
	allowProducer := make(chan struct{})
	var releaseReader, releaseAccept, releaseProducer sync.Once
	t.Cleanup(func() {
		server.Stop()
		releaseAccept.Do(func() { close(allowAccept) })
		releaseProducer.Do(func() { close(allowProducer) })
		_ = pool.Close()
		_ = peer.Close()
		releaseReader.Do(func() { close(blocked.gate) })
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Drain(ctx)
		_ = spends.Close()
	})
	key := route.ClosedCarrierKey{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, LocalNodeID: [32]byte{3},
		PeerNodeID: [32]byte{4}, PeerKey: [32]byte{5}, CarrierProfile: route.ClosedCarrierTCP}
	lease, err := pool.AcquireContext(context.Background(), key, func() error { return nil }, func() (route.Carrier, error) { return blocked, nil })
	if err != nil {
		t.Fatal(err)
	}
	helloRead := make(chan error, 1)
	handshake := make(chan error, 1)
	go func() {
		_, err := ardp.ReadFrame(peer)
		helloRead <- err
		if err == nil {
			<-allowAccept
			frame, encodeErr := ardp.AcceptFrame(0, 64<<10)
			err = encodeErr
			if err == nil {
				err = ardp.WriteFrame(peer, frame)
			}
		}
		handshake <- err
	}()
	end := time.Now().UTC().Truncate(time.Second).Add(time.Minute)
	producerResult := make(chan error, 1)
	producerDone := make(chan struct{})
	server.workers.Add(1)
	go func() {
		defer server.workers.Done()
		_, acquireErr := server.sessions.acquire(context.Background(), key, lease, time.Now().Add(time.Second), func() (ardp.Hello, error) {
			return ardp.Hello{NetworkID: [32]byte{1}, StateGeneration: [32]byte{2}, StateDigest: [32]byte{3},
				ProfileDigest: [32]byte{4}, RecipientNodeID: [32]byte{5}, RecipientDutyGeneration: 1,
				Purpose: ardp.PurposeForwarding, ChannelNonce: [32]byte{6}, Deadline: end}, nil
		})
		producerResult <- acquireErr
		<-allowProducer
		close(producerDone)
	}()
	if err := <-helloRead; err != nil {
		t.Fatal(err)
	}
	releaseAccept.Do(func() { close(allowAccept) })
	if err := <-handshake; err != nil {
		t.Fatal(err)
	}
	if err := <-producerResult; err != nil {
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
	releaseProducer.Do(func() { close(allowProducer) })
	select {
	case <-producerDone:
	case <-time.After(time.Second):
		t.Fatal("late successful handshake producer did not join")
	}
	readerOnly, cancelReaderOnly := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancelReaderOnly()
	if err := server.Drain(readerOnly); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Drain completed without the retained reader after producer join: %v", err)
	}
	if replacement, err := route.OpenClosedSpendLedger(root, binding); err == nil {
		_ = replacement.Close()
		t.Fatal("unjoined reader lost its root lease after producer join")
	}
	releaseReader.Do(func() { close(blocked.gate) })
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
