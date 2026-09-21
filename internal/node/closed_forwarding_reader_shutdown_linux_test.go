//go:build linux

package node

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/resource"
	"github.com/dianabuilds/ardents-network/internal/route"
)

type queuedForwardingListener struct {
	accepted chan route.ClosedSharedCarrier
	closed   chan struct{}
	once     sync.Once
}

func (listener *queuedForwardingListener) Accept(ctx context.Context, _ time.Duration) (route.ClosedSharedCarrier, error) {
	select {
	case accepted := <-listener.accepted:
		return accepted, nil
	case <-listener.closed:
		return route.ClosedSharedCarrier{}, context.Canceled
	case <-ctx.Done():
		return route.ClosedSharedCarrier{}, ctx.Err()
	}
}

func (listener *queuedForwardingListener) Close() error {
	listener.once.Do(func() { close(listener.closed) })
	return nil
}

// Block only the child OPEN write. The successful outer HELLO has already
// published its session reader, while the accepted production caller still
// owns and must join the child opener.
type delayedForwardingChildWrite struct {
	*delayedForwardingRead
	gate    chan struct{}
	started chan struct{}
	once    sync.Once
	writes  int
}

func (writer *delayedForwardingChildWrite) Write(body []byte) (int, error) {
	writer.writes++
	if writer.writes == 2 {
		writer.once.Do(func() { close(writer.started) })
		<-writer.gate
	}
	return writer.Conn.Write(body)
}

