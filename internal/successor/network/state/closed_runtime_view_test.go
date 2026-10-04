package state

import (
	networkdomain "github.com/dianabuilds/ardents-network/internal/successor/network"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/state/durable"
)

func TestClosedProfileRejectsAmbiguousRecordBindingBeforeCommit(t *testing.T) {
	owner, raw := closedProfileStoreFixture(t)
	owner.current.Candidates = append(owner.current.Candidates, owner.current.Candidates[0])
	if _, err := owner.AcceptClosedProfile(raw); err == nil {
		t.Fatal("profile accepted an ambiguous record binding")
	}
	generation, err := closedProfileGeneration(owner.current.Snapshot.Generation)
	if err != nil {
		t.Fatal(err)
	}
	stored, _, err := owner.storage.LoadClosedProfile(generation)
	if err != nil || stored != (durable.ClosedProfileState{}) {
		t.Fatalf("ambiguous membership was durably accepted: %+v / %v", stored, err)
	}
}

func TestClosedRuntimeRejectsExpiryDuringDurableProfileRead(t *testing.T) {
	owner, raw := closedProfileStoreFixture(t)
	if _, err := owner.AcceptClosedProfile(raw); err != nil {
		t.Fatal(err)
	}
	now := owner.config.clock()
	owner.config.clock = func() time.Time { return now }
	owner.config.observe = owner.config.clock
	owner.mu.RLock()
	defer owner.mu.RUnlock()
	if _, _, err := owner.currentMembershipLocked(owner.storage.LoadClosedProfile); err != nil {
		t.Fatalf("positive control: %v", err)
	}
	_, _, err := owner.currentMembershipLocked(func(generation [32]byte) (durable.ClosedProfileState, []byte, error) {
		stored, body, err := owner.storage.LoadClosedProfile(generation)
		// The durable read completes at the expiry boundary, before State
		// verifies the returned bytes and publishes a usable observation.
		now = owner.current.Snapshot.ValidUntil
		return stored, body, err
	})
	if err == nil {
		t.Fatal("profile expired during I/O was published as current")
	}
}

func TestClosedRuntimeDoesNotReviveMemberUsingCallerTimeBeforeRead(t *testing.T) {
	owner, raw := closedProfileStoreFixture(t)
	beforeRead := owner.config.clock().Add(time.Second)
	observed := beforeRead.Add(time.Minute)
	owner.config.clock = func() time.Time { return observed }
	owner.config.observe = owner.config.clock
	member := &owner.current.Candidates[0]
	member.PublicKey, member.FamilyID, member.Capacity = [32]byte{1}, [32]byte{2}, 1
	member.ValidFrom, member.ValidUntil = beforeRead.Add(-time.Second), beforeRead.Add(time.Second)
	member.AssignmentNotAfter = observed.Add(time.Hour)
	if _, err := owner.AcceptClosedProfile(raw); err != nil {
		t.Fatal(err)
	}
	view, err := owner.CurrentRuntime()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := view.Member(member.NodeID, beforeRead); err == nil {
		t.Fatal("caller time revived a member already expired at State observation")
	}
	if view.Members()[0].Current(beforeRead) {
		t.Fatal("inventory revived a member already expired at State observation")
	}
}

func TestClosedRuntimeZeroAndCopiedFacts(t *testing.T) {
	if err := (networkdomain.RuntimeView{}).Check(time.Now()); err == nil {
		t.Fatal("zero view accepted")
	}
	owner, raw := closedProfileStoreFixture(t)
	now := owner.config.clock()
	member := &owner.current.Candidates[0]
	member.PublicKey, member.FamilyID, member.Capacity = [32]byte{1}, [32]byte{2}, 1
	member.ValidFrom, member.ValidUntil, member.AssignmentNotAfter = now.Add(-time.Minute), now.Add(time.Hour), now.Add(time.Hour)
	if _, err := owner.AcceptClosedProfile(raw); err != nil {
		t.Fatal(err)
	}
	view, err := owner.CurrentRuntime()
	if err != nil {
		t.Fatal(err)
	}
	first, err := view.Member(member.NodeID, now)
	if err != nil {
		t.Fatal(err)
	}
	copy := view.Members()
	copy[0].PublicKey = [32]byte{99}
	second, err := view.Member(member.NodeID, now)
	if err != nil || first != second {
		t.Fatal("returned member mutated owner")
	}
	profile := view.Profile()
	profile.TokenKeys[0].SPKI[0] ^= 1
	if profile.TokenKeys[0] == view.Profile().TokenKeys[0] {
		t.Fatal("profile copy aliases owner")
	}
	if _, err := view.Member(member.NodeID, member.ValidUntil); err == nil {
		t.Fatal("expired member accepted")
	}
	owner.mu.Lock()
	owner.distribution.conflicting = true
	owner.mu.Unlock()
	if _, err := owner.CurrentRuntime(); err == nil {
		t.Fatal("conflicting State exposed runtime facts")
	}
}

func TestClosedRuntimeIncompleteMemberIsLocal(t *testing.T) {
	owner, raw := closedProfileStoreFixture(t)
	if _, err := owner.AcceptClosedProfile(raw); err != nil {
		t.Fatal(err)
	}
	view, err := owner.CurrentRuntime()
	if err != nil {
		t.Fatal(err)
	}
	if err := view.Check(owner.config.clock()); err != nil {
		t.Fatal(err)
	}
	if _, err := view.Member(owner.current.Candidates[0].NodeID, owner.config.clock()); err == nil {
		t.Fatal("incomplete member accepted")
	}
}

func TestClosedRuntimeMemberValidityIsLocal(t *testing.T) {
	for _, future := range []bool{false, true} {
		t.Run(map[bool]string{false: "expired", true: "not yet active"}[future], func(t *testing.T) {
			owner, raw := closedProfileStoreFixture(t)
			now := owner.config.clock()
			member := &owner.current.Candidates[0]
			member.PublicKey, member.FamilyID, member.Capacity = [32]byte{1}, [32]byte{2}, 1
			member.ValidFrom, member.ValidUntil = now.Add(-time.Hour), now
			member.AssignmentNotAfter = now.Add(time.Hour)
			if future {
				member.ValidFrom, member.ValidUntil = now.Add(time.Minute), now.Add(time.Hour)
			}
			if _, err := owner.AcceptClosedProfile(raw); err != nil {
				t.Fatal(err)
			}
			view, err := owner.CurrentRuntime()
			if err != nil || view.Check(now) != nil {
				t.Fatalf("one unavailable member retired the profile: %v", err)
			}
			if _, err := view.Member(member.NodeID, now); err == nil {
				t.Fatal("unavailable member accepted")
			}
		})
	}
}
