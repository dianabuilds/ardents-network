//go:build linux

package endpoint

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/service/publication"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

func addTextResolutionState(source *textSourceStateFixture) {
	source.view.NodeCount, source.snapshot.CandidateCount = 7, 7
	for index := 5; index < 7; index++ {
		domain, subrole := uint8(2), uint8(5)
		if index == 6 {
			domain, subrole = 4, 3
		}
		id, record := fixtureID(byte(10+index)), fixtureID(byte(30+index))
		source.view.Nodes[index] = state.ClosedRouteNodeView{NodeID: id, RecordDigest: record, RoleDomain: domain, Subrole: subrole, DutyGeneration: uint64(index + 1)}
		candidate := source.snapshot.Candidates[4]
		candidate.NodeID, candidate.RecordDigest = id, record
		source.snapshot.Candidates[index] = candidate
	}
}

// This joins actual issuer, Endpoint stock/journal, retained forwarding, fresh
// terminal TLS and the Node Store. Publication/Introduction facts are explicit
// fixtures: this test does not claim registered Publisher or command readiness.
func TestTextResolutionUsesIssuedControlThroughRetainedPrefix(t *testing.T) {
	for _, carrier := range []route.CarrierProfile{route.ClosedCarrierTCP, route.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			_, owner, source := startTextControlNetwork(t, carrier, true)
			prefix, err := owner.openTextPrefix(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			target, raw := textResolutionProof(t, source)
			receiver, err := prefix.ResolutionRecipient()
			if err != nil || receiver != source.view.Nodes[5].NodeID {
				t.Fatalf("State resolution: %x %v", receiver, err)
			}
			if err := owner.issueTextTokens(t.Context(), [][32]byte{receiver}, 1); err != nil {
				t.Fatal(err)
			}
			// The explicit Publisher fixture sends a genuine signed proof using
			// actual issued stock; no success callback pre-populates the Store.
			status, _, err := prefix.ExchangeDescriptor(t.Context(), func(hello route.ClosedHello, class uint8) ([]byte, error) {
				owner.mu.Lock()
				defer owner.mu.Unlock()
				profile, now, err := owner.textPermissionProfileLocked()
				if err != nil {
					return nil, err
				}
				return owner.takeTextTokenLocked(profile, now, hello, class, t.Context())
			}, [32]byte{}, raw)
			if err != nil || status != 0 {
				t.Fatalf("real publication: %d %v", status, err)
			}
			verified, err := owner.lookupTextDescriptor(t.Context(), target)
			if err != nil || verified.Descriptor.Target != target {
				t.Fatalf("actual Endpoint resolution: %v", err)
			}
			if !bytes.Equal(verified.Current.Record, verified.Descriptor.Publication) || verified.Descriptor.Private.Slot != fixtureID(182) {
				t.Fatal("resolved proof changed")
			}
			if err := owner.issueTextTokens(t.Context(), [][32]byte{receiver}, 1); err != nil {
				t.Fatal(err)
			}
			owner.mu.Lock()
			reserved := owner.permission.reserved
			owner.mu.Unlock()
			if _, err := owner.lookupTextDescriptor(t.Context(), target); err != nil {
				t.Fatalf("existing resolution token stock: %v", err)
			}
			owner.mu.Lock()
			sameReservation := owner.permission.reserved == reserved
			owner.mu.Unlock()
			if !sameReservation {
				t.Fatal("resolution ignored existing token and consumed more allocation")
			}
			if _, err := owner.lookupTextDescriptor(t.Context(), fixtureID(199)); err == nil {
				t.Fatal("absent Target was accepted")
			}
			owner.mu.Lock()
			retained := owner.prefix == prefix && owner.resolution == nil && owner.issuance == nil && owner.permission.batches == 2
			owner.mu.Unlock()
			if !retained {
				t.Fatal("ordinary resolution replaced its prefix or minted more bootstrap allowance")
			}
		})
	}
}

func textResolutionProof(t *testing.T, source *textSourceStateFixture) ([32]byte, []byte) {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	profile := source.view.Profile
	public, authority, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(authority)
	instancePublic, signer, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(signer)
	var instance [32]byte
	copy(instance[:], instancePublic)
	credential, err := (publication.Credential{InstancePublic: instance, IntroductionHPKEPublic: fixtureID(181), Generation: 1,
		NotBefore: now.Add(-time.Second).Unix(), NotAfter: profile.NotAfter.Unix(), NetworkID: profile.NetworkID, Capabilities: 3}).Issue(authority)
	if err != nil {
		t.Fatal(err)
	}
	publisher, err := publication.Open(publication.Config{Root: textNetworkPrivateRoot(t), NetworkID: profile.NetworkID, Authority: public, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := publisher.Close(); err != nil {
			t.Error(err)
		}
	}()
	current, err := publisher.Publish(t.Context(), publication.PublishInput{Credential: credential, InstanceSigner: signer, Acknowledgement: []byte("explicit Publication fixture"), At: now})
	if err != nil {
		t.Fatal(err)
	}
	until := now.Add(30 * time.Second)
	if profile.NotAfter.Before(until) {
		until = profile.NotAfter
	}
	raw, _, err := reachability.IssuePrivate(reachability.PrivateIssueInput{Current: current, ProfileDigest: profile.Digest, InstanceSigner: signer,
		Introduction: reachability.PrivateIntroduction{Revision: 1, NodeID: source.view.Nodes[6].NodeID, Slot: fixtureID(182), RecipientKey: fixtureID(183), NotBefore: now, NotAfter: until}})
	if err != nil {
		t.Fatal(err)
	}
	return current.Credential.Target, raw
}
