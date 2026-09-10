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

func TestTextDescriptorFloorRejectsRollbackAndRetainsConflicts(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	network, profile, node := fixtureID(1), fixtureID(2), fixtureID(3)
	_, authority, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(authority)
	current, signer := textFloorPublication(t, authority, network, 1, now, now.Add(10*time.Minute))
	owner := &textContext{}
	accept := func(raw []byte, at time.Time) error {
		_, err := owner.acceptTextDescriptorLocked(raw, current.Credential.Target, network, profile, at)
		return err
	}
	first := textFloorDescriptor(t, current, signer, profile, node, 1, 10, now, now.Add(300*time.Second))
	second := textFloorDescriptor(t, current, signer, profile, node, 2, 20, now, now.Add(100*time.Second))
	if err := accept(first, now); err != nil {
		t.Fatal(err)
	}
	if err := accept(second, now); err != nil {
		t.Fatalf("higher revision with shorter expiry: %v", err)
	}
	if err := accept(second, now); err != nil {
		t.Fatalf("exact retry: %v", err)
	}
	if err := accept(first, now); err == nil {
		t.Fatal("lower revision revived")
	}
	if err := accept(first, now.Add(110*time.Second)); err == nil {
		t.Fatal("expired successor revived still-valid predecessor")
	}
	forged := append([]byte(nil), second...)
	forged[len(forged)-1] ^= 1
	if err := accept(forged, now); err == nil {
		t.Fatal("forged signature accepted")
	}
	if err := accept(second, now); err != nil {
		t.Fatalf("forgery poisoned floor: %v", err)
	}
	conflicting := textFloorDescriptor(t, current, signer, profile, node, 2, 30, now, now.Add(120*time.Second))
	if err := accept(conflicting, now); err == nil {
		t.Fatal("same revision conflict accepted")
	}
	if err := accept(second, now); err == nil {
		t.Fatal("exact retry erased revision conflict")
	}
	third := textFloorDescriptor(t, current, signer, profile, node, 3, 40, now, now.Add(140*time.Second))
	if err := accept(third, now); err != nil {
		t.Fatalf("higher revision failed to repair revision conflict: %v", err)
	}
	// Caller mutation cannot change the retained hashes or create cache aliases.
	verified, err := owner.acceptTextDescriptorLocked(third, current.Credential.Target, network, profile, now)
	if err != nil {
		t.Fatal(err)
	}
	clear(verified.Current.Record)
	clear(verified.Descriptor.Publication)
	if err := accept(third, now); err != nil {
		t.Fatalf("returned proof aliases retained state: %v", err)
	}
}

func TestTextDescriptorFloorPublicationConflictRetainsLongestAuthority(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	network, profile, node := fixtureID(1), fixtureID(2), fixtureID(3)
	_, authority, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(authority)
	owner := &textContext{}
	first, signer := textFloorPublication(t, authority, network, 1, now, now.Add(100*time.Second))
	firstRaw := textFloorDescriptor(t, first, signer, profile, node, 1, 10, now, now.Add(100*time.Second))
	accept := func(raw []byte, at time.Time) error {
		_, err := owner.acceptTextDescriptorLocked(raw, first.Credential.Target, network, profile, at)
		return err
	}
	if err := accept(firstRaw, now); err != nil {
		t.Fatal(err)
	}
	other, otherSigner := textFloorPublication(t, authority, network, 1, now, now.Add(200*time.Second))
	otherRaw := textFloorDescriptor(t, other, otherSigner, profile, node, 2, 20, now, now.Add(150*time.Second))
	if err := accept(otherRaw, now); err == nil {
		t.Fatal("conflicting Instance at same generation accepted")
	}
	higherRevision := textFloorDescriptor(t, first, signer, profile, node, 3, 30, now, now.Add(90*time.Second))
	if err := accept(higherRevision, now); err == nil {
		t.Fatal("revision repaired publication conflict")
	}
	overlapping, overlapSigner := textFloorPublication(t, authority, network, 2, now.Add(100*time.Second), now.Add(300*time.Second))
	overlapRaw := textFloorDescriptor(t, overlapping, overlapSigner, profile, node, 1, 40, now.Add(100*time.Second), now.Add(300*time.Second))
	if err := accept(overlapRaw, now.Add(100*time.Second)); err == nil {
		t.Fatal("successor overlapped longer conflicting authority")
	}
	next, nextSigner := textFloorPublication(t, authority, network, 3, now.Add(200*time.Second), now.Add(400*time.Second))
	nextRaw := textFloorDescriptor(t, next, nextSigner, profile, node, 1, 50, now.Add(200*time.Second), now.Add(400*time.Second))
	if err := accept(nextRaw, now.Add(200*time.Second)); err != nil {
		t.Fatalf("non-overlapping successor refused: %v", err)
	}
	if err := accept(overlapRaw, now.Add(210*time.Second)); err == nil {
		t.Fatal("older publication generation revived")
	}
}

