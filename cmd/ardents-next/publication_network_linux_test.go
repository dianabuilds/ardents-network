//go:build linux

package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// The receiving side retains genuine Network history independently of the
// Publisher. Only signed public inputs and detached initial facts are shared;
// Publisher conflict cannot mutate this root, its lease or its currentness.
// Signing and clock infrastructure still have one fixture controller.
func publicationReceivingNetwork(t *testing.T, publisher *networkAdmissionFixture) *networkAdmissionFixture {
	t.Helper()
	original, err := publisher.current()
	if err != nil {
		t.Fatal(err)
	}
	public := publisher.spec.Authority.Public().(ed25519.PublicKey)
	config := state.Config{Root: filepath.Join(t.TempDir(), "receiving-network"), NetworkID: publisher.profile.NetworkID,
		Authorities: map[[32]byte]ed25519.PublicKey{sha256.Sum256(public): public}, Threshold: 1,
		ClosedProfileAuthority: public, AcceptedProfile: epoch.ProfileClosedRoute, Clock: time.Now, ObserveClock: time.Now}
	owner, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := owner.Close(); err != nil {
			t.Error(err)
		}
	})
	bundle, err := networkfixture.BuildClosed(publisher.spec)
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Epoch.Digest != publisher.digest {
		t.Fatal("receiving signed Epoch differs from original Publisher")
	}
	if _, err := owner.Accept(t.Context(), bundle.Epoch.Raw, bundle.Epoch.Inputs, bundle.Epoch.Materials); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.AcceptClosedProfile(bundle.Profile); err != nil {
		t.Fatal(err)
	}
	current, err := owner.CurrentRuntime()
	if err != nil || current.Profile().ProfileBinding != original.Profile().ProfileBinding {
		t.Fatal("receiving authority differs from original Publisher", err)
	}
	return &networkAdmissionFixture{current: owner.CurrentRuntime, authority: networkAdmissionAuthority(owner.CurrentRuntime, owner.Close), profile: publisher.profile}
}

func TestPublicationReceivingNetworkSurvivesPublisherConflict(t *testing.T) {
	for _, carrier := range []transport.CarrierProfile{transport.ClosedCarrierTCP, transport.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			publisher, _, _ := publicationRouteNetwork(t, carrier)
			receiving := publicationReceivingNetwork(t, publisher)
			before, err := receiving.current()
			if err != nil {
				t.Fatal(err)
			}
			if err := publisher.profileConflict(t); err == nil || err.Error() != "closed profile has a durable conflict" {
				t.Fatal("Publisher signed conflict did not commit", err)
			}
			if _, err := publisher.current(); err == nil || err.Error() != "closed profile is unavailable" {
				t.Fatal("Publisher retained authority after conflict", err)
			}
			after, err := receiving.current()
			if err != nil || after.Profile().ProfileBinding != before.Profile().ProfileBinding {
				t.Fatal("Publisher conflict changed independent receiving authority", err)
			}
			for _, node := range publisher.spec.Nodes {
				if _, err := after.Member(node.NodeID, after.ObservedAt()); err != nil {
					t.Fatal("independent receiving assignment became unavailable", err)
				}
			}
		})
	}
}
