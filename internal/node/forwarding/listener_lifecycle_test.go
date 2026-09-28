package forwarding

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/resource"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
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
	server := newServerWithHost(dependencies{}, tls.Certificate{}, listener,
		&receivingResources{spends: spends}, pool, nil, 1, time.Now)
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

func (idleForwardingListener) Accept(ctx context.Context, _ time.Duration) (routecarrier.ClosedSharedCarrier, error) {
	<-ctx.Done()
	return routecarrier.ClosedSharedCarrier{}, ctx.Err()
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
	pool, err := routecarrier.NewClosedCarrierPool(time.Now)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	binding := replay.Binding{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2},
		ReceiverNodeID: [32]byte{3}, ReceiverDutyGeneration: 1}
	spends, err := replay.Open(root, binding)
	if err != nil {
		t.Fatal(err)
	}
	server := newServerWithHost(dependencies{}, tls.Certificate{},
		idleForwardingListener{}, &receivingResources{spends: spends}, pool, nil, 1, time.Now)
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
	key := routecarrier.ClosedCarrierKey{NetworkID: [32]byte{1}, ProfileDigest: [32]byte{2}, LocalNodeID: [32]byte{3},
		PeerNodeID: [32]byte{4}, PeerKey: [32]byte{5}, CarrierProfile: routecarrier.ClosedCarrierTCP}
	lease, err := pool.AcquireContext(context.Background(), key, func() error { return nil }, func() (routecarrier.Carrier, error) { return blocked, nil })
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
	if replacement, err := replay.Open(root, binding); err == nil {
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
	if replacement, err := replay.Open(root, binding); err == nil {
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
	reopened, err := replay.Open(root, binding)
	if err != nil {
		t.Fatalf("joined shutdown retained root: %v", err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
}

type sharedHostingSampleFixture struct {
	sampled chan struct{}
	once    sync.Once
}

func (host *sharedHostingSampleFixture) Sample(context.Context, time.Duration) (resource.HostingSample, error) {
	host.once.Do(func() { close(host.sampled) })
	return resource.HostingSample{}, nil
}

func (*sharedHostingSampleFixture) Close() error { return nil }

type sharedHostingSampleListener struct {
	closed chan struct{}
	once   sync.Once
}

func (listener *sharedHostingSampleListener) Accept(ctx context.Context, _ time.Duration) (routecarrier.ClosedSharedCarrier, error) {
	select {
	case <-ctx.Done():
		return routecarrier.ClosedSharedCarrier{}, ctx.Err()
	case <-listener.closed:
		return routecarrier.ClosedSharedCarrier{}, context.Canceled
	}
}

func (listener *sharedHostingSampleListener) Close() error {
	listener.once.Do(func() { close(listener.closed) })
	return nil
}

// Periodic Node observation shares the recently committed whole-host sample.
// It must not create another exclusive writer per local duty: the qualification
// host runs several independently supervised duties against this one ledger.
func TestClosedForwardingReaperSharesRecentHostingSample(t *testing.T) {
	host := &sharedHostingSampleFixture{sampled: make(chan struct{})}
	listener := &sharedHostingSampleListener{closed: make(chan struct{})}
	pool, err := routecarrier.NewClosedCarrierPool(time.Now)
	if err != nil {
		t.Fatal(err)
	}
	_, cancel := context.WithCancel(context.Background())
	server := &forwardServer{host: host, listener: listener, pool: pool, stopped: make(chan struct{}), cancel: cancel}
	server.workers.Add(1)
	joined := make(chan struct{})
	go func() {
		server.reap()
		close(joined)
	}()
	select {
	case <-host.sampled:
	case <-server.stopped:
		t.Fatal("periodic shared-host observation stopped the Node listener")
	case <-time.After(3 * time.Second):
		t.Fatal("periodic shared-host observation did not run")
	}
	select {
	case <-server.stopped:
		t.Fatal("fresh shared-host sample stopped the Node listener")
	default:
	}
	server.Stop()
	select {
	case <-joined:
	case <-time.After(2 * time.Second):
		t.Fatal("periodic shared-host observer did not join")
	}
	if err := pool.Close(); err != nil {
		t.Fatal(err)
	}
}
