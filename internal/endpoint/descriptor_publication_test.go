//go:build linux

package endpoint

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/endpoint/introduction"
	"github.com/dianabuilds/ardents-network/internal/endpoint/source"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	routecarrier "github.com/dianabuilds/ardents-network/internal/route/carrier"
	servicepublication "github.com/dianabuilds/ardents-network/internal/service/publication"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

// Worker qualification and accepted public State remain explicit fixtures.
// Instance acceptance, registration, publication floor/record, private signer,
// issuer tokens, both prefixes and the receiving Node Store are real owners.
func TestTextPublisherCommitsInstanceSignedDescriptor(t *testing.T) {
	for _, carrier := range []routecarrier.CarrierProfile{routecarrier.ClosedCarrierTCP, routecarrier.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			endpoint, owner, sourceState := startRoleNetwork(t, roleNetworkFixture{carrier: carrier, resolution: true, publisher: true})
			public, authority, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			defer clear(authority)
			now := time.Now().UTC().Truncate(time.Second)
			root, binding := acceptedInstanceBinding(t, serviceInstanceFixtureRoot(t), endpoint.network, authority, now.Add(-time.Second), sourceState.view.Profile.NotAfter)
			t.Cleanup(func() {
				if err := endpoint.Close(); err != nil {
					t.Error(err)
				}
				if err := root.Close(); err != nil {
					t.Error(err)
				}
			})
			publisher, err := servicepublication.Open(servicepublication.Config{Root: networkPrivateRoot(t), NetworkID: endpoint.network, Authority: public, Clock: time.Now})
			if err != nil {
				t.Fatal(err)
			}
			endpoint.publisherBinding, endpoint.publications = binding, publisher
			endpoint.authority = [32]byte(public)
			if _, err := owner.openPrefix(t.Context()); err != nil {
				t.Fatal(err)
			}
			if _, err := owner.openIntroductionPrefix(t.Context()); err != nil {
				t.Fatal(err)
			}
			first, err := owner.registerIntroduction(t.Context(), 1, time.Now().UTC().Add(time.Minute).Truncate(time.Second))
			if err != nil {
				t.Fatal(err)
			}
			published, err := owner.publishDescriptor(t.Context())
			if err != nil {
				t.Fatalf("publish registered Instance: %v", err)
			}
			fields := published.Descriptor.Private
			if fields.Revision != 1 || fields.Slot != introduction.Slot(first) || fields.NodeID != introduction.Node(first) ||
				fields.RecipientKey == [32]byte{} || published.Current.Credential != binding.Credential() {
				t.Fatal("private Descriptor escaped its registered Instance")
			}
			retrieved := lookupPublishedProof(t, owner, published.Descriptor.Target)
			if !bytes.Equal(retrieved, first.CopyDescriptorLocked()) {
				t.Fatal("resolution Node did not retain exact signed Descriptor")
			}
			// Even accidental reuse of trusted internal handles cannot transfer the
			// selected Instance into another independently admitted local context.
			foreign := permissionContextFixture(t, endpoint, fixtureID(211), broker.Administration)
			// A separate holder request meets the non-nil permission preflight
			// without transferring the original owner's authority or secret key.
			if _, _, err := foreign.tokens.Request([3]uint32{1, 0, 0}); err != nil {
				t.Fatal(err)
			}
			foreign.mu.Lock()
			source.TransplantLive(&foreign.source, owner.source.CurrentLocked())
			introduction.TransplantCurrent(&foreign.publication.pair, first)
			foreign.mu.Unlock()
			_, foreignErr := foreign.publishDescriptor(t.Context())
			foreign.mu.Lock()
			source.TransplantLive(&foreign.source, nil)
			introduction.TransplantCurrent(&foreign.publication.pair, nil)
			foreign.mu.Unlock()
			if foreignErr == nil || !strings.Contains(foreignErr.Error(), "another Publisher context owns the publication") || endpoint.publisherOwner != owner {
				t.Fatal("another context stole the Instance publication")
			}
			if err := foreign.Close(); err != nil {
				t.Fatal(err)
			}
			original := append([]byte(nil), first.CopyDescriptorLocked()...)
			if _, err := owner.publishDescriptor(t.Context()); err != nil {
				t.Fatalf("exact publication retry: %v", err)
			}
			if !bytes.Equal(first.CopyDescriptorLocked(), original) {
				t.Fatal("publication retry rotated signed bytes or key")
			}
			if err := owner.withdrawIntroduction(t.Context()); err != nil {
				t.Fatal(err)
			}
			if first.RecipientPublicLocked(time.Now()) != [32]byte{} {
				t.Fatal("withdrawal retained recipient key")
			}
			second, err := owner.registerIntroduction(t.Context(), 2, time.Now().UTC().Add(time.Minute).Truncate(time.Second))
			if err != nil {
				t.Fatal(err)
			}
			refreshed, err := owner.publishDescriptor(t.Context())
			if err != nil {
				t.Fatalf("refresh registered Instance: %v", err)
			}
			if refreshed.Current.Digest != published.Current.Digest || refreshed.Descriptor.Private.Revision != 2 ||
				refreshed.Descriptor.Private.RecipientKey == fields.RecipientKey || refreshed.Descriptor.Private.Slot == fields.Slot {
				t.Fatal("refresh changed publication generation or reused slot/key")
			}
			retrieved = lookupPublishedProof(t, owner, refreshed.Descriptor.Target)
			decoded, err := reachability.VerifyPrivate(retrieved, refreshed.Descriptor.Target, endpoint.network, sourceState.view.Profile.Digest, time.Now().UTC())
			if err != nil || decoded.Descriptor.Private.Revision != 2 {
				t.Fatalf("refreshed Node proof: %v", err)
			}
			if err := owner.Close(); err != nil {
				t.Fatal(err)
			}
			if _, err := publisher.Acquire(t.Context()); err == nil {
				t.Fatal("context shutdown retained live publication")
			}
			if binding.Public() != nil {
				t.Fatal("context shutdown retained Instance signer")
			}
			if second.RecipientPublicLocked(time.Now()) != [32]byte{} {
				t.Fatal("context shutdown retained recipient key")
			}
		})
	}
}

func lookupPublishedProof(t *testing.T, owner *dutyContext, target [32]byte) []byte {
	t.Helper()
	owner.mu.Lock()
	acquisition := owner.source.AcquireResolutionLocked()
	owner.mu.Unlock()
	if acquisition == nil {
		t.Fatal("text Source resolution acquisition unavailable")
	}
	defer acquisition.Release()
	receiver, err := acquisition.ResolutionRecipient()
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.issueTokens(t.Context(), [][32]byte{receiver}, 1); err != nil {
		t.Fatal(err)
	}
	status, raw, err := acquisition.ExchangeDescriptor(t.Context(), func(hello ardp.Hello, class uint8) ([]byte, error) {
		owner.mu.Lock()
		defer owner.mu.Unlock()
		profile, now, err := owner.permissionProfileLocked()
		if err != nil {
			return nil, err
		}
		return owner.tokens.TakeTokenLocked(profile, now, hello, class, t.Context())
	}, target, nil)
	if err != nil || status != 0 {
		t.Fatalf("read actual Node proof: %d %v", status, err)
	}
	return raw
}
