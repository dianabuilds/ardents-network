//go:build linux

package endpoint

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	routecarrier "github.com/dianabuilds/ardents-network/internal/route/carrier"
	"github.com/dianabuilds/ardents-network/internal/route/client"
)

func addDataJoinState(source *sourceStateFixture) {
	source.view.NodeCount, source.snapshot.CandidateCount = 16, 16
	source.view.Nodes[15] = state.ClosedRouteNodeView{NodeID: fixtureID(202), RecordDigest: fixtureID(203), DutyGeneration: 16, RoleDomain: 2, Subrole: 4}
	candidate := source.snapshot.Candidates[4]
	candidate.NodeID, candidate.RecordDigest = source.view.Nodes[15].NodeID, source.view.Nodes[15].RecordDigest
	source.snapshot.Candidates[15] = candidate
}

// State/worker identity remain explicit fixtures. This exercises real Endpoint
// issuance and token journal plus Source and Responder prefixes, JOIN client,
// sixteen Node runtimes and both selected Carriers. No Service TLS claim here.
func TestTextRouteJoinConnectsSourceAndResponder(t *testing.T) {
	for _, carrier := range []routecarrier.CarrierProfile{routecarrier.ClosedCarrierTCP, routecarrier.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			endpoint, publisher, source := startRoleNetwork(t, roleNetworkFixture{carrier: carrier, resolution: true, publisher: true, join: true})
			reader := permissionContextFixture(t, endpoint, fixtureID(211), broker.Connection)
			source.issuePermission(t, reader, [3]uint32{64, 64, 0})
			if _, err := reader.openPrefix(t.Context()); err != nil {
				t.Fatal(err)
			}
			if _, err := publisher.openPrefix(t.Context()); err != nil {
				t.Fatal(err)
			}
			exchangeRouteData(t, reader, publisher, source.view.Nodes[15].NodeID, nil)
		})
	}
}

// exchangeRouteData verifies a real two-way DataJoin exchange. The optional
// ready callback observes both joined streams before either direction writes.
func exchangeRouteData(t *testing.T, reader, publisher *dutyContext, receiver [32]byte, ready func([]*client.ClosedJoinedStream, []*dutyContext)) {
	t.Helper()
	if _, err := publisher.openResponderPrefix(t.Context()); err != nil {
		t.Fatal(err)
	}
	for _, owner := range []*dutyContext{reader, publisher} {
		if err := owner.issueTokens(t.Context(), [][32]byte{receiver}, 2); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	intent := client.ClosedJoinIntent{Secret: fixtureID(232), Context: fixtureID(233), SetupDeadline: time.Now().UTC().Add(10 * time.Second).Truncate(time.Second), WorkDeadline: time.Now().UTC().Add(time.Minute).Truncate(time.Second)}
	type opened struct {
		stream *client.ClosedJoinedStream
		err    error
		owner  *dutyContext
	}
	results := make(chan opened, 2)
	reader.mu.Lock()
	readerJoin := reader.source.AcquireJoinLocked()
	reader.mu.Unlock()
	if readerJoin == nil {
		t.Fatal("text Source JOIN acquisition unavailable")
	}
	defer readerJoin.Release()
	publisher.mu.Lock()
	responderJoin := publisher.responder.acquireJoinLocked(publisher.source.CurrentLocked())
	publisher.mu.Unlock()
	if responderJoin == nil {
		t.Fatal("text Responder JOIN acquisition unavailable")
	}
	defer responderJoin.Release()
	prefixes := []joinPrefix{sourceJoinAcquisition{readerJoin}, responderJoin}
	for index, owner := range []*dutyContext{reader, publisher} {
		go func() {
			stream, err := prefixes[index].Join(ctx, func(hello ardp.Hello, class uint8) ([]byte, error) {
				owner.mu.Lock()
				defer owner.mu.Unlock()
				profile, now, err := owner.permissionProfileLocked()
				if err != nil {
					return nil, err
				}
				if hello.Purpose != ardp.PurposeDataJoin || hello.RecipientNodeID != receiver || class != 2 {
					return nil, errors.New("JOIN crossed token purpose")
				}
				return owner.tokens.TakeTokenLocked(profile, now, hello, class, ctx)
			}, intent)
			results <- opened{stream, err, owner}
		}()
	}
	var streams []*client.ClosedJoinedStream
	var streamOwners []*dutyContext
	for range 2 {
		result := <-results
		if result.err != nil {
			cancel()
			for _, stream := range streams {
				_ = stream.Close()
			}
			if len(streams) == 0 {
				other := <-results
				if other.stream != nil {
					_ = other.stream.Close()
				}
			}
			t.Fatal(result.err)
		}
		streams = append(streams, result.stream)
		streamOwners = append(streamOwners, result.owner)
	}
	t.Cleanup(func() {
		cancel()
		for _, stream := range streams {
			if err := stream.Close(); err != nil {
				t.Error(err)
			}
		}
	})
	if ready != nil {
		ready(streams, streamOwners)
	}
	payload := bytes.Repeat([]byte("framed opaque bytes "), 8192)
	sent := make(chan error, 1)
	go func() {
		_, err := streams[0].Write(payload)
		if err == nil {
			err = streams[0].CloseWrite()
		}
		sent <- err
	}()
	received, err := io.ReadAll(streams[1])
	writeErr := <-sent
	if err != nil || writeErr != nil || !bytes.Equal(received, payload) {
		t.Fatalf("joined payload or EOF: %v", errors.Join(err, writeErr))
	}
	reverse := []byte("reverse after EOF")
	go func() {
		_, err := streams[1].Write(reverse)
		if err == nil {
			err = streams[1].CloseWrite()
		}
		sent <- err
	}()
	received, err = io.ReadAll(streams[0])
	writeErr = <-sent
	if err != nil || writeErr != nil || !bytes.Equal(received, reverse) {
		t.Fatalf("reverse after EOF: %v", errors.Join(err, writeErr))
	}
}
