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
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
)

// This is the caller boundary: TCP/TLS and QUIC both cross the real shared
// listener, outer HELLO/ACCEPT exchange, and child OPEN grammar. The State
// projection only supplies the selected peer; it does not replace transport.
func TestClosedForwardingOpenActualCarrierOutcomes(t *testing.T) {
	for _, profile := range []route.CarrierProfile{route.ClosedCarrierTCP, route.ClosedCarrierQUIC} {
		for _, outcome := range []string{"success", "refusal", "cancellation"} {
			t.Run(string(profile)+"/"+outcome, func(t *testing.T) {
				testClosedForwardingOpenActualCarrierOutcome(t, profile, outcome)
			})
		}
	}
}

func testClosedForwardingOpenActualCarrierOutcome(t *testing.T, profile route.CarrierProfile, outcome string) {
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
		if accepted.Kind != route.ClosedSharedNode || accepted.NodeKey != clientKey {
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
		link *closedForwardingLink
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

func closedForwardingActualCarrierEndpoint(t *testing.T, profile route.CarrierProfile) string {
	t.Helper()
	if profile == route.ClosedCarrierTCP {
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
