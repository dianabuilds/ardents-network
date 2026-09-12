//go:build linux

package node

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestClosedNodeRestrictionRefusesValidPrivateTokenWithoutSpendingIt(t *testing.T) {
	for _, transport := range []route.CarrierProfile{route.ClosedCarrierTCP, route.ClosedCarrierQUIC} {
		t.Run(string(transport), func(t *testing.T) {
			fixture := newClosedBootstrapFixture(t)
			token := closedRestrictionToken(t, fixture)
			certificate, key := rendezvousCertificate(t, 211, "restricted-interior")
			peerCertificate, peerKey := rendezvousCertificate(t, 212, "restricted-entry")
			fixture.snapshot.ProbeEndpoint, fixture.snapshot.CarrierProfile = reserveClosedBootstrapAddress(t, transport), string(transport)
			fixture.snapshot.NodePublicKey = key
			fixture.snapshot.Candidates[0].PublicKey = peerKey
			fixture.config.now = func() time.Time { return fixture.now }
			fixture.config.Current = func() (DutyView, error) { return fixture.snapshot, nil }
			fixture.config.CurrentClosedProfile = func() (state.ClosedProfileView, bool) { return fixture.view.Profile, true }
			fixture.config.ClosedForwarding = ClosedForwardingProfile{Root: filepath.Join(t.TempDir(), "spends"), Certificate: certificate, ConnectionLimit: 2, DrainTimeout: 2 * time.Second}
			if err := os.MkdirAll(fixture.config.ClosedForwarding.Root, 0o700); err != nil {
				t.Fatal(err)
			}
			server, err := startClosedForwarding(fixture.config, fixture.snapshot)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				server.Stop()
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				if err := server.Drain(ctx); err != nil {
					t.Error(err)
				}
			}()
			deadline := time.Now().Add(8 * time.Second)
			outer, err := route.OpenClosedNodeCarrier(t.Context(), route.ClosedNodeCarrierRequest{CarrierProfile: transport, Endpoint: fixture.snapshot.ProbeEndpoint,
				Certificate: peerCertificate, ExpectedPeerKey: key, Deadline: deadline})
			if err != nil {
				t.Fatal(err)
			}
			defer outer.Close()
			if err := outer.SetDeadline(deadline); err != nil {
				t.Fatal(err)
			}
			receiver := fixture.receiver
			hello := route.ClosedHello{NetworkID: receiver.NetworkID, StateGeneration: receiver.StateGeneration, StateDigest: receiver.StateDigest, ProfileDigest: receiver.ProfileDigest,
				RecipientNodeID: receiver.NodeID, RecipientDutyGeneration: receiver.DutyGeneration, Purpose: route.ClosedPurposeForwarding, ChannelNonce: [32]byte{110}, Deadline: fixture.now.Add(9 * time.Second)}
			body, err := route.EncodeClosedHello(hello)
			if err != nil {
				t.Fatal(err)
			}
			if err := route.WriteClosedLaneFrame(outer, route.ClosedLaneFrame{Kind: 1, Body: body}); err != nil {
				t.Fatal(err)
			}
			if accepted, err := route.ReadClosedLaneFrame(outer); err != nil || accepted.Kind != 5 {
				t.Fatalf("outer acceptance: %+v %v", accepted, err)
			}
			for index, restriction := range []route.ClosedChildRestriction{route.ClosedChildIssuerBootstrap, route.ClosedChildOrdinary} {
				lane := uint32(index*2 + 1)
				body, err := route.EncodeClosedNodeOpen(route.ClosedOpen{NextNodeID: receiver.NodeID, NextDutyGeneration: receiver.DutyGeneration,
					Purpose: route.ClosedPurposeForwarding, Deadline: fixture.now.Add(8 * time.Second)}, restriction)
				if err != nil {
					t.Fatal(err)
				}
				if err := route.WriteClosedLaneFrame(outer, route.ClosedLaneFrame{Kind: 4, Lane: lane, Body: body}); err != nil {
					t.Fatal(err)
				}
				inner, err := route.OpenClosedRoleTLS(t.Context(), &outerTestInnerConn{outer: outer, lane: lane}, key, deadline)
				if err != nil {
					t.Fatal(err)
				}
				hello.ChannelNonce = [32]byte{byte(111 + index)}
				hello.Deadline = fixture.now.Add(8 * time.Second)
				body, err = route.EncodeClosedHello(hello)
				if err != nil {
					t.Fatal(err)
				}
				if err := route.WriteClosedLaneFrame(inner, route.ClosedLaneFrame{Kind: 1, Body: body}); err != nil {
					t.Fatal(err)
				}
				if err := route.WriteClosedLaneFrame(inner, route.ClosedLaneFrame{Kind: 2, Body: append([]byte{2}, token...)}); err != nil {
					t.Fatal(err)
				}
				accepted, err := route.ReadClosedLaneFrame(inner)
				if restriction == route.ClosedChildIssuerBootstrap {
					if err == nil {
						t.Fatalf("restricted child accepted valid private token: %+v", accepted)
					}
				} else if err != nil || accepted.Kind != 5 || accepted.Body[0] != 0 {
					t.Fatalf("ordinary child could not spend same valid token: %+v %v", accepted, err)
				}
			}
		})
	}
}