func TestTextDescriptorFloorBelongsToContextAcrossWorkerLoss(t *testing.T) {
	endpoint, principal := textContextEndpoint(t)
	owner := admittedTextContext(t, endpoint, principal, broker.Connection)
	other := admittedTextContext(t, endpoint, principal, broker.Connection)
	job, err := owner.beginJob(endpoint, broker.Connection)
	if err != nil {
		t.Fatal(err)
	}
	target := fixtureID(9)
	owner.descriptorFloors = map[[32]byte]textDescriptorFloor{target: {generation: 1, revision: 2, revisionConflict: true}}
	owner.retireJob(job)
	if err := owner.finishJobCleanup(job, nil); err != nil {
		t.Fatal(err)
	}
	if !owner.descriptorFloors[target].revisionConflict {
		t.Fatal("worker loss erased context floor")
	}
	if len(other.descriptorFloors) != 0 {
		t.Fatal("context history shared with another authorization")
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if owner.descriptorFloors != nil {
		t.Fatal("retired context retained private history")
	}
}

// The existing real Store has revision 1. The Endpoint separately verifies a
// signed revision 2 as a prior observation; the next genuine network response
// must be refused locally even though that Node still considers revision 1 live.
func TestTextResolutionNetworkCannotRollBackLocalDescriptorFloor(t *testing.T) {
	for _, carrier := range []route.CarrierProfile{route.ClosedCarrierTCP, route.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			_, owner, source := startTextControlNetwork(t, carrier, true)
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
			status, _, err := prefix.ExchangeDescriptor(t.Context(), func(hello route.ClosedHello, class uint8) ([]byte, error) {
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
			floor := owner.descriptorFloors[current.Credential.Target]
			_, err = owner.acceptTextDescriptorLocked(second, current.Credential.Target, profile.NetworkID, profile.Digest, now)
			owner.mu.Unlock()
			if floor.revision != 1 || err != nil {
				t.Fatalf("lookup failed to retain floor or prior observation invalid: %v", err)
			}
			if _, err := owner.lookupTextDescriptor(t.Context(), current.Credential.Target); err == nil {
				t.Fatal("actual resolver response rolled back locally retained revision")
			}
			owner.mu.Lock()
			retained := owner.descriptorFloors[current.Credential.Target]
			healthy := owner.prefix == prefix && owner.resolution == nil && !owner.closed
			owner.mu.Unlock()
			if retained.revision != 2 || !healthy {
				t.Fatal("ordinary stale response erased floor or damaged context")
			}
			// Explicit capacity fixture: retained hashes stand in for already
			// observed Targets. The real admitted issuer/stock remains in use.
			owner.mu.Lock()
			for index := 1; len(owner.descriptorFloors) < maximumTextDescriptorTargets; index++ {
				owner.descriptorFloors[fixtureID(byte(index))] = textDescriptorFloor{generation: 1, revision: 1}
			}
			_, err = owner.acceptTextDescriptorLocked(second, current.Credential.Target, profile.NetworkID, profile.Digest, now)
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
			unchanged := len(owner.descriptorFloors) == maximumTextDescriptorTargets &&
				owner.descriptorFloors[current.Credential.Target].revision == 2 && owner.permission.reserved == reserved &&
				owner.permission.batches == batches && tokensBefore == tokensAfter && owner.resolution == nil && owner.issuance == nil
			owner.mu.Unlock()
			if !unchanged {
				t.Fatal("capacity refusal evicted floors or consumed network issuance/admission")
			}
		})
	}
}
