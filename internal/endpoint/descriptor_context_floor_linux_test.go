//go:build linux

package endpoint

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	servicepublication "github.com/dianabuilds/ardents-network/internal/service/publication"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
)

func floorPublication(t *testing.T, authority ed25519.PrivateKey, network [32]byte, generation uint64, from, until time.Time) (servicepublication.Current, ed25519.PrivateKey) {
	t.Helper()
	public, signer, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { clear(signer) })
	var instance [32]byte
	copy(instance[:], public)
	credential, err := signFixtureCredential(servicepublication.Credential{InstancePublic: instance,
		Generation: generation, NotBefore: from.Unix(), NotAfter: until.Unix(), NetworkID: network, Capabilities: 3}, authority)
	if err != nil {
		t.Fatal(err)
	}
	root, err := servicepublication.Open(servicepublication.Config{Root: networkPrivateRoot(t), NetworkID: network, Authority: authority.Public().(ed25519.PublicKey)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	})
	current, err := root.Publish(t.Context(), servicepublication.PublishInput{Credential: credential, InstanceSigner: signer, Acknowledgement: []byte("explicit floor test publication"), At: from})
	if err != nil {
		t.Fatal(err)
	}
	return current, signer
}

func floorDescriptor(t *testing.T, current servicepublication.Current, signer ed25519.PrivateKey, profile, node [32]byte, revision uint64, slot byte, from, until time.Time) []byte {
	t.Helper()
	raw, _, err := reachability.IssuePrivate(reachability.PrivateIssueInput{Current: current, InstanceSigner: signer, ProfileDigest: profile,
		Introduction: reachability.PrivateIntroduction{Revision: revision, NodeID: node, Slot: fixtureID(slot), RecipientKey: fixtureID(slot + 1), NotBefore: from, NotAfter: until}})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestTextDescriptorFloorBelongsToContextAcrossWorkerLoss(t *testing.T) {
	endpoint, principal := dutyContextEndpoint(t)
	owner := admittedDutyContext(t, endpoint, principal, broker.Connection)
	other := admittedDutyContext(t, endpoint, principal, broker.Connection)
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
	current, signer := floorPublication(t, authority, network, 1, now, now.Add(10*time.Minute))
	target := current.Credential.Target
	raw := floorDescriptor(t, current, signer, profile, node, 2, 20, now, now.Add(100*time.Second))
	if _, err := owner.descriptorHistory.Accept(raw, target, network, profile, now); err != nil {
		t.Fatal(err)
	}
	conflicting := floorDescriptor(t, current, signer, profile, node, 2, 30, now, now.Add(100*time.Second))
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