// Exercise serve -> serveAccepted -> serveDirect -> openings.start ->
// openForwardingLink. Stop interrupts the pool while the real accepted worker
// is joining a late child opener; only then may shutdown wait for the session
// reader published by that opener's successful outer HELLO.
func TestClosedForwardingDrainJoinsActualAcceptedProducerBeforeReader(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Hour).Add(time.Hour + 10*time.Minute)
	fixture := newClosedBootstrapFixture(t)
	fixture.now = now
	fixture.view.Profile.NotBefore = now.Add(-time.Second)
	fixture.view.Profile.NotAfter = now.Add(time.Minute)
	token := closedRestrictionToken(t, fixture)
	fixture.snapshot.EpochValidFrom = fixture.view.Profile.NotBefore
	fixture.snapshot.ValidUntil = fixture.view.Profile.NotAfter
	fixture.snapshot.RecordValidFrom = fixture.view.Profile.NotBefore
	fixture.snapshot.RecordValidUntil = fixture.view.Profile.NotAfter
	for index := range fixture.snapshot.Candidates {
		fixture.snapshot.Candidates[index].ValidFrom = fixture.view.Profile.NotBefore
		fixture.snapshot.Candidates[index].ValidUntil = fixture.view.Profile.NotAfter
		fixture.snapshot.Candidates[index].AssignmentNotAfter = fixture.view.Profile.NotAfter
	}
	serverCertificate, serverKey := rendezvousCertificate(t, 291, "reader-shutdown-server")
	fixture.snapshot.CarrierProfile = string(route.ClosedCarrierTCP)
	fixture.snapshot.NodePublicKey = serverKey
	fixture.config.now = func() time.Time { return now }
	fixture.config.Current = func() (DutyView, error) { return fixture.snapshot, nil }
	fixture.config.CurrentClosedProfile = func() (state.ClosedProfileView, bool) { return fixture.view.Profile, true }
	host := &cleanupFailureHost{}
	fixture.config.ClosedForwarding = ClosedForwardingProfile{Certificate: serverCertificate,
		AdmissionTraffic: resource.HostingTraffic{Tx: 1}, TerminationTraffic: resource.HostingTraffic{Tx: 1}, host: host}
	open := fixture.open
	open.Deadline = now.Add(20 * time.Second)
	candidate, err := closedForwardRecipient(fixture.config, fixture.snapshot, open, now)
	if err != nil {
		t.Fatal(err)
	}
	receiver, available := closedRouteReceiver(fixture.config, fixture.snapshot, route.ClosedPurposeForwarding, now)
	if !available {
		t.Fatal("forwarding receiver unavailable")
	}
	key := route.ClosedCarrierKey{NetworkID: receiver.NetworkID, ProfileDigest: receiver.ProfileDigest, LocalNodeID: receiver.NodeID,
		PeerNodeID: candidate.NodeID, PeerKey: candidate.PublicKey, CarrierProfile: route.CarrierProfile(candidate.CarrierProfile)}
	pool, err := route.NewClosedCarrierPool(fixture.config.now)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	binding := route.ClosedSpendBinding{NetworkID: receiver.NetworkID, ProfileDigest: receiver.ProfileDigest,
		ReceiverNodeID: receiver.NodeID, ReceiverDutyGeneration: receiver.DutyGeneration}
	spends, err := route.OpenClosedSpendLedger(root, binding)
	if err != nil {
		t.Fatal(err)
	}
	limits, err := route.NewClosedDutyLimits(fixture.config.now)
	if err != nil {
		t.Fatal(err)
	}
	listener := &queuedForwardingListener{accepted: make(chan route.ClosedSharedCarrier), closed: make(chan struct{})}
	server := newClosedForwardingServerWithHost(fixture.config, fixture.snapshot, serverCertificate, listener, spends, limits, pool, nil, host, 1)

	local, peer := net.Pipe()
	closeErr := errors.New("fixture physical close failed")
	blockedRead := &delayedForwardingRead{Conn: local, gate: make(chan struct{}), interrupted: make(chan struct{}), closeErr: closeErr}
	blockedChild := &delayedForwardingChildWrite{delayedForwardingRead: blockedRead, gate: make(chan struct{}), started: make(chan struct{})}
	seed, err := pool.AcquireContext(t.Context(), key, func() error { return nil }, func() (route.Carrier, error) { return blockedChild, nil })
	if err != nil {
		t.Fatal(err)
	}

	serverRaw, clientRaw := net.Pipe()
	tlsDone := make(chan error, 1)
	go func() {
		secured, acceptErr := route.AcceptClosedRoleTLS(context.Background(), serverRaw, serverCertificate, time.Now().Add(5*time.Second))
		if acceptErr == nil {
			select {
			case listener.accepted <- route.ClosedSharedCarrier{Kind: route.ClosedSharedDirect, Connection: secured}:
			case <-listener.closed:
				acceptErr = context.Canceled
			}
		}
		tlsDone <- acceptErr
	}()
	client, err := route.OpenClosedRoleTLS(t.Context(), clientRaw, serverKey, time.Now().Add(5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err := <-tlsDone; err != nil {
		t.Fatal(err)
	}

	peerDone := make(chan error, 1)
	accept := mustClosedForwardAccept(t)
	go func() {
		frame, peerErr := route.ReadClosedLaneFrame(peer)
		if peerErr == nil && (frame.Kind != 1 || frame.Lane != 0) {
			peerErr = errors.New("outer HELLO was altered")
		}
		if peerErr == nil {
			peerErr = route.WriteClosedLaneFrame(peer, accept)
		}
		peerDone <- peerErr
	}()
	var releaseChild, releaseReader sync.Once
	joinedPeer := false
	t.Cleanup(func() {
		releaseChild.Do(func() { close(blockedChild.gate) })
		releaseReader.Do(func() { close(blockedRead.gate) })
		_ = client.Close()
		_ = peer.Close()
		_ = seed.Release()
		_ = server.Stop()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Drain(ctx)
		if !joinedPeer {
			select {
			case <-peerDone:
			case <-time.After(time.Second):
				t.Error("forwarding peer did not join")
			}
		}
		_ = spends.Close()
	})

	hello := route.ClosedHello{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest,
		ProfileDigest: receiver.ProfileDigest, RecipientNodeID: receiver.NodeID, RecipientDutyGeneration: receiver.DutyGeneration,
		Purpose: route.ClosedPurposeForwarding, ChannelNonce: [32]byte{19}, Deadline: receiver.NotAfter}
	helloBody, err := route.EncodeClosedHello(hello)
	if err == nil {
		err = route.WriteClosedLaneFrame(client, route.ClosedLaneFrame{Kind: 1, Body: helloBody})
	}
	if err == nil {
		err = route.WriteClosedLaneFrame(client, route.ClosedLaneFrame{Kind: 2, Body: append([]byte{2}, token...)})
	}
	if err != nil {
		t.Fatal(err)
	}
	if frame, readErr := route.ReadClosedLaneFrame(client); readErr != nil || frame.Kind != 5 {
		t.Fatalf("forwarding acceptance: %+v / %v", frame, readErr)
	}
	openBody, err := route.EncodeClosedOpen(open)
	if err != nil {
		t.Fatal(err)
	}
	if err := route.WriteClosedLaneFrame(client, route.ClosedLaneFrame{Kind: 4, Lane: 1, Body: openBody}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-blockedChild.started:
	case <-time.After(time.Second):
		t.Fatal("production child opener did not reach its delayed write")
	}
	if err := server.Stop(); err != nil {
		t.Fatal(err)
	}
	first, cancelFirst := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancelFirst()
	if err := server.Drain(first); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Drain completed before accepted producer join: %v", err)
	}
	assertClosedForwardingRootHeld(t, root, binding, "live accepted producer")

	releaseChild.Do(func() { close(blockedChild.gate) })
	deadline := time.Now().Add(time.Second)
	for server.Active() != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if server.Active() != 0 {
		t.Fatal("accepted production caller did not join")
	}
	readerOnly, cancelReaderOnly := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancelReaderOnly()
	if err := server.Drain(readerOnly); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Drain completed before delayed reader join: %v", err)
	}
	assertClosedForwardingRootHeld(t, root, binding, "delayed session reader")
	select {
	case <-blockedRead.interrupted:
	case <-time.After(time.Second):
		t.Fatal("Stop did not interrupt the outgoing Carrier")
	}
	releaseReader.Do(func() { close(blockedRead.gate) })
	if err := <-peerDone; err != nil {
		t.Fatal(err)
	}
	joinedPeer = true
	joined, cancelJoined := context.WithTimeout(t.Context(), time.Second)
	defer cancelJoined()
	for range 2 {
		if err := server.Drain(joined); !errors.Is(err, closeErr) {
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

func assertClosedForwardingRootHeld(t *testing.T, root string, binding route.ClosedSpendBinding, owner string) {
	t.Helper()
	if replacement, err := route.OpenClosedSpendLedger(root, binding); err == nil {
		_ = replacement.Close()
		t.Fatalf("%s lost its spend root", owner)
	}
}
