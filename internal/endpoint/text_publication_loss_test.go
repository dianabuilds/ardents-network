//go:build linux

package endpoint

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/service/publication"
)

// The same real registered Publisher setup feeds successful and interrupted
// Descriptor handovers. Only accepted State and worker qualification are fixtures.
func startTextRegisteredPublisherNetwork(t *testing.T, carrier route.CarrierProfile, gate *textDescriptorACKGate) (*endpoint, *textContext, *textSourceStateFixture, *textIntroductionRegistration) {
	t.Helper()
	endpoint, owner, source := startTextRoleNetwork(t, carrier, true, true, gate.configure(t))
	source.mu.Lock()
	source.view.NodeCount, source.snapshot.CandidateCount = 16, 16
	source.view.Nodes[15] = state.ClosedRouteNodeView{NodeID: fixtureID(202), RecordDigest: fixtureID(203), DutyGeneration: 16, RoleDomain: 2, Subrole: 4}
	candidate := source.snapshot.Candidates[4]
	candidate.NodeID, candidate.RecordDigest, candidate.FamilyID, candidate.PublicKey = fixtureID(202), fixtureID(203), fixtureID(204), fixtureID(205)
	source.snapshot.Candidates[15] = candidate
	source.mu.Unlock()
	public, authority, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(authority)
	now := time.Now().UTC().Truncate(time.Second)
	root, binding := acceptedInstanceBinding(t, serviceInstanceFixtureRoot(t), endpoint.network, authority, now.Add(-time.Second), source.view.Profile.NotAfter)
	t.Cleanup(func() {
		if err := endpoint.Close(); err != nil {
			t.Error(err)
		}
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	})
	publications, err := publication.Open(publication.Config{Root: textNetworkPrivateRoot(t), NetworkID: endpoint.network, Authority: public, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	endpoint.publisherBinding, endpoint.publications, endpoint.authority = binding, publications, [32]byte(public)
	if _, err := owner.openTextPrefix(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.openTextIntroductionPrefix(t.Context()); err != nil {
		t.Fatal(err)
	}
	first, err := owner.registerTextIntroduction(t.Context(), 1, time.Now().UTC().Add(120*time.Second).Truncate(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	return endpoint, owner, source, first
}

// Every failure occurs after the real Store commits the replacement, before
// its network acknowledgement. None may revive accepting readiness or keys.
func TestTextPublicationLossBeforeAcknowledgementRetiresRecipients(t *testing.T) {
	for _, carrier := range []route.CarrierProfile{route.ClosedCarrierTCP, route.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			for _, failure := range []string{"replacement channel", "predecessor channel", "context revoke"} {
				t.Run(failure, func(t *testing.T) {
					gate := newTextDescriptorACKGate()
					defer gate.open()
					endpoint, owner, _, first := startTextRegisteredPublisherNetwork(t, carrier, gate)
					if _, err := owner.publishTextDescriptor(t.Context()); err != nil {
						t.Fatal(err)
					}
					gate.arm(t)
					owner.mu.Lock()
					refresh := owner.refresh
					first.refreshAt = time.Now().Add(-time.Second)
					owner.signalTextRegistrationsLocked()
					owner.mu.Unlock()
					select {
					case <-gate.held:
					case <-refresh.done:
						owner.mu.Lock()
						cause := refresh.err
						owner.mu.Unlock()
						t.Fatalf("refresh ended before replacement Store commit: %v", cause)
					case <-time.After(10 * time.Second):
						owner.mu.Lock()
						cause, registered := refresh.err, owner.registration != nil && owner.registration != first
						owner.mu.Unlock()
						t.Fatalf("replacement did not reach Store commit before ACK: registered=%t refresh=%v", registered, cause)
					}
					owner.mu.Lock()
					second := owner.registration
					ready := first.published && second != nil && second != first && !second.published && second.recipient != nil
					owner.mu.Unlock()
					if !ready || first.recipient.Public(time.Now()) == [32]byte{} || second.recipient.Public(time.Now()) == [32]byte{} {
						t.Fatal("failure was not injected between live registration and acknowledged replacement")
					}
					switch failure {
					case "replacement channel":
						if err := second.close(); err != nil {
							t.Fatal(err)
						}
					case "predecessor channel":
						if err := first.close(); err != nil {
							t.Fatal(err)
						}
					case "context revoke":
						owner.lease.Release()
					}
					gate.open()
					select {
					case <-refresh.done:
					case <-time.After(10 * time.Second):
						t.Fatal("failed publication did not join refresh")
					}
					if failure == "context revoke" {
						select {
						case <-owner.done:
						case <-time.After(10 * time.Second):
							t.Fatal("revoked context did not finish cleanup")
						}
					}
					owner.mu.Lock()
					retired := owner.registration == nil && owner.previousRegistration == nil && !second.published
					cause := refresh.err
					owner.mu.Unlock()
					if !retired || failure != "context revoke" && cause == nil {
						t.Fatalf("late ACK retained readiness or lost failure: retired=%t cause=%v", retired, cause)
					}
					if first.recipient.Public(time.Now()) != [32]byte{} || second.recipient.Public(time.Now()) != [32]byte{} {
						t.Fatal("failed replacement retained recipient key material")
					}
					if _, err := owner.publishTextDescriptor(t.Context()); err == nil {
						t.Fatal("failed replacement resurrected through exact publication retry")
					}
					if failure == "context revoke" {
						endpoint.publisherMu.Lock()
						released := endpoint.publisherBinding == nil && !endpoint.textPublicationLive
						endpoint.publisherMu.Unlock()
						if !released {
							t.Fatal("revocation retained Instance publication authority")
						}
					}
				})
			}
		})
	}
}
