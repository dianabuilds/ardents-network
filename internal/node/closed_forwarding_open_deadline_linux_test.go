//go:build linux

package node

import (
	"context"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"net"
	"sync"
	"testing"
	"time"
)

func TestClosedForwardingOpenExpiresWhileExactCarrierDialWaits(t *testing.T) {
	fixture := newClosedBootstrapFixture(t)
	fixture.now = time.Now().UTC()
	until := fixture.now.Add(time.Hour)
	fixture.view.Profile.NotBefore = fixture.now.Add(-time.Second)
	fixture.view.Profile.NotAfter = until
	fixture.snapshot.EpochValidFrom = fixture.view.Profile.NotBefore
	fixture.snapshot.ValidUntil = until
	fixture.snapshot.RecordValidFrom = fixture.now.Add(-time.Second)
	fixture.snapshot.RecordValidUntil = until
	for index := range fixture.snapshot.Candidates {
		fixture.snapshot.Candidates[index].ValidFrom = fixture.now.Add(-time.Second)
		fixture.snapshot.Candidates[index].ValidUntil = until
		fixture.snapshot.Candidates[index].AssignmentNotAfter = until
	}
	fixture.config.now = time.Now
	fixture.config.Current = func() (state.NodeDuty, error) { return fixture.snapshot, nil }

	candidate, err := closedForwardRecipient(fixture.config, fixture.snapshot, fixture.open, fixture.now)
	if err != nil {
		t.Fatal(err)
	}
	receiver, available := closedRouteReceiver(fixture.config, fixture.snapshot, ardp.PurposeForwarding, fixture.now)
	if !available {
		t.Fatal("forwarding receiver unavailable")
	}
	key := route.ClosedCarrierKey{NetworkID: receiver.NetworkID, ProfileDigest: receiver.ProfileDigest, LocalNodeID: receiver.NodeID,
		PeerNodeID: candidate.NodeID, PeerKey: candidate.PublicKey, CarrierProfile: route.CarrierProfile(candidate.CarrierProfile)}
	local, peer := net.Pipe()
	defer local.Close()
	defer peer.Close()
	pool, err := route.NewClosedCarrierPool(time.Now)
	if err != nil {
		t.Fatal(err)
	}
	opened, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	ownerDone := make(chan error, 1)
	go func() {
		lease, acquireErr := pool.AcquireContext(context.Background(), key, func() error { return nil }, func() (route.Carrier, error) {
			close(opened)
			<-release
			return local, nil
		})
		if acquireErr == nil {
			acquireErr = lease.Release()
		}
		ownerDone <- acquireErr
	}()
	<-opened
	joined := false
	t.Cleanup(func() {
		releaseOnce.Do(func() { close(release) })
		if !joined {
			select {
			case err := <-ownerDone:
				if err != nil {
					t.Error(err)
				}
			case <-time.After(time.Second):
				t.Error("blocked dial did not join")
			}
		}
		if err := pool.Close(); err != nil {
			t.Error(err)
		}
	})
	certificate, _ := nodeCertificate(t, 237, "forwarding-deadline")
	server := &closedForwardingServer{config: fixture.config, certificate: certificate, pool: pool, sessions: newClosedForwardingSessions(), clock: time.Now}
	open := fixture.open
	open.Deadline = time.Now().Add(40 * time.Millisecond)
	started := time.Now()
	openDone := make(chan error, 1)
	go func() {
		_, openErr := server.openForwardingLink(context.Background(), open, route.ClosedChildOrdinary, 1, nil, nil, nil)
		openDone <- openErr
	}()
	var openErr error
	select {
	case openErr = <-openDone:
	case <-time.After(500 * time.Millisecond):
		releaseOnce.Do(func() { close(release) })
		select {
		case openErr = <-openDone:
		case <-time.After(time.Second):
			t.Fatal("forwarding open did not join after blocked dial release")
		}
		t.Fatalf("forwarding open waited %s beyond its handshake deadline", time.Since(started))
	}
	if openErr == nil {
		t.Fatal("forwarding open acquired behind pending exact-key dial")
	}
	releaseOnce.Do(func() { close(release) })
	if err := <-ownerDone; err != nil {
		t.Fatal(err)
	}
	joined = true
}
