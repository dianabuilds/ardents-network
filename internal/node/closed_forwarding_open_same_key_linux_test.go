//go:build linux

package node

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
)

// TestClosedForwardingOpenSameKeySharesOneActualOuterHello holds the first
// actual outer exchange before ACCEPT. A second caller must borrow a distinct
// pool lease, wait for that one creator, then receive the same session without
// opening another Carrier or writing another outer HELLO.
func TestClosedForwardingOpenSameKeySharesOneActualOuterHello(t *testing.T) {
	for _, profile := range []route.CarrierProfile{route.ClosedCarrierTCP, route.ClosedCarrierQUIC} {
		t.Run(string(profile), func(t *testing.T) {
			testClosedForwardingOpenSameKeyActualCarrier(t, profile)
		})
	}
}

func testClosedForwardingOpenSameKeyActualCarrier(t *testing.T, profile route.CarrierProfile) {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	deadline := now.Add(5 * time.Second)
	fixture := newClosedBootstrapFixture(t)
	fixture.now = now
	fixture.view.Profile.NotBefore = now.Add(-time.Second)
	fixture.view.Profile.NotAfter = now.Add(time.Minute)
	fixture.snapshot.EpochValidFrom = fixture.view.Profile.NotBefore
	fixture.snapshot.ValidUntil = fixture.view.Profile.NotAfter
	fixture.snapshot.RecordValidFrom = fixture.view.Profile.NotBefore
	fixture.snapshot.RecordValidUntil = fixture.view.Profile.NotAfter
	for index := range fixture.snapshot.Candidates {
		fixture.snapshot.Candidates[index].ValidFrom = fixture.view.Profile.NotBefore
		fixture.snapshot.Candidates[index].ValidUntil = fixture.view.Profile.NotAfter
		fixture.snapshot.Candidates[index].AssignmentNotAfter = fixture.view.Profile.NotAfter
	}
	clientCertificate, clientKey := nodeCertificate(t, 278, "forwarding-same-key-client")
	peerCertificate, peerKey := nodeCertificate(t, 279, "forwarding-same-key-peer")
	endpoint := closedForwardingActualCarrierEndpoint(t, profile)
	fixture.snapshot.Candidates[1].Endpoint = endpoint
	fixture.snapshot.Candidates[1].CarrierProfile = string(profile)
	fixture.snapshot.Candidates[1].PublicKey = peerKey
	fixture.config.now = time.Now
	fixture.config.Current = func() (DutyView, error) { return fixture.snapshot, nil }
	fixture.config.CurrentClosedProfile = func() (state.ClosedProfileView, bool) { return fixture.view.Profile, true }

	listener, err := route.ListenClosedSharedCarrier(profile, endpoint, peerCertificate, func(key [32]byte) bool { return key == clientKey }, 1)
	if err != nil {
		t.Fatal(err)
	}
	pool, err := route.NewClosedCarrierPool(time.Now)
	if err != nil {
		_ = listener.Close()
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	server := &closedForwardingServer{config: fixture.config, certificate: clientCertificate, pool: pool, sessions: newClosedForwardingSessions(), clock: time.Now}
	open := fixture.open
	open.Deadline = deadline
	releaseAccept := make(chan struct{})
	releasePeer := make(chan struct{})
	var releaseAcceptOnce, releasePeerOnce sync.Once
	releaseAcceptNow := func() { releaseAcceptOnce.Do(func() { close(releaseAccept) }) }
	releasePeerNow := func() { releasePeerOnce.Do(func() { close(releasePeer) }) }
	helloRead := make(chan struct{})
	peerDone := make(chan error, 1)
	workers.Add(1)
	go func() {
		defer workers.Done()
		accepted, acceptErr := listener.Accept(t.Context(), 10*time.Second)
		if acceptErr != nil {
			peerDone <- acceptErr
			return
		}
		connection := accepted.Connection
		defer connection.Close()
		if accepted.Kind != route.ClosedSharedNode || accepted.NodeKey != clientKey {
			peerDone <- errors.New("same-key actual Carrier lost Node authentication")
			return
		}
		hello, readErr := ardp.ReadFrame(connection)
		if readErr != nil || hello.Kind != 1 || hello.Lane != 0 {
			peerDone <- errors.New("same-key actual outer HELLO was not received")
			return
		}
		if _, readErr = ardp.DecodeHello(hello.Body); readErr != nil {
			peerDone <- readErr
			return
		}
		close(helloRead)
		<-releaseAccept
		accept, frameErr := ardp.AcceptFrame(0, 64<<10)
		if frameErr == nil {
			frameErr = ardp.WriteFrame(connection, accept)
		}
		if frameErr != nil {
			peerDone <- frameErr
			return
		}
		lanes := make(map[uint32]struct{}, 2)
		for range 2 {
			child, childErr := ardp.ReadFrame(connection)
			if childErr != nil || child.Kind != 4 || child.Lane == 0 {
				peerDone <- fmt.Errorf("same-key actual child OPEN = %+v / %w", child, childErr)
				return
			}
			received, restriction, decodeErr := route.DecodeClosedNodeOpen(child.Body)
			if decodeErr != nil || restriction != route.ClosedChildOrdinary || received != open {
				peerDone <- errors.New("same-key actual child OPEN was altered")
				return
			}
			if _, duplicate := lanes[child.Lane]; duplicate {
				peerDone <- errors.New("same-key actual child lane was reused")
				return
			}
			lanes[child.Lane] = struct{}{}
		}
		for lane := range lanes {
			if childErr := ardp.WriteFrame(connection, ardp.Frame{Kind: 6, Lane: lane, Body: []byte("same-key-response")}); childErr != nil {
				peerDone <- childErr
				return
			}
		}
		<-releasePeer
		peerDone <- nil
	}()

	type openResult struct {
		link *closedForwardingLink
		err  error
	}
	results := make(chan openResult, 2)
	started := 0
	received := 0
	var first, second *closedForwardingLink
	defer func() {
		releaseAcceptNow()
		_ = listener.Close()
		_ = pool.Close()
		if first != nil {
			_ = first.close()
		}
		if second != nil {
			_ = second.close()
		}
		for range started - received {
			opened := <-results
			if opened.link != nil && opened.link != first && opened.link != second {
				_ = opened.link.close()
			}
		}
		releasePeerNow()
		workers.Wait()
		_ = server.sessions.joinedResult()
	}()

	responses := make(chan ardp.Frame, 2)
	firstChannel := closedForwardingActualChannel(t, deadline, open)
	secondChannel := closedForwardingActualChannel(t, deadline, open)
	openCaller := func(channel *route.ClosedForwardingChannel) {
		link, openErr := server.openForwardingLink(context.Background(), open, route.ClosedChildOrdinary, 1,
			channel, func(frame ardp.Frame) error { responses <- frame; return nil }, func() {})
		results <- openResult{link: link, err: openErr}
	}
	started++
	workers.Add(1)
	go func() { defer workers.Done(); openCaller(firstChannel) }()
	select {
	case <-helloRead:
	case <-time.After(time.Second):
		t.Fatal("first caller did not write its actual outer HELLO")
	}
	started++
	workers.Add(1)
	go func() { defer workers.Done(); openCaller(secondChannel) }()
	select {
	case opened := <-results:
		received++
		if opened.link != nil {
			_ = opened.link.close()
		}
		t.Fatalf("same-key caller returned before ACCEPT: link %p error %v", opened.link, opened.err)
	case <-time.After(50 * time.Millisecond):
	}
	releaseAcceptNow()
	firstResult := <-results
	secondResult := <-results
	received += 2
	if firstResult.err != nil || secondResult.err != nil || firstResult.link == nil || secondResult.link == nil {
		if firstResult.link != nil {
			_ = firstResult.link.close()
		}
		if secondResult.link != nil {
			_ = secondResult.link.close()
		}
		t.Fatalf("same-key actual opens = (%p, %v), (%p, %v)", firstResult.link, firstResult.err, secondResult.link, secondResult.err)
	}
	first, second = firstResult.link, secondResult.link
	if first.session != second.session {
		t.Fatalf("same-key actual sessions differ: %p / %p", first.session, second.session)
	}
	if first.lease == second.lease || !first.lease.SameCarrier(second.lease) {
		t.Fatalf("same-key actual leases were not independent shares: %p / %p", first.lease, second.lease)
	}
	if first.remoteLane == second.remoteLane {
		t.Fatalf("same-key actual children share lane %d", first.remoteLane)
	}
	for range 2 {
		select {
		case frame := <-responses:
			if frame.Kind != 6 || frame.Lane != 1 || string(frame.Body) != "same-key-response" {
				t.Fatalf("same-key actual child response = %+v", frame)
			}
		case <-time.After(time.Second):
			t.Fatal("same-key actual child response was not delivered")
		}
	}
	if err := first.close(); err != nil {
		t.Fatal(err)
	}
	first = nil
	if err := second.close(); err != nil {
		t.Fatal(err)
	}
	second = nil
	releasePeerNow()
	if err := <-peerDone; err != nil {
		t.Fatal(err)
	}
	workers.Wait()
	if err := server.sessions.joinedResult(); err != nil {
		t.Fatal(err)
	}
	server.sessions.mu.Lock()
	pending, published := len(server.sessions.pending), len(server.sessions.sessions)
	server.sessions.mu.Unlock()
	// Neither child wrote application bytes, so each released lease is unused
	// and the pool closes this test-only Carrier. Its reader retires the
	// published session before the explicit reader join completes.
	if pending != 0 || published != 0 {
		t.Fatalf("same-key actual session state = pending %d published %d", pending, published)
	}
}
