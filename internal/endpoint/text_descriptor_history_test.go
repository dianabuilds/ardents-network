//go:build linux

package endpoint

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/endpoint/descriptorhistory"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/service/publication"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

func textFloorPublication(t *testing.T, authority ed25519.PrivateKey, network [32]byte, generation uint64, from, until time.Time) (publication.Current, ed25519.PrivateKey) {
	t.Helper()
	public, signer, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { clear(signer) })
	var instance [32]byte
	copy(instance[:], public)
	credential, err := (publication.Credential{InstancePublic: instance, IntroductionHPKEPublic: fixtureID(181),
		Generation: generation, NotBefore: from.Unix(), NotAfter: until.Unix(), NetworkID: network, Capabilities: 3}).Issue(authority)
	if err != nil {
		t.Fatal(err)
	}
	root, err := publication.Open(publication.Config{Root: textNetworkPrivateRoot(t), NetworkID: network, Authority: authority.Public().(ed25519.PublicKey)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	})
	current, err := root.Publish(t.Context(), publication.PublishInput{Credential: credential, InstanceSigner: signer, Acknowledgement: []byte("explicit floor test publication"), At: from})
	if err != nil {
		t.Fatal(err)
	}
	return current, signer
}

func textFloorDescriptor(t *testing.T, current publication.Current, signer ed25519.PrivateKey, profile, node [32]byte, revision uint64, slot byte, from, until time.Time) []byte {
	t.Helper()
	raw, _, err := reachability.IssuePrivate(reachability.PrivateIssueInput{Current: current, InstanceSigner: signer, ProfileDigest: profile,
		Introduction: reachability.PrivateIntroduction{Revision: revision, NodeID: node, Slot: fixtureID(slot), RecipientKey: fixtureID(slot + 1), NotBefore: from, NotAfter: until}})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestTextDescriptorFloorBelongsToContextAcrossWorkerLoss(t *testing.T) {
	endpoint, principal := textContextEndpoint(t)
	owner := admittedTextContext(t, endpoint, principal, broker.Connection)
	other := admittedTextContext(t, endpoint, principal, broker.Connection)
	job, err := owner.beginJob(endpoint, broker.Connection)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	network, profile, node := fixtureID(1), fixtureID(2), fixtureID(3)
	_, authority, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(authority)
	current, signer := textFloorPublication(t, authority, network, 1, now, now.Add(10*time.Minute))
	target := current.Credential.Target
	raw := textFloorDescriptor(t, current, signer, profile, node, 2, 20, now, now.Add(100*time.Second))
	if _, err := owner.descriptorHistory.Accept(raw, target, network, profile, now); err != nil {
		t.Fatal(err)
	}
	conflicting := textFloorDescriptor(t, current, signer, profile, node, 2, 30, now, now.Add(100*time.Second))
	if _, err := owner.descriptorHistory.Accept(conflicting, target, network, profile, now); err == nil {
		t.Fatal("same revision conflict accepted")
	}
	owner.retireJob(job)
	if err := owner.finishJobCleanup(job, nil); err != nil {
		t.Fatal(err)
	}
	if !owner.descriptorHistory.Has(target) || owner.descriptorHistory.Matches(target, current.Digest, 2) {
		t.Fatal("worker loss erased context floor")
	}
	if other.descriptorHistory.Has(target) {
		t.Fatal("context history shared with another authorization")
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if !owner.descriptorHistory.Cleared() || owner.descriptorHistory.Has(target) {
		t.Fatal("retired context retained private history")
	}
}

// The existing real Store has revision 1. The Endpoint separately verifies a
// signed revision 2 as a prior observation; the next genuine network response
// must be refused locally even though that Node still considers revision 1 live.
func TestTextResolutionNetworkCannotRollBackLocalDescriptorFloor(t *testing.T) {
	for _, carrier := range []route.CarrierProfile{route.ClosedCarrierTCP, route.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			_, owner, source := startTextRoleNetwork(t, textRoleNetworkFixture{carrier: carrier, resolution: true})
			prefix, err := owner.openTextPrefix(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			now := time.Now().UTC().Truncate(time.Second)
			profile := source.view.Profile
			_, authority, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			defer clear(authority)
			current, signer := textFloorPublication(t, authority, profile.NetworkID, 1, now, now.Add(10*time.Minute))
			first := textFloorDescriptor(t, current, signer, profile.Digest, source.view.Nodes[6].NodeID, 1, 10, now, now.Add(120*time.Second))
			second := textFloorDescriptor(t, current, signer, profile.Digest, source.view.Nodes[6].NodeID, 2, 20, now, now.Add(100*time.Second))
			receiver, err := prefix.ResolutionRecipient()
			if err != nil {
				t.Fatal(err)
			}
			if err := owner.issueTextTokens(t.Context(), [][32]byte{receiver}, 1); err != nil {
				t.Fatal(err)
			}
			status, _, err := prefix.ExchangeDescriptor(t.Context(), func(hello ardp.Hello, class uint8) ([]byte, error) {
				owner.mu.Lock()
				defer owner.mu.Unlock()
				profile, at, err := owner.textPermissionProfileLocked()
				if err != nil {
					return nil, err
				}
				return owner.takeTextTokenLocked(profile, at, hello, class, t.Context())
			}, [32]byte{}, first)
			if err != nil || status != 0 {
				t.Fatalf("actual fixture publication: %d %v", status, err)
			}
			got, err := owner.lookupTextDescriptor(t.Context(), current.Credential.Target)
			if err != nil || !bytes.Equal(got.Current.Record, current.Record) {
				t.Fatalf("actual first lookup: %v", err)
			}
			owner.mu.Lock()
			floorRetained := owner.descriptorHistory.Matches(current.Credential.Target, current.Digest, 1)
			_, err = owner.descriptorHistory.Accept(second, current.Credential.Target, profile.NetworkID, profile.Digest, now)
			owner.mu.Unlock()
			if !floorRetained || err != nil {
				t.Fatalf("lookup failed to retain floor or prior observation invalid: %v", err)
			}
			if _, err := owner.lookupTextDescriptor(t.Context(), current.Credential.Target); err == nil {
				t.Fatal("actual resolver response rolled back locally retained revision")
			}
			owner.mu.Lock()
			retained := owner.descriptorHistory.Matches(current.Credential.Target, current.Digest, 2)
			healthy := owner.currentTextSourceLocked() == prefix && owner.resolution == nil && !owner.closed
			owner.mu.Unlock()
			if !retained || !healthy {
				t.Fatal("ordinary stale response erased floor or damaged context")
			}
			// Fill the real context history through independently signed
			// Descriptors. The admitted issuer and stock remain in use.
			type observedDescriptor struct {
				target [32]byte
				raw    []byte
			}
			observed := make([]observedDescriptor, 0, descriptorhistory.MaximumTargets-1)
			for index := 1; index < descriptorhistory.MaximumTargets; index++ {
				_, nextAuthority, err := ed25519.GenerateKey(rand.Reader)
				if err != nil {
					t.Fatal(err)
				}
				next, nextSigner := textFloorPublication(t, nextAuthority, profile.NetworkID, 1, now, now.Add(10*time.Minute))
				clear(nextAuthority)
				nextRaw := textFloorDescriptor(t, next, nextSigner, profile.Digest, source.view.Nodes[6].NodeID, 1, byte(index), now, now.Add(100*time.Second))
				observed = append(observed, observedDescriptor{target: next.Credential.Target, raw: nextRaw})
			}
			owner.mu.Lock()
			for index, next := range observed {
				if _, err := owner.descriptorHistory.Accept(next.raw, next.target, profile.NetworkID, profile.Digest, now); err != nil {
					owner.mu.Unlock()
					t.Fatalf("capacity setup Descriptor %d: %v", index, err)
				}
			}
			_, err = owner.descriptorHistory.Accept(second, current.Credential.Target, profile.NetworkID, profile.Digest, now)
			reserved, batches := owner.permission.reserved, owner.permission.batches
			tokensBefore := 0
			for _, stock := range owner.permission.stock {
				tokensBefore += len(stock.tokens)
			}
			owner.mu.Unlock()
			if err != nil {
				t.Fatalf("full cache rejected existing Target: %v", err)
			}
			if _, err := owner.lookupTextDescriptor(t.Context(), fixtureID(199)); err == nil {
				t.Fatal("full context accepted another Target")
			}
			owner.mu.Lock()
			tokensAfter := 0
			for _, stock := range owner.permission.stock {
				tokensAfter += len(stock.tokens)
			}
			unchanged := !owner.descriptorHistory.CanAdmit(fixtureID(199)) &&
				owner.descriptorHistory.Matches(current.Credential.Target, current.Digest, 2) && owner.permission.reserved == reserved &&
				owner.permission.batches == batches && tokensBefore == tokensAfter && owner.resolution == nil && owner.issuance == nil
			owner.mu.Unlock()
			if !unchanged {
				t.Fatal("capacity refusal evicted floors or consumed network issuance/admission")
			}
		})
	}
}
