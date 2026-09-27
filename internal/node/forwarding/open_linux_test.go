//go:build linux

package forwarding

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/carrier"
)

// This is the caller boundary: TCP/TLS and QUIC both cross the real shared
// listener, outer HELLO/ACCEPT exchange, and child OPEN grammar. The State
// projection only supplies the selected peer; it does not replace transport.
func TestClosedForwardingOpenActualCarrierOutcomes(t *testing.T) {
	for _, profile := range []carrier.CarrierProfile{carrier.ClosedCarrierTCP, carrier.ClosedCarrierQUIC} {
		for _, outcome := range []string{"success", "refusal", "cancellation"} {
			t.Run(string(profile)+"/"+outcome, func(t *testing.T) {
				testClosedForwardingOpenActualCarrierOutcome(t, profile, outcome)
			})
		}
	}
}

func testClosedForwardingOpenActualCarrierOutcome(t *testing.T, profile carrier.CarrierProfile, outcome string) {
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
	clientCertificate, clientKey := nodeCertificate(t, 276, "forwarding-open-client")
	peerCertificate, peerKey := nodeCertificate(t, 277, "forwarding-open-peer")
	endpoint := closedForwardingActualCarrierEndpoint(t, profile)
	fixture.snapshot.Candidates[1].Endpoint = endpoint
	fixture.snapshot.Candidates[1].CarrierProfile = string(profile)
	fixture.snapshot.Candidates[1].PublicKey = peerKey
	fixture.config.now = time.Now
	fixture.config.Current = func() (state.NodeDuty, error) { return fixture.snapshot, nil }
	fixture.config.CurrentClosedProfile = func() (state.ClosedProfileView, bool) { return fixture.view.Profile, true }
	listener, err := carrier.ListenClosedSharedCarrier(profile, endpoint, peerCertificate, func(key [32]byte) bool { return key == clientKey }, 1)
	if err != nil {
		t.Fatal(err)
	}
	pool, err := carrier.NewClosedCarrierPool(time.Now)
	if err != nil {
		_ = listener.Close()
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	server := &forwardServer{dependencies: forwardingDependencies(fixture.config, nil), certificate: clientCertificate, pool: pool, sessions: newSessionSet(), clock: time.Now}
	open := fixture.open
	open.Deadline = deadline
	releasePeer := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(releasePeer) }) }
	peerDone := make(chan error, 1)
	helloRead := make(chan struct{})
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
		if accepted.Kind != carrier.ClosedSharedNode || accepted.NodeKey != clientKey {
			peerDone <- errors.New("actual carrier lost Node authentication")
			return
		}
		hello, readErr := ardp.ReadFrame(connection)
		if readErr != nil || hello.Kind != 1 || hello.Lane != 0 {
			peerDone <- errors.New("actual outer HELLO was not received")
			return
		}
		if _, readErr = ardp.DecodeHello(hello.Body); readErr != nil {
			peerDone <- readErr
			return
		}
		close(helloRead)
		if outcome == "cancellation" {
			<-releasePeer
			peerDone <- nil
			return
		}
		status := uint8(0)
		if outcome == "refusal" {
			status = 1
		}
		credit := uint32(64 << 10)
		if status != 0 {
			credit = 0
		}
		accept, frameErr := ardp.AcceptFrame(status, credit)
		if frameErr == nil {
			frameErr = ardp.WriteFrame(connection, accept)
		}
		if frameErr != nil {
			peerDone <- frameErr
			return
		}
		if outcome == "refusal" {
			// Retain the TCP/QUIC carrier until the caller consumed the refusal.
			// A QUIC close can discard an otherwise valid terminal control tail.
			<-releasePeer
			peerDone <- nil
			return
		}
		child, readErr := ardp.ReadFrame(connection)
		if readErr != nil || child.Kind != 4 || child.Lane != 1 {
			peerDone <- errors.New("actual child OPEN was not received")
			return
		}
		if received, restriction, decodeErr := route.DecodeClosedNodeOpen(child.Body); decodeErr != nil || restriction != route.ClosedChildOrdinary || received != open {
			peerDone <- errors.New("actual child OPEN was altered")
			return
		}
		if frameErr = ardp.WriteFrame(connection, ardp.Frame{Kind: 6, Lane: child.Lane, Body: []byte("child-response")}); frameErr != nil {
			peerDone <- frameErr
			return
		}
		// Keep a QUIC tail alive until the caller consumed it; closing the
		// connection earlier may discard an otherwise valid response.
		<-releasePeer
		peerDone <- nil
	}()
	t.Cleanup(func() {
		release()
		_ = listener.Close()
		_ = pool.Close()
		workers.Wait()
		_ = server.sessions.joinedResult()
	})
	ctx := t.Context()
	cancel := func() {}
	cancelJoined := make(chan struct{})
	if outcome == "cancellation" {
		ctx, cancel = context.WithCancel(ctx)
		defer cancel()
		go func() {
			defer close(cancelJoined)
			select {
			case <-helloRead:
				cancel()
			case <-ctx.Done():
			}
		}()
	} else {
		close(cancelJoined)
	}
	t.Cleanup(func() {
		cancel()
		<-cancelJoined
	})
	outputs := make(chan ardp.Frame, 1)
	type openResult struct {
		link *forwardLink
		err  error
	}
	result := make(chan openResult, 1)
	channel := closedForwardingActualChannel(t, deadline, open)
	workers.Add(1)
	go func() {
		defer workers.Done()
		link, openErr := server.openForwardingLink(ctx, open, route.ClosedChildOrdinary, 1, channel, func(frame ardp.Frame) error {
			outputs <- frame
			return nil
		}, func() {})
		result <- openResult{link: link, err: openErr}
	}()
	var opened openResult
	if outcome == "cancellation" {
		select {
		case opened = <-result:
			if opened.link != nil || !errors.Is(opened.err, context.Canceled) {
				if opened.link != nil {
					_ = opened.link.close()
				}
				t.Fatalf("actual cancellation = link %p error %v", opened.link, opened.err)
			}
		case <-time.After(time.Second):
			cancel()
			release()
			opened = <-result
			if opened.link != nil {
				_ = opened.link.close()
			}
			t.Fatal("actual cancellation waited beyond its bounded handshake")
		}
	} else {
		opened = <-result
	}
	link, openErr := opened.link, opened.err
	if link != nil {
		t.Cleanup(func() { _ = link.close() })
	}
	if outcome == "success" {
		if openErr != nil || link == nil {
			t.Fatalf("actual successful open = link %p error %v", link, openErr)
		}
		select {
		case frame := <-outputs:
			if frame.Kind != 6 || frame.Lane != 1 || string(frame.Body) != "child-response" {
				t.Fatalf("actual child response = %+v", frame)
			}
		case <-time.After(time.Second):
			t.Fatal("actual child response was not delivered")
		}
		if err := link.close(); err != nil {
			t.Fatal(err)
		}
	} else if outcome == "refusal" && (link != nil || openErr == nil || openErr.Error() != "closed forwarding outer HELLO is unavailable") {
		t.Fatalf("actual %s open = link %p error %v", outcome, link, openErr)
	}
	release()
	<-cancelJoined
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
	if pending != 0 || published != 0 {
		t.Fatalf("actual %s retained invalid session: pending=%d sessions=%d", outcome, pending, published)
	}
}

