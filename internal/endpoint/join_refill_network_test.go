//go:build linux

package endpoint

import (
	"bytes"
	"context"
	"io"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	routecarrier "github.com/dianabuilds/ardents-network/internal/route/carrier"
	"github.com/dianabuilds/ardents-network/internal/route/client"
)

// State/worker identity are declared fixtures. All tokens, journals, actual
// Hosting reservations, pairing, role TLS, client traffic and ACKs are real.
func TestTextRouteJoinRefillsActualTrafficThroughBothCarriers(t *testing.T) {
	for _, carrier := range []routecarrier.CarrierProfile{routecarrier.ClosedCarrierTCP, routecarrier.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			endpoint, publisher, state := startRoleNetwork(t, roleNetworkFixture{carrier: carrier, resolution: true, publisher: true, join: true})
			reader := permissionContextFixture(t, endpoint, fixtureID(211), broker.Connection)
			state.issuePermission(t, reader, [3]uint32{64, 64, 0})
			if _, err := reader.openPrefix(t.Context()); err != nil {
				t.Fatal(err)
			}
			if _, err := publisher.openPrefix(t.Context()); err != nil {
				t.Fatal(err)
			}
			exchangeRouteData(t, reader, publisher, state.view.Nodes[15].NodeID, func(streams []*client.ClosedJoinedStream, owners []*dutyContext) {
				ctx, cancel := context.WithTimeout(t.Context(), 40*time.Second)
				defer cancel()
				var presentations atomic.Int32
				presenters := make([]client.ClosedTokenPresenter, 2)
				for index, owner := range owners {
					presenters[index] = func(hello ardp.Hello, class uint8) ([]byte, error) {
						owner.mu.Lock()
						profile, _, err := owner.permissionProfileLocked()
						stocked := err == nil && owner.tokens.Permission != nil && owner.tokens.Permission.StockCountForDuty(profile.Digest, hello.RecipientNodeID, hello.RecipientDutyGeneration, class) != 0
						owner.mu.Unlock()
						if err != nil {
							return nil, err
						}
						if !stocked {
							if err := owner.issueTokens(ctx, [][32]byte{hello.RecipientNodeID}, class); err != nil {
								return nil, err
							}
						}
						owner.mu.Lock()
						defer owner.mu.Unlock()
						profile, now, err := owner.permissionProfileLocked()
						if err != nil {
							return nil, err
						}
						presentations.Add(1)
						return owner.tokens.TakeTokenLocked(profile, now, hello, class, ctx)
					}
				}
				// Unused JOIN must not consume a fresh token.
				for index, stream := range streams {
					if err := stream.Replenish(ctx, presenters[index]); err != nil {
						t.Fatal(err)
					}
				}
				if presentations.Load() != 0 {
					t.Fatal("unused JOIN requested refill authority")
				}
				block := bytes.Repeat([]byte{71}, 4<<20)
				for round := 0; round < 2; round++ {
					// Both directions account at least 8 MiB per side. Read
					// consumption returns actual credit; no counter is forged.
					for from := 0; from < 2; from++ {
						sent := make(chan error, 1)
						go func() { _, err := streams[from].Write(block); sent <- err }()
						received := make([]byte, len(block))
						_, readErr := io.ReadFull(streams[1-from], received)
						writeErr := <-sent
						if readErr != nil || writeErr != nil || !bytes.Equal(received, block) {
							t.Fatalf("round %d direction %d: read=%v write=%v", round, from, readErr, writeErr)
						}
					}
					before := presentations.Load()
					for index, stream := range streams {
						refill, finish := context.WithTimeout(ctx, 2*time.Second)
						err := stream.Replenish(refill, presenters[index])
						finish()
						if err != nil {
							t.Fatalf("round %d real JOIN refill %d: %v", round, index, err)
						}
					}
					if presentations.Load() != before+2 {
						t.Fatal("genuine threshold did not request exactly one token per JOIN")
					}
					// Preserve the actual enclosing prefixes' independent
					// allowance before the next traffic episode.
					for index, owner := range owners {
						owner.mu.Lock()
						source, responder := owner.source.CurrentLocked(), owner.responder.currentLocked()
						owner.mu.Unlock()
						if source != nil {
							if err := source.Replenish(ctx, presenters[index]); err != nil {
								t.Fatal(err)
							}
						}
						if responder != nil {
							if err := responder.replenish(ctx, presenters[index]); err != nil {
								t.Fatal(err)
							}
						}
					}
				}
			})
		})
	}
}
