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
	"github.com/dianabuilds/ardents-network/internal/route"
)

func addTextDataJoinState(source *textSourceStateFixture) {
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
	for _, carrier := range []route.CarrierProfile{route.ClosedCarrierTCP, route.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			endpoint, publisher, source := startTextRoleNetworkWithJoin(t, carrier, true, true, true)
			reader := textPermissionContextFixture(t, endpoint, fixtureID(211), broker.Connection)
			source.issuePermission(t, reader, [3]uint32{64, 64, 0})
			if _, err := reader.openTextPrefix(t.Context()); err != nil {
				t.Fatal(err)
			}
			if _, err := publisher.openTextPrefix(t.Context()); err != nil {
				t.Fatal(err)
			}
			responder, err := publisher.openTextPublisherPrefix(t.Context(), &publisher.responder, 3)
			if err != nil {
				t.Fatal(err)
			}
			receiver := source.view.Nodes[15].NodeID
			for _, owner := range []*textContext{reader, publisher} {
				if err := owner.issueTextTokens(t.Context(), [][32]byte{receiver}, 2); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			intent := route.ClosedJoinIntent{Secret: fixtureID(232), Context: fixtureID(233), SetupDeadline: time.Now().UTC().Add(10 * time.Second).Truncate(time.Second), WorkDeadline: time.Now().UTC().Add(time.Minute).Truncate(time.Second)}
			type opened struct {
				stream *route.ClosedJoinedStream
				err    error
			}
			results := make(chan opened, 2)
			prefixes := []*route.ClosedSourcePrefix{reader.prefix, responder}
			for index, owner := range []*textContext{reader, publisher} {
				go func() {
					stream, err := prefixes[index].Join(ctx, func(hello route.ClosedHello, class uint8) ([]byte, error) {
						owner.mu.Lock()
						defer owner.mu.Unlock()
						profile, now, err := owner.textPermissionProfileLocked()
						if err != nil {
							return nil, err
						}
						if hello.Purpose != route.ClosedPurposeDataJoin || hello.RecipientNodeID != receiver || class != 2 {
							return nil, errors.New("JOIN crossed token purpose")
						}
						return owner.takeTextTokenLocked(profile, now, hello, class, ctx)
					}, intent)
					results <- opened{stream, err}
				}()
			}
			var streams []*route.ClosedJoinedStream
			for range 2 {
				result := <-results
				if result.err != nil {
					cancel()
					for _, stream := range streams {
						_ = stream.Close()
					}
					// The other in-flight Join remains owned by ctx and is joined by its
					// prefix cleanup if this is the first result; drain it before failure.
					if len(streams) == 0 {
						other := <-results
						if other.stream != nil {
							_ = other.stream.Close()
						}
					}
					t.Fatal(result.err)
				}
				streams = append(streams, result.stream)
			}
			t.Cleanup(func() {
				cancel()
				for _, stream := range streams {
					if err := stream.Close(); err != nil {
						t.Error(err)
					}
				}
			})
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
		})
	}
}