func closedForwardingActualCarrierEndpoint(t *testing.T, profile carrier.CarrierProfile) string {
	t.Helper()
	if profile == carrier.ClosedCarrierTCP {
		return reserveAddress(t)
	}
	packet, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	endpoint := packet.LocalAddr().String()
	if err := packet.Close(); err != nil {
		t.Fatal(err)
	}
	return endpoint
}

func closedForwardingActualChannel(t *testing.T, deadline time.Time, open route.ClosedOpen) *route.ClosedForwardingChannel {
	t.Helper()
	clock := time.Now
	limits, err := route.NewClosedDutyLimits(clock)
	if err != nil {
		t.Fatal(err)
	}
	bootstrap, err := route.NewClosedBootstrapController(clock)
	if err != nil {
		t.Fatal(err)
	}
	reservation, err := bootstrap.Admit([32]byte{91}, deadline)
	if err != nil {
		t.Fatal(err)
	}
	channel, err := route.NewClosedBootstrapForwardingChannel(reservation, limits, func(route.ClosedOpen) error { return nil }, clock)
	if err != nil {
		reservation.Release()
		t.Fatal(err)
	}
	body, err := route.EncodeClosedOpen(open)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = channel.Accept(ardp.Frame{Kind: 4, Lane: 1, Body: body}); err != nil {
		t.Fatal(err)
	}
	if _, available := channel.NextAvailable(nil); !available {
		t.Fatal("actual forwarding channel did not retain child")
	}
	t.Cleanup(func() { _ = channel.Cancel() })
	return channel
}

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
	key := carrier.ClosedCarrierKey{NetworkID: receiver.NetworkID, ProfileDigest: receiver.ProfileDigest, LocalNodeID: receiver.NodeID,
		PeerNodeID: candidate.NodeID, PeerKey: candidate.PublicKey, CarrierProfile: carrier.CarrierProfile(candidate.CarrierProfile)}
	local, peer := net.Pipe()
	defer local.Close()
	defer peer.Close()
	pool, err := carrier.NewClosedCarrierPool(time.Now)
	if err != nil {
		t.Fatal(err)
	}
	opened, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	ownerDone := make(chan error, 1)
	go func() {
		lease, acquireErr := pool.AcquireContext(context.Background(), key, func() error { return nil }, func() (carrier.Carrier, error) {
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
	server := &forwardServer{dependencies: forwardingDependencies(fixture.config, nil), certificate: certificate, pool: pool, sessions: newSessionSet(), clock: time.Now}
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

// TestClosedForwardingOpenSameKeySharesOneActualOuterHello holds the first
// actual outer exchange before ACCEPT. A second caller must borrow a distinct
// pool lease, wait for that one creator, then receive the same session without
// opening another Carrier or writing another outer HELLO.
func TestClosedForwardingOpenSameKeySharesOneActualOuterHello(t *testing.T) {
	for _, profile := range []carrier.CarrierProfile{carrier.ClosedCarrierTCP, carrier.ClosedCarrierQUIC} {
		t.Run(string(profile), func(t *testing.T) {
			testClosedForwardingOpenSameKeyActualCarrier(t, profile)
		})
	}
}

func testClosedForwardingOpenSameKeyActualCarrier(t *testing.T, profile carrier.CarrierProfile) {
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
	fixture.config.Current = func() (state.NodeDuty, error) { return fixture.snapshot, nil }
	fixture.config.CurrentClosedProfile = func() (state.ClosedProfileView, bool) { return fixture.view.Profile, true }

	listener, err := carrier.ListenClosedSharedCarrier(profile, endpoint, peerCertificate, func(key [32]byte) bool { return key == clientKey }, 1)
	if err != nil {
		t.Fatal(err)
	}
	pool, err := carrier.NewClosedCarrierPool(time.Now)
	if err != nil {
		_ = listener.Close()
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	server := &forwardServer{dependencies: forwardingDependencies(fixture.config, nil), certificate: clientCertificate, pool: pool, sessions: newSessionSet(), clock: time.Now}
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
		if accepted.Kind != carrier.ClosedSharedNode || accepted.NodeKey != clientKey {
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
		link *forwardLink
		err  error
	}
	results := make(chan openResult, 2)
	started := 0
	received := 0
	var first, second *forwardLink
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
