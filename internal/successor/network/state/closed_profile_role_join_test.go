package state

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"fmt"
	networkdomain "github.com/dianabuilds/ardents-network/internal/successor/network"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/closedprofile"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state/durable"
	networkfixture "github.com/dianabuilds/ardents-network/tests/epochfixture/network"
)

// A signed profile must join the domain assigned by a genuinely accepted
// Epoch. Refusal leaves no accepted profile; matching input survives reopen.
func TestClosedProfileRoleDomainMustMatchEpochAssignment(t *testing.T) {
	for _, test := range []struct {
		name      string
		uncertain bool
	}{
		{name: "ordinary acceptance"},
		{name: "post-rename sync uncertainty", uncertain: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			testClosedProfileRoleJoinCase(t, test.uncertain)
		})
	}
}

func testClosedProfileRoleJoinCase(t *testing.T, uncertain bool) {
	now := time.Now().UTC()
	hour := now.Truncate(time.Hour)
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{9}, ed25519.SeedSize))
	networkID := sha256.Sum256([]byte("closed role join network"))
	seed := sha256.Sum256([]byte("closed role join seed"))
	domains := []string{"initiator", "rendezvous"}

	// These frozen families select the stated roles under the signed seed.
	buildRecord := func(id byte, family, endpoint string) networkfixture.Record {
		t.Helper()
		key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{id}, ed25519.SeedSize))
		record, err := networkfixture.BuildRecord(networkfixture.RecordSpec{
			NetworkID: networkID, NodeID: [32]byte{id}, Generation: uint64(id),
			ValidFrom: hour.Add(-time.Minute), ValidUntil: hour.Add(2 * time.Hour),
			Family: family, Endpoint: endpoint, Carrier: closedTCPCarrierProfile,
			Capability: 2, Capacity: 3, PrivateKey: key,
		})
		if err != nil {
			t.Fatal(err)
		}
		return record
	}
	issuer := buildRecord(1, "closed-role-join-rendezvous-0", "127.0.0.1:4101")
	other := buildRecord(2, "closed-role-join-initiator-0", "127.0.0.1:4102")
	epoch, err := networkfixture.BuildEpoch(networkfixture.EpochSpec{
		NetworkID: networkID, Number: 1, ValidFrom: hour, ValidUntil: hour.Add(2 * time.Hour),
		Inputs: [][]byte{issuer.Raw, other.Raw}, Accepted: []networkfixture.Record{issuer, other},
		AssignmentSeed: seed, Domains: domains, Authorities: []ed25519.PrivateKey{authority},
		Profile: closedRouteProfile, Version: 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	config := Config{
		Root: t.TempDir(), NetworkID: networkID,
		Authorities: map[[32]byte]ed25519.PublicKey{
			sha256.Sum256(authority.Public().(ed25519.PublicKey)): authority.Public().(ed25519.PublicKey),
		},
		Threshold: 1, Clock: time.Now, ObserveClock: time.Now,
		AcceptedProfile:        closedRouteProfile,
		ClosedProfileAuthority: authority.Public().(ed25519.PublicKey),
	}
	store, err := Open(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if _, err := store.Accept(context.Background(), epoch.Raw, epoch.Inputs, epoch.Materials[:1]); err != nil {
		t.Fatalf("accept signed Epoch: %v", err)
	}
	issuerEntry := closedProfileNode{nodeID: issuer.NodeID, recordDigest: sha256.Sum256(issuer.Raw),
		domain: 2, subrole: 6, generation: 1}
	otherEntry := closedProfileNode{nodeID: other.NodeID, recordDigest: sha256.Sum256(other.Raw),
		domain: 3, subrole: 1, generation: 2}
	mismatch := testClosedProfileAt(t, authority, networkID, epoch.Digest, epoch.Digest, 1, now,
		[]closedProfileNode{issuerEntry, otherEntry})
	if _, err := closedprofile.Verify(mismatch, closedprofile.Context{StateGeneration: epoch.Digest, NetworkID: networkID, EpochDigest: epoch.Digest, Epoch: 1, Authority: authority.Public().(ed25519.PublicKey), Now: now}); err != nil {
		t.Fatalf("mismatched signed profile is not structurally valid: %v", err)
	}
	if _, err := store.AcceptClosedProfile(mismatch); err == nil {
		t.Fatal("accepted signed Role Domain contradicting the Epoch assignment")
	}
	if _, err := store.CurrentRuntime(); err == nil {
		t.Fatal("mismatched profile exposed a closed route")
	}

	otherEntry.domain = 1
	matching := testClosedProfileAt(t, authority, networkID, epoch.Digest, epoch.Digest, 1, now,
		[]closedProfileNode{issuerEntry, otherEntry})
	if uncertain {
		failure := errors.New("injected post-rename directory sync failure")
		_, err = store.acceptClosedProfileWithCommit(matching, func(state durable.ClosedProfileState, raw []byte) error {
			if err := store.storage.CommitClosedProfile(state, raw); err != nil {
				return err
			}
			return fmt.Errorf("%w: %w", durable.ErrClosedProfileStateSyncUncertain, failure)
		})
		if !errors.Is(err, durable.ErrClosedProfileStateSyncUncertain) || !store.closed {
			t.Fatalf("uncertain signed profile acceptance: err=%v closed=%t", err, store.closed)
		}
		if _, err := store.CurrentRuntime(); err == nil {
			t.Fatal("retired State exposed signed route")
		}
		if err := store.Wait(context.Background()); !errors.Is(err, failure) {
			t.Fatalf("Wait lost sync failure: %v", err)
		}
		if err := store.Close(); !errors.Is(err, failure) {
			t.Fatalf("Close lost sync failure: %v", err)
		}
	} else {
		view, err := store.AcceptClosedProfile(matching)
		if err != nil || view.Digest != sha256.Sum256(matching) {
			t.Fatalf("accept matching profile: %+v / %v", view, err)
		}
		route, err := store.CurrentRuntime()
		if err != nil || len(route.Members()) != 2 || route.Members()[0].RoleDomain != 2 ||
			route.Members()[1].RoleDomain != 1 {
			t.Fatalf("current joined route: %+v / %v", route, err)
		}
		assertSignedRuntimeRecords(t, store, epoch.Digest, issuer, other)
		if err := store.Close(); err != nil {
			t.Fatal(err)
		}
		if _, err := store.CurrentRuntime(); err == nil {
			t.Fatal("closed owner exposed runtime facts")
		}
	}
	reopened, err := Open(config)
	if err != nil {
		t.Fatalf("reopen accepted State/profile: %v", err)
	}
	defer reopened.Close()
	assertSignedRuntimeRecords(t, reopened, epoch.Digest, issuer, other)
	route, err := reopened.CurrentRuntime()
	if err != nil || len(route.Members()) != 2 || route.Members()[0].RoleDomain != 2 ||
		route.Members()[1].RoleDomain != 1 {
		t.Fatalf("reopened joined route: %+v / %v", route, err)
	}
}

// Exercise the runtime projection with signed Epoch inputs and durable profile
// acceptance, including reopening after an uncertain directory sync.
func assertSignedRuntimeRecords(t *testing.T, owner *networkState, generation [32]byte, records ...networkfixture.Record) {
	t.Helper()
	view, err := owner.CurrentRuntime()
	if err != nil {
		t.Fatal(err)
	}
	if view.Profile().Generation != generation || view.Profile().EpochDigest != generation || view.ObservedAt().IsZero() {
		t.Fatal("runtime mixed authenticated generation or lost observation time")
	}
	if len(view.Members()) != len(records) {
		t.Fatal("runtime lost signed members")
	}
	for _, record := range records {
		member, err := view.Member(record.NodeID, view.ObservedAt())
		if err != nil || member.RecordDigest != sha256.Sum256(record.Raw) || member.FamilyID != sha256.Sum256([]byte(record.Family)) || member.Capacity != record.Capacity {
			t.Fatalf("runtime did not join signed record: %+v / %v", member, err)
		}
		byKey, err := view.MemberByKey(member.PublicKey, view.ObservedAt())
		if err != nil || byKey != member || member.DomainProofDigest == [32]byte{} {
			t.Fatalf("authenticated key did not resolve the exact signed participant: %v", err)
		}
		if _, err := view.Member(record.NodeID, member.NotAfter()); err == nil {
			t.Fatal("runtime accepted expired signed member")
		}
	}
	if _, err := view.Member([32]byte{255}, view.ObservedAt()); err == nil {
		t.Fatal("runtime substituted a different signed member")
	}
	member, err := view.Member(records[0].NodeID, view.ObservedAt())
	if err != nil {
		t.Fatal(err)
	}
	duty, err := view.RetainDuty(member.NodeID, view.ObservedAt())
	if err != nil || duty.Generation != generation || duty.NodeID != member.NodeID {
		t.Fatalf("runtime cannot retain the exact signed duty: %+v / %v", duty, err)
	}
	if err := view.MatchDuty(duty, view.ObservedAt()); err != nil {
		t.Fatalf("runtime refused current materialized local duty: %v", err)
	}
	for _, mutate := range []func(*networkdomain.RetainedDuty){
		func(d *networkdomain.RetainedDuty) { d.RecordGeneration++ },
		func(d *networkdomain.RetainedDuty) { d.Generation[0] ^= 1 },
		func(d *networkdomain.RetainedDuty) { d.Epoch.Network[0] ^= 1 },
		func(d *networkdomain.RetainedDuty) { d.PublicKey[0] ^= 1 },
		func(d *networkdomain.RetainedDuty) { d.Epoch.Digest[0] ^= 1 },
		func(d *networkdomain.RetainedDuty) { d.FamilyID[0] ^= 1 },
		func(d *networkdomain.RetainedDuty) { d.Assignment = "responder" },
		func(d *networkdomain.RetainedDuty) { d.Endpoint = "127.0.0.1:4999" },
		func(d *networkdomain.RetainedDuty) { d.CarrierProfile = "other-carrier" },
	} {
		stale := duty
		mutate(&stale)
		if err := view.MatchDuty(stale, view.ObservedAt()); err == nil {
			t.Fatal("runtime accepted incompatible retained local duty")
		}
	}
}
