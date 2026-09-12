//go:build linux

package endpoint

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/service/publication"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

// Worker qualification and accepted public State remain explicit fixtures.
// Instance acceptance, registration, publication floor/record, private signer,
// issuer tokens, both prefixes and the receiving Node Store are real owners.
func TestTextPublisherCommitsInstanceSignedDescriptor(t *testing.T) {
	for _, carrier := range []route.CarrierProfile{route.ClosedCarrierTCP, route.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			endpoint, owner, source := startTextRoleNetwork(t, carrier, true, true)
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
			publisher, err := publication.Open(publication.Config{Root: textNetworkPrivateRoot(t), NetworkID: endpoint.network, Authority: public, Clock: time.Now})
			if err != nil {
				t.Fatal(err)
			}
			endpoint.publisherBinding, endpoint.publications = binding, publisher
			endpoint.authority = [32]byte(public)
			if _, err := owner.openTextPrefix(t.Context()); err != nil {
				t.Fatal(err)
			}
			if _, err := owner.openTextIntroductionPrefix(t.Context()); err != nil {
				t.Fatal(err)
			}
			first, err := owner.registerTextIntroduction(t.Context(), 1, time.Now().UTC().Add(time.Minute).Truncate(time.Second))
			if err != nil {
				t.Fatal(err)
			}
			published, err := owner.publishTextDescriptor(t.Context())
			if err != nil {
				t.Fatalf("publish registered Instance: %v", err)
			}
			fields := published.Descriptor.Private
			if fields.Revision != 1 || fields.Slot != first.request.Slot || fields.NodeID != first.node ||
				fields.RecipientKey == [32]byte{} || fields.RecipientKey == binding.IntroductionPublic() || published.Current.Credential != binding.Credential() {
				t.Fatal("private Descriptor escaped its registered Instance or reused legacy recipient")
			}
			retrieved := lookupTextPublishedProof(t, owner, published.Descriptor.Target)
			if !bytes.Equal(retrieved, first.descriptor) {
				t.Fatal("resolution Node did not retain exact signed Descriptor")
			}
			// Even accidental reuse of trusted internal handles cannot transfer the
			// selected Instance into another independently admitted local context.
			foreign := textPermissionContextFixture(t, endpoint, fixtureID(211), broker.Administration)
			foreign.mu.Lock()
			foreign.prefix, foreign.registration, foreign.permission = owner.prefix, first, owner.permission
			foreign.mu.Unlock()
			_, foreignErr := foreign.publishTextDescriptor(t.Context())
			foreign.mu.Lock()
			foreign.prefix, foreign.registration, foreign.permission = nil, nil, nil
			foreign.mu.Unlock()
			if foreignErr == nil || endpoint.textPublisherOwner != owner {
				t.Fatal("another context stole the Instance publication")
			}
			if err := foreign.Close(); err != nil {
				t.Fatal(err)
			}
			original := append([]byte(nil), first.descriptor...)
			if _, err := owner.publishTextDescriptor(t.Context()); err != nil {
				t.Fatalf("exact publication retry: %v", err)
			}
			if !bytes.Equal(first.descriptor, original) {
				t.Fatal("publication retry rotated signed bytes or key")
			}
			if err := owner.withdrawTextIntroduction(t.Context()); err != nil {
				t.Fatal(err)
			}
			if first.recipient.Public(time.Now()) != [32]byte{} {
				t.Fatal("withdrawal retained recipient key")
			}
			second, err := owner.registerTextIntroduction(t.Context(), 2, time.Now().UTC().Add(time.Minute).Truncate(time.Second))
			if err != nil {
				t.Fatal(err)
			}
			refreshed, err := owner.publishTextDescriptor(t.Context())
			if err != nil {
				t.Fatalf("refresh registered Instance: %v", err)
			}
			if refreshed.Current.Digest != published.Current.Digest || refreshed.Descriptor.Private.Revision != 2 ||
				refreshed.Descriptor.Private.RecipientKey == fields.RecipientKey || refreshed.Descriptor.Private.Slot == fields.Slot {
				t.Fatal("refresh changed publication generation or reused slot/key")
			}
			retrieved = lookupTextPublishedProof(t, owner, refreshed.Descriptor.Target)
			decoded, err := reachability.VerifyPrivate(retrieved, refreshed.Descriptor.Target, endpoint.network, source.view.Profile.Digest, time.Now().UTC())
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
			if second.recipient.Public(time.Now()) != [32]byte{} {
				t.Fatal("context shutdown retained recipient key")
			}
		})
	}
}

func lookupTextPublishedProof(t *testing.T, owner *textContext, target [32]byte) []byte {
	t.Helper()
	prefix := owner.prefix
	receiver, err := prefix.ResolutionRecipient()
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.issueTextTokens(t.Context(), [][32]byte{receiver}, 1); err != nil {
		t.Fatal(err)
	}
	status, raw, err := prefix.ExchangeDescriptor(t.Context(), func(hello route.ClosedHello, class uint8) ([]byte, error) {
		owner.mu.Lock()
		defer owner.mu.Unlock()
		profile, now, err := owner.textPermissionProfileLocked()
		if err != nil {
			return nil, err
		}
		return owner.takeTextTokenLocked(profile, now, hello, class, t.Context())
	}, target, nil)
	if err != nil || status != 0 {
		t.Fatalf("read actual Node proof: %d %v", status, err)
	}
	return raw
}
