//go:build linux

package node

import (
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/resource"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/replay"
)

// The parent reader must keep serving bounded lane-zero control while an
// admitted child is waiting for its real downstream HELLO/ACCEPT exchange.
func TestClosedForwardingParentReaderServesControlWhileOpenBlocks(t *testing.T) {
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
	serverCertificate, serverKey := nodeCertificate(t, 281, "parent-reader-server")
	peerCertificate, peerKey := nodeCertificate(t, 282, "parent-reader-peer")
	bCertificate, bKey := nodeCertificate(t, 283, "parent-reader-b")
	endpoint := closedForwardingActualCarrierEndpoint(t, route.ClosedCarrierTCP)
	bEndpoint := closedForwardingActualCarrierEndpoint(t, route.ClosedCarrierTCP)
	fixture.snapshot.Candidates[0].Endpoint, fixture.snapshot.Candidates[0].PublicKey = bEndpoint, bKey
	fixture.snapshot.Candidates[1].Endpoint = endpoint
	fixture.snapshot.Candidates[1].CarrierProfile = string(route.ClosedCarrierTCP)
	fixture.snapshot.Candidates[1].PublicKey = peerKey
	fixture.snapshot.CarrierProfile = string(route.ClosedCarrierTCP)
	fixture.snapshot.NodePublicKey = serverKey
	fixture.config.now = func() time.Time { return now }
	fixture.config.Current = func() (DutyView, error) { return fixture.snapshot, nil }
	fixture.config.CurrentClosedProfile = func() (state.ClosedProfileView, bool) { return fixture.view.Profile, true }
	host := &cleanupFailureHost{}
	fixture.config.ClosedForwarding = ClosedForwardingProfile{Certificate: serverCertificate, AdmissionTraffic: resource.HostingTraffic{Tx: 1}, TerminationTraffic: resource.HostingTraffic{Tx: 1}, host: host}
	receiver, ok := closedRouteReceiver(fixture.config, fixture.snapshot, ardp.PurposeForwarding, now)
	if !ok {
		t.Fatal("receiver unavailable")
	}
	listener, err := route.ListenClosedSharedCarrier(route.ClosedCarrierTCP, endpoint, peerCertificate, func(key [32]byte) bool { return key == serverKey }, 1)
	if err != nil {
		t.Fatal(err)
	}
	bListener, err := route.ListenClosedSharedCarrier(route.ClosedCarrierTCP, bEndpoint, bCertificate, func(key [32]byte) bool { return key == serverKey }, 1)
	if err != nil {
		_ = listener.Close()
		t.Fatal(err)
	}
	pool, err := route.NewClosedCarrierPool(func() time.Time { return now })
	if err != nil {
		_ = listener.Close()
		t.Fatal(err)
	}
	spends, err := replay.Open(t.TempDir(), replay.Binding{NetworkID: receiver.NetworkID, ProfileDigest: receiver.ProfileDigest,
		ReceiverNodeID: receiver.NodeID, ReceiverDutyGeneration: receiver.DutyGeneration})
	if err != nil {
		_ = listener.Close()
		_ = pool.Close()
		t.Fatal(err)
	}
	limits, err := route.NewClosedDutyLimits(fixture.config.now)
	if err != nil {
		_ = spends.Close()
		_ = listener.Close()
		_ = pool.Close()
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	server := &closedForwardingServer{config: fixture.config, certificate: serverCertificate, receiving: &closedForwardingReceivingResources{spends: spends, limits: limits},
		host: host, pool: pool, sessions: newClosedForwardingSessions(), clock: func() time.Time { return now }}
	peerRelease := make(chan struct{})
	defer func() {
		close(peerRelease)
		_ = bListener.Close()
		_ = listener.Close()
		_ = pool.Close()
		workers.Wait()
		_ = server.sessions.joinedResult()
		_ = server.receiving.Close()
	}()
	helloRead := make(chan struct{})
	aClosed := make(chan struct{})
	peerDone := make(chan error, 2)
	workers.Add(1)
	go func() {
		defer workers.Done()
		accepted, acceptErr := listener.Accept(t.Context(), 5*time.Second)
		if acceptErr != nil {
			peerDone <- acceptErr
			return
		}
		defer accepted.Connection.Close()
		frame, readErr := ardp.ReadFrame(accepted.Connection)
		if readErr != nil || frame.Kind != 1 {
			peerDone <- errors.New("blocked downstream HELLO missing")
			return
		}
		close(helloRead)
		_ = accepted.Connection.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, readErr = ardp.ReadFrame(accepted.Connection)
		if readErr == nil || errors.Is(readErr, io.ErrNoProgress) {
			peerDone <- errors.New("A carrier remained open after CLOSE")
			return
		}
		if timeout, ok := readErr.(net.Error); ok && timeout.Timeout() {
			peerDone <- errors.New("A carrier did not close after CLOSE")
			return
		}
		close(aClosed)
		<-peerRelease
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
		if hello, readErr := ardp.ReadFrame(accepted.Connection); readErr != nil || hello.Kind != 1 {
			peerDone <- errors.New("B HELLO missing")
			return
		}
		accept, frameErr := ardp.AcceptFrame(0, 64<<10)
		if frameErr == nil {
			frameErr = ardp.WriteFrame(accepted.Connection, accept)
		}
		if frameErr != nil {
			peerDone <- frameErr
			return
		}
		if child, readErr := ardp.ReadFrame(accepted.Connection); readErr != nil || child.Kind != 4 || child.Lane != 1 {
			peerDone <- errors.New("B child OPEN missing")
			return
		}
		close(bProgress)
		<-peerRelease
	}()
	client, done := closedForwardingParentReaderAccepted(t, server, serverKey, receiver, token)
	joined := false
	defer func() {
		_ = client.Close()
		if !joined {
			<-done
		}
	}()
	open := fixture.open
	open.Deadline = now.Add(20 * time.Second)
	body, err := route.EncodeClosedOpen(open)
	if err != nil {
		t.Fatal(err)
	}
	if err = ardp.WriteFrame(client, ardp.Frame{Kind: 4, Lane: 1, Body: body}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-helloRead:
	case err := <-done:
		joined = true
		t.Fatalf("A opener ended before downstream HELLO: %v", err)
	case <-time.After(6 * time.Second):
		t.Fatal("A did not begin blocked downstream HELLO")
	}
	if err = ardp.WriteFrame(client, ardp.Frame{Kind: 9, Lane: 1, Body: []byte{1}}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-aClosed:
	case err := <-peerDone:
		t.Fatal(err)
	case <-time.After(3 * time.Second):
		t.Fatal("A opener did not close after CLOSE")
	}
	bOpen := route.ClosedOpen{NextNodeID: fixture.snapshot.Candidates[0].NodeID, NextDutyGeneration: fixture.view.Nodes[0].DutyGeneration, Purpose: ardp.PurposeForwarding, Deadline: now.Add(20 * time.Second)}
	bBody, err := route.EncodeClosedOpen(bOpen)
	if err != nil {
		t.Fatal(err)
	}
	if err = ardp.WriteFrame(client, ardp.Frame{Kind: 4, Lane: 3, Body: bBody}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-bProgress:
	case err := <-peerDone:
		t.Fatal(err)
	case <-time.After(time.Second):
		t.Fatal("B did not progress while A blocked")
	}
	if err = ardp.WriteFrame(client, ardp.Frame{Kind: 2, Lane: 0, Body: append([]byte{2}, closedRestrictionToken(t, fixture)...)}); err != nil {
		t.Fatal(err)
	}
	if err = client.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	frame, err := ardp.ReadFrame(client)
	if err != nil || frame.Kind != 5 {
		t.Fatalf("control did not progress past blocked A: %+v / %v", frame, err)
	}
}

func closedForwardingParentReaderAccepted(t *testing.T, server *closedForwardingServer, key [32]byte, receiver route.ClosedRoleReceiver, token []byte) (net.Conn, <-chan error) {
	t.Helper()
	serverRaw, clientRaw := net.Pipe()
	deadline := time.Now().Add(5 * time.Second)
	result := make(chan error, 1)
	go func() {
		secured, err := route.AcceptClosedRoleTLS(t.Context(), serverRaw, server.certificate, deadline)
		if err == nil {
			err = server.serveDirect(t.Context(), secured, nil, [32]byte{}, route.ClosedChildOrdinary, nil)
		}
		result <- err
	}()
	client, err := route.OpenClosedRoleTLS(t.Context(), clientRaw, key, deadline)
	if err != nil {
		_ = clientRaw.Close()
		_ = serverRaw.Close()
		t.Fatalf("inner client TLS: %v / %v", err, <-result)
	}
	hello := ardp.Hello{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest, ProfileDigest: receiver.ProfileDigest,
		RecipientNodeID: receiver.NodeID, RecipientDutyGeneration: receiver.DutyGeneration, Purpose: ardp.PurposeForwarding, ChannelNonce: [32]byte{19}, Deadline: receiver.NotAfter}
	body, err := ardp.EncodeHello(hello)
	if err == nil {
		_ = client.SetWriteDeadline(time.Now().Add(time.Second))
		err = ardp.WriteFrame(client, ardp.Frame{Kind: 1, Body: body})
	}
	if err == nil {
		err = ardp.WriteFrame(client, ardp.Frame{Kind: 2, Body: append([]byte{2}, token...)})
	}
	_ = client.SetWriteDeadline(time.Time{})
	if err != nil {
		_ = client.Close()
		t.Fatalf("forwarding admission write: %v / %v", err, <-result)
	}
	_ = client.SetReadDeadline(time.Now().Add(time.Second))
	frame, err := ardp.ReadFrame(client)
	_ = client.SetReadDeadline(time.Time{})
	if err != nil || frame.Kind != 5 {
		_ = client.Close()
		t.Fatalf("forwarding acceptance: %+v / %v", frame, err)
	}
	return client, result
}
