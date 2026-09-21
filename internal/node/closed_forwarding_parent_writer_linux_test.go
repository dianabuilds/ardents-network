//go:build linux

package node

import (
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/resource"
	"github.com/dianabuilds/ardents-network/internal/route"
)

// A completed OPEN must not let one child's physical downstream write hold the
// parent reader. The A peer deliberately stops reading after its child OPEN;
// B uses a distinct selected TCP Carrier and must complete before A is released.
func TestClosedForwardingParentReaderServesIndependentChildWhileWriteBlocks(t *testing.T) {
	fixture := newClosedBootstrapFixture(t)
	now := time.Now().UTC().Truncate(time.Hour).Add(time.Hour + 10*time.Minute)
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
	serverCertificate, serverKey := rendezvousCertificate(t, 284, "parent-writer-server")
	aCertificate, aKey := rendezvousCertificate(t, 285, "parent-writer-a")
	bCertificate, bKey := rendezvousCertificate(t, 286, "parent-writer-b")
	aEndpoint := closedForwardingActualCarrierEndpoint(t, route.ClosedCarrierTCP)
	bEndpoint := closedForwardingActualCarrierEndpoint(t, route.ClosedCarrierTCP)
	fixture.snapshot.Candidates[0].Endpoint, fixture.snapshot.Candidates[0].PublicKey = bEndpoint, bKey
	fixture.snapshot.Candidates[1].Endpoint, fixture.snapshot.Candidates[1].PublicKey = aEndpoint, aKey
	fixture.snapshot.Candidates[0].CarrierProfile = string(route.ClosedCarrierTCP)
	fixture.snapshot.Candidates[1].CarrierProfile = string(route.ClosedCarrierTCP)
	fixture.snapshot.CarrierProfile = string(route.ClosedCarrierTCP)
	fixture.snapshot.NodePublicKey = serverKey
	fixture.config.now = func() time.Time { return now }
	fixture.config.Current = func() (DutyView, error) { return fixture.snapshot, nil }
	fixture.config.CurrentClosedProfile = func() (state.ClosedProfileView, bool) { return fixture.view.Profile, true }
	host := &cleanupFailureHost{}
	fixture.config.ClosedForwarding = ClosedForwardingProfile{Certificate: serverCertificate, AdmissionTraffic: resource.HostingTraffic{Tx: 1}, TerminationTraffic: resource.HostingTraffic{Tx: 1}, host: host}
	receiver, ok := closedRouteReceiver(fixture.config, fixture.snapshot, route.ClosedPurposeForwarding, now)
	if !ok {
		t.Fatal("receiver unavailable")
	}
	aListener, err := route.ListenClosedSharedCarrier(route.ClosedCarrierTCP, aEndpoint, aCertificate, func(key [32]byte) bool { return key == serverKey }, 1)
	if err != nil {
		t.Fatal(err)
	}
	bListener, err := route.ListenClosedSharedCarrier(route.ClosedCarrierTCP, bEndpoint, bCertificate, func(key [32]byte) bool { return key == serverKey }, 1)
	if err != nil {
		_ = aListener.Close()
		t.Fatal(err)
	}
	pool, err := route.NewClosedCarrierPool(func() time.Time { return now })
	if err != nil {
		_ = aListener.Close()
		_ = bListener.Close()
		t.Fatal(err)
	}
	spends, err := route.OpenClosedSpendLedger(t.TempDir(), route.ClosedSpendBinding{NetworkID: receiver.NetworkID, ProfileDigest: receiver.ProfileDigest, ReceiverNodeID: receiver.NodeID, ReceiverDutyGeneration: receiver.DutyGeneration})
	if err != nil {
		_ = aListener.Close()
		_ = bListener.Close()
		_ = pool.Close()
		t.Fatal(err)
	}
	limits, err := route.NewClosedDutyLimits(fixture.config.now)
	if err != nil {
		_ = spends.Close()
		_ = aListener.Close()
		_ = bListener.Close()
		_ = pool.Close()
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	server := &closedForwardingServer{config: fixture.config, certificate: serverCertificate, spends: spends, limits: limits, host: host, pool: pool, sessions: newClosedForwardingSessions(), clock: func() time.Time { return now }}
	releaseA := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(releaseA) }) }
	peerDone := make(chan error, 2)
	releaseB := make(chan struct{})
	var releaseBOnce sync.Once
	releaseBPeer := func() { releaseBOnce.Do(func() { close(releaseB) }) }
	defer func() {
		release()
		releaseBPeer()
		_ = aListener.Close()
		_ = bListener.Close()
		_ = pool.Close()
		workers.Wait()
		_ = server.sessions.joinedResult()
		_ = spends.Close()
	}()
	aOpen, aWriteStarted := make(chan struct{}), make(chan struct{})
	workers.Add(1)
	go func() {
		defer workers.Done()
		accepted, acceptErr := aListener.Accept(t.Context(), 5*time.Second)
		if acceptErr != nil {
			peerDone <- acceptErr
			return
		}
		defer accepted.Connection.Close()
		transport, available := accepted.Connection.(interface{ NetConn() net.Conn })
		if !available {
			peerDone <- errors.New("A TCP transport is unavailable")
			return
		}
		raw, available := transport.NetConn().(*net.TCPConn)
		if !available || raw.SetReadBuffer(1) != nil {
			peerDone <- errors.New("A TCP read bound is unavailable")
			return
		}
		hello, readErr := route.ReadClosedLaneFrame(accepted.Connection)
		if readErr != nil || hello.Kind != 1 {
			peerDone <- errors.New("A HELLO missing")
			return
		}
		accept, frameErr := route.ClosedAcceptFrame(0, 64<<10)
		if frameErr == nil {
			frameErr = route.WriteClosedLaneFrame(accepted.Connection, accept)
		}
		if frameErr != nil {
			peerDone <- frameErr
			return
		}
		child, readErr := route.ReadClosedLaneFrame(accepted.Connection)
		if readErr != nil || child.Kind != 4 {
			peerDone <- errors.New("A child OPEN missing")
			return
		}
		close(aOpen)
		frame, readErr := route.ReadClosedLaneFrame(accepted.Connection)
		if readErr != nil || frame.Kind != 6 || len(frame.Body) != 16<<10 {
			peerDone <- errors.New("A first forwarded bytes missing")
			return
		}
		close(aWriteStarted)
		<-releaseA
		for range 3 {
			frame, readErr := route.ReadClosedLaneFrame(accepted.Connection)
			if readErr != nil || frame.Kind != 6 || len(frame.Body) != 16<<10 {
				peerDone <- errors.New("A blocked bytes missing")
				return
			}
		}
		peerDone <- nil
	}()
	bProgress := make(chan struct{})
	workers.Add(1)
	go func() {
		defer workers.Done()
		accepted, acceptErr := bListener.Accept(t.Context(), 5*time.Second)
		if acceptErr != nil {
			peerDone <- acceptErr
			return
		}
		defer accepted.Connection.Close()
		hello, readErr := route.ReadClosedLaneFrame(accepted.Connection)
		if readErr != nil || hello.Kind != 1 {
			peerDone <- errors.New("B HELLO missing")
			return
		}
		accept, frameErr := route.ClosedAcceptFrame(0, 64<<10)
		if frameErr == nil {
			frameErr = route.WriteClosedLaneFrame(accepted.Connection, accept)
		}
		if frameErr != nil {
			peerDone <- frameErr
			return
		}
		child, readErr := route.ReadClosedLaneFrame(accepted.Connection)
		if readErr != nil || child.Kind != 4 || child.Lane != 1 {
			peerDone <- errors.New("B child OPEN missing")
			return
		}
		close(bProgress)
		<-releaseB
		peerDone <- nil
	}()
	client, done := closedForwardingParentReaderAccepted(t, server, serverKey, receiver, token)
	joined := false
	defer func() {
		_ = client.Close()
		if !joined {
			<-done
		}
	}()
	aOpenFrame := fixture.open
	aOpenFrame.Deadline = now.Add(20 * time.Second)
	aBody, err := route.EncodeClosedOpen(aOpenFrame)
	if err != nil {
		t.Fatal(err)
	}
	if err = route.WriteClosedLaneFrame(client, route.ClosedLaneFrame{Kind: 4, Lane: 1, Body: aBody}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-aOpen:
	case err := <-done:
		joined = true
		t.Fatalf("A OPEN failed: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("A OPEN did not complete")
	}
	for range 4 {
		if err = route.WriteClosedLaneFrame(client, route.ClosedLaneFrame{Kind: 6, Lane: 1, Body: make([]byte, 16<<10)}); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case <-aWriteStarted:
	case err := <-peerDone:
		t.Fatal(err)
	case <-time.After(2 * time.Second):
		t.Fatal("A downstream write did not start")
	}
	bOpen := route.ClosedOpen{NextNodeID: fixture.snapshot.Candidates[0].NodeID, NextDutyGeneration: fixture.view.Nodes[0].DutyGeneration, Purpose: route.ClosedPurposeForwarding, Deadline: now.Add(20 * time.Second)}
	bBody, err := route.EncodeClosedOpen(bOpen)
	if err != nil {
		t.Fatal(err)
	}
	if err = route.WriteClosedLaneFrame(client, route.ClosedLaneFrame{Kind: 4, Lane: 3, Body: bBody}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-bProgress:
	case err := <-peerDone:
		t.Fatal(err)
	case <-time.After(2 * time.Second):
		t.Fatal("B did not progress while A downstream write was held")
	}
	if err = route.WriteClosedLaneFrame(client, route.ClosedLaneFrame{Kind: 2, Lane: 0, Body: append([]byte{2}, closedRestrictionToken(t, fixture)...)}); err != nil {
		t.Fatal(err)
	}
	_ = client.SetReadDeadline(time.Now().Add(time.Second))
	frame, err := route.ReadClosedLaneFrame(client)
	if err != nil || frame.Kind != 5 {
		t.Fatalf("control did not progress while A write was held: %+v / %v", frame, err)
	}
	_ = client.SetReadDeadline(time.Time{})
	release()
	releaseBPeer()
	_ = client.Close()
	select {
	case <-done:
		joined = true
	case <-time.After(3 * time.Second):
		t.Fatal("parent did not join downstream I/O during cleanup")
	}
}
