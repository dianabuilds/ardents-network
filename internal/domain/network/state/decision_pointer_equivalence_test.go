package state

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/network/duty"
	"github.com/dianabuilds/ardents-network/internal/domain/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/domain/network/source"
	"github.com/dianabuilds/ardents-network/internal/domain/network/state/internal/distribution"
)

// These units prove the NW-7 pointer wave path preserves the historical
// value-passing facts: projections, collision/duty/refusal outcomes, canonical
// durable bytes, nil/zero-value equivalence and caller mutation isolation.
// Oracles project from an owned value copy, independently of the helpers.

// cloneDecisionForOracle deep-copies the exported owned slices so a later
// in-place mutation of the caller's decision cannot hide behind shared backing
// arrays. Unexported verification state is copied by value; the converted
// helpers never read or write it.
func cloneDecisionForOracle(decision epoch.Decision) epoch.Decision {
	clone := decision
	clone.EpochBytes = append([]byte(nil), decision.EpochBytes...)
	clone.Inputs = make([][]byte, len(decision.Inputs))
	for index := range decision.Inputs {
		clone.Inputs[index] = append([]byte(nil), decision.Inputs[index]...)
	}
	clone.Candidates = make([]epoch.Candidate, len(decision.Candidates))
	copy(clone.Candidates, decision.Candidates)
	for index := range decision.Candidates {
		clone.Candidates[index].DomainProof = append([]byte(nil), decision.Candidates[index].DomainProof...)
	}
	return clone
}

func TestDecisionPointerProjectionsMatchValueOracle(t *testing.T) {
	decision, _ := sourceResolverGoldenDecision(t)
	oracle := cloneDecisionForOracle(decision)

	// Independent value projection of the serving duty.
	wantDuty := duty.Duty{Identity: oracle.Snapshot.NodeID,
		Family: sha256.Sum256([]byte(oracle.Snapshot.DeclaredFamily)),
		Class:  "direct-source", State: "live", NotAfter: oracle.Snapshot.ValidUntil}
	if got := sourceServerDuty(&decision); got != wantDuty {
		t.Fatalf("sourceServerDuty pointer projection = %+v, want %+v", got, wantDuty)
	}
	if got, zero := sourceServerDuty(nil), sourceServerDuty(&epoch.Decision{}); got != zero {
		t.Fatalf("nil serving duty = %+v, want zero-value %+v", got, zero)
	}

	// Independent value oracle for the distinct identity/family duty set.
	dutySetOracle := func(value epoch.Decision, predecessors []duty.Duty) []duty.Duty {
		duties := append([]duty.Duty(nil), predecessors...)
		current := duty.Duty{Identity: value.Snapshot.NodeID,
			Family: sha256.Sum256([]byte(value.Snapshot.DeclaredFamily)),
			Class:  "direct-source", State: "live", NotAfter: value.Snapshot.ValidUntil}
		for index := range duties {
			if duties[index].Identity == current.Identity && duties[index].Family == current.Family {
				duties[index] = current
				return duties
			}
		}
		return append(duties, current)
	}
	foreign := duty.Duty{Identity: [32]byte{91}, Family: [32]byte{92}, Class: "direct-source", State: "live"}
	for _, predecessors := range [][]duty.Duty{nil, {foreign}, {wantDuty}, {foreign, wantDuty}} {
		got := servingDutySet(&decision, predecessors)
		want := dutySetOracle(oracle, predecessors)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("servingDutySet(%+v) = %+v, want %+v", predecessors, got, want)
		}
		if nilSet, zeroSet := servingDutySet(nil, predecessors), servingDutySet(&epoch.Decision{}, predecessors); !reflect.DeepEqual(nilSet, zeroSet) {
			t.Fatalf("nil serving duty set = %+v, want zero-value %+v", nilSet, zeroSet)
		}
	}

	// Independent value projection of the public Snapshot facts.
	wantSnapshot := Snapshot{
		Generation: oracle.Snapshot.Generation, NetworkID: oracle.Snapshot.NetworkID,
		Epoch: oracle.Snapshot.Epoch, Digest: oracle.Snapshot.Digest,
		EpochValidFrom: oracle.Snapshot.EpochValidFrom, ValidUntil: oracle.Snapshot.ValidUntil,
		Profile: oracle.Snapshot.Profile, ViewRoot: oracle.Snapshot.ViewRoot, ViewLength: oracle.Snapshot.ViewLength,
		RejectedRoot: oracle.Snapshot.RejectedRoot, RejectedLength: oracle.Snapshot.RejectedLength,
		SourceNodeID: oracle.Snapshot.NodeID, SourceFamily: oracle.Snapshot.DeclaredFamily,
	}
	if got := snapshotFromEpoch(&decision.Snapshot); got != wantSnapshot {
		t.Fatalf("snapshotFromEpoch pointer projection = %+v, want %+v", got, wantSnapshot)
	}
	if got, zero := snapshotFromEpoch(nil), snapshotFromEpoch(&epoch.Snapshot{}); got != zero || got != (Snapshot{}) {
		t.Fatalf("nil Snapshot projection = %+v, want zero %+v", got, zero)
	}

	if !reflect.DeepEqual(decision, oracle) {
		t.Fatal("read-only projections mutated the caller's decision")
	}
}

func TestDecisionPointerCollisionOutcomesMatchValueOracle(t *testing.T) {
	decision, _ := sourceResolverGoldenDecision(t)
	oracle := cloneDecisionForOracle(decision)
	if len(oracle.Candidates) == 0 {
		t.Fatal("golden decision has no Candidate to collide with")
	}
	member := oracle.Candidates[0]
	collidesOracle := func(value epoch.Decision, identity [32]byte, family, endpoint string) bool {
		for _, candidate := range value.Candidates {
			if candidate.NodeID == identity || candidate.KeyID == identity ||
				candidate.Family == family || candidate.Endpoint == endpoint {
				return true
			}
		}
		return false
	}
	for _, test := range []struct {
		name     string
		identity [32]byte
		family   string
		endpoint string
	}{
		{name: "node identity", identity: member.NodeID, family: "other-family", endpoint: "other-endpoint"},
		{name: "key identity", identity: member.KeyID, family: "other-family", endpoint: "other-endpoint"},
		{name: "family", identity: [32]byte{93}, family: member.Family, endpoint: "other-endpoint"},
		{name: "endpoint", identity: [32]byte{93}, family: "other-family", endpoint: member.Endpoint},
		{name: "no collision", identity: [32]byte{93}, family: "other-family", endpoint: "other-endpoint"},
	} {
		t.Run(test.name, func(t *testing.T) {
			want := collidesOracle(oracle, test.identity, test.family, test.endpoint)
			if got := epochDecisionCollides(&decision, test.identity, test.family, test.endpoint); got != want {
				t.Fatalf("epochDecisionCollides = %t, want %t", got, want)
			}
			info := source.Details{Configured: true,
				Identities:      [2][32]byte{test.identity, {94}},
				Families:        [2]string{test.family, "other-family"},
				EndpointHandles: [2]string{test.endpoint, "other-endpoint"}}
			wantAny := collidesOracle(oracle, info.Identities[0], info.Families[0], info.EndpointHandles[0]) ||
				collidesOracle(oracle, info.Identities[1], info.Families[1], info.EndpointHandles[1])
			if got := sourceCollides(info, &decision); got != wantAny {
				t.Fatalf("sourceCollides = %t, want %t", got, wantAny)
			}
			if got := sourceCollides(source.Details{}, &decision); got {
				t.Fatal("unconfigured Source plan collided")
			}
			if epochDecisionCollides(nil, test.identity, test.family, test.endpoint) ||
				sourceCollides(info, nil) || sourceCollides(info, &epoch.Decision{}) {
				t.Fatal("nil/zero decision collided")
			}
			s := &State{config: config{sourceInfo: info}}
			err := s.rejectDecisionSourceCollisions(&decision)
			if wantAny && !errors.Is(err, errSourceRoleCollision) {
				t.Fatalf("collision refusal = %v, want errSourceRoleCollision", err)
			}
			if !wantAny && err != nil {
				t.Fatalf("non-colliding decision refused: %v", err)
			}
			if nilErr, zeroErr := s.rejectDecisionSourceCollisions(nil), s.rejectDecisionSourceCollisions(&epoch.Decision{}); nilErr != nil || zeroErr != nil {
				t.Fatalf("nil/zero decision refused: nil=%v zero=%v", nilErr, zeroErr)
			}
		})
	}
	if !reflect.DeepEqual(decision, oracle) {
		t.Fatal("collision checks mutated the caller's decision")
	}
}

func TestSelectEpochHistoryPointerPreservesFacts(t *testing.T) {
	now := time.Unix(1_800_000_100, 0).UTC()
	predecessor := completionEpoch(1, 1, now)
	successor := completionEpoch(2, 2, now)
	s := &State{current: &predecessor,
		distribution: distribution.Record{EpochFloor: 1, EpochDigest: predecessor.Header.Digest}}
	candidates := []*epoch.Decision{&successor}
	selected, selection, err := s.selectEpochHistory(candidates)
	if err != nil {
		t.Fatalf("selectEpochHistory: %v", err)
	}
	if selected != candidates[selection.Index()] {
		t.Fatal("selection did not return the candidate pointer")
	}
	if selected.Header != successor.Header || selected.Snapshot != successor.Snapshot {
		t.Fatal("selected pointer lost the candidate facts")
	}

	// A nil candidate contributes the same zero facts as the historical zero
	// value, so reconciliation refuses both identically.
	_, nilSelection, nilErr := s.selectEpochHistory([]*epoch.Decision{nil})
	_, zeroSelection, zeroErr := s.selectEpochHistory([]*epoch.Decision{{}})
	if (nilErr == nil) != (zeroErr == nil) || nilErr != nil && nilErr.Error() != zeroErr.Error() {
		t.Fatalf("nil/zero candidate refusal differs: nil=%v zero=%v", nilErr, zeroErr)
	}
	if nilErr == nil && zeroErr == nil && nilSelection.Index() != zeroSelection.Index() {
		t.Fatal("nil/zero candidate selection differs")
	}

	// A second digest at the retained number still refuses through pointers.
	competing := completionEpoch(2, 3, now)
	if _, _, err := s.selectEpochHistory([]*epoch.Decision{&successor, &competing}); !isEpochHistoryConflict(err) {
		t.Fatalf("conflicting candidates reconciled: %v", err)
	}
}

// nw7Successor derives a distinct authenticated-facts successor from a verified
// decision, keeping its owned canonical bytes for durable publication.
func nw7Successor(t *testing.T, current epoch.Decision, seed string) epoch.Decision {
	t.Helper()
	successor := current
	successor.Header.Number++
	successor.Header.Digest = sha256.Sum256([]byte(seed))
	successor.Header.Previous = current.Header.Digest
	successor.Header.ValidUntil = current.Header.ValidUntil.Add(time.Hour)
	successor.Snapshot.Epoch++
	successor.Snapshot.Digest = successor.Header.Digest
	successor.Snapshot.Generation = hex.EncodeToString(successor.Header.Digest[:])
	successor.Snapshot.NodeID = sha256.Sum256([]byte(seed + "-node"))
	successor.Snapshot.DeclaredFamily = seed + "-family"
	successor.Snapshot.ValidUntil = successor.Header.ValidUntil
	return successor
}

func assertPublishedDecisionBytes(t *testing.T, s *State, want epoch.Decision) {
	t.Helper()
	_, values, err := s.storage.LoadState()
	if err != nil {
		t.Fatal(err)
	}
	for _, generation := range values {
		if generation.Name != want.Snapshot.Generation {
			continue
		}
		if !bytes.Equal(generation.Epoch, want.EpochBytes) || !equalInputs(generation.Inputs, want.Inputs) {
			t.Fatal("durable generation bytes differ from the decision's canonical bytes")
		}
		return
	}
	t.Fatalf("durable generation %s is missing", want.Snapshot.Generation)
}

func TestCommitActiveDecisionPointerBytesAndIsolation(t *testing.T) {
	for _, serving := range []bool{false, true} {
		t.Run(map[bool]string{false: "acquiring", true: "serving"}[serving], func(t *testing.T) {
			s, current := newControlCommitFixture(t, serving)
			successor := nw7Successor(t, current, "nw7-active-successor")
			before := cloneDecisionForOracle(successor)
			next := s.distribution
			next.Sequence++
			if err := s.commitActiveDecision(&successor, next); err != nil {
				t.Fatalf("commitActiveDecision: %v", err)
			}
			if s.current != &successor {
				t.Fatal("active publication did not keep the committed pointer")
			}
			if s.current.Header != before.Header || s.current.Snapshot != before.Snapshot ||
				!bytes.Equal(s.current.EpochBytes, before.EpochBytes) || !equalInputs(s.current.Inputs, before.Inputs) {
				t.Fatal("published current lost the committed facts")
			}
			if !reflect.DeepEqual(successor, before) {
				t.Fatal("commit mutated the caller's decision")
			}
			assertPublishedDecisionBytes(t, s, before)
			if s.distribution.EpochFloor != successor.Header.Number || s.distribution.EpochDigest != successor.Header.Digest {
				t.Fatal("distribution floor does not match the committed decision")
			}
			if serving {
				newFamily := sha256.Sum256([]byte(successor.Snapshot.DeclaredFamily))
				if held, err := duty.ReadConflict(s.config.localRoles, s.config.clock, successor.Snapshot.NodeID, newFamily); err != nil || !held {
					t.Fatalf("successor serving duty missing: held=%t err=%v", held, err)
				}
				oldFamily := sha256.Sum256([]byte(current.Snapshot.DeclaredFamily))
				if held, err := duty.ReadConflict(s.config.localRoles, s.config.clock, current.Snapshot.NodeID, oldFamily); err != nil || held {
					t.Fatalf("joined predecessor duty retained: held=%t err=%v", held, err)
				}
			}
		})
	}
}

func TestCommitPendingDecisionPointerBytesAndIsolation(t *testing.T) {
	s, current := newControlCommitFixture(t, false)
	pending := nw7Successor(t, current, "nw7-pending-successor")
	pending.Header.ValidFrom = s.config.clock().Add(20 * time.Second)
	before := cloneDecisionForOracle(pending)
	next := s.distribution
	next.Sequence++
	if err := s.commitPendingDecision(&pending, next); err != nil {
		t.Fatalf("commitPendingDecision: %v", err)
	}
	if s.pendingDecision != &pending {
		t.Fatal("pending publication did not keep the committed pointer")
	}
	if s.pendingDecision.Header != before.Header || s.pendingDecision.Snapshot != before.Snapshot {
		t.Fatal("published pending lost the committed facts")
	}
	if !reflect.DeepEqual(pending, before) {
		t.Fatal("staging mutated the caller's decision")
	}
	assertPublishedDecisionBytes(t, s, before)
	if s.distribution.PendingDigest != pending.Header.Digest || s.distribution.PendingValidFrom != pending.Header.ValidFrom.Unix() {
		t.Fatal("distribution pending marker does not match the staged decision")
	}
}

func TestCompleteSourceWavePointerPathPreservesResultFacts(t *testing.T) {
	for _, serving := range []bool{false, true} {
		t.Run(map[bool]string{false: "acquiring", true: "serving"}[serving], func(t *testing.T) {
			s, current := newControlCommitFixture(t, serving)
			// This fixture verifies committed result facts. Advance both
			// reference readings from the fixture's original monotonic origin
			// so disk latency cannot create artificial clock disagreement.
			wall, origin := s.config.anchorWall, s.config.anchorMono
			clock := func() time.Time { return wall.Add(time.Since(origin)) }
			s.config.clock, s.config.observe = clock, clock
			now := s.config.clock()
			wave := s.distribution
			wave.Sequence++
			wave.CycleID++
			wave.CycleActive = true
			wave.CyclePurpose = distribution.Refresh
			wave.CycleStarted = now.Unix()
			wave.CycleDeadline = now.Add(15 * time.Second).Unix()
			wave.CycleSeed = sha256.Sum256([]byte("nw7 pointer wave cycle"))
			wave.SourceOrder = [2]byte{0, 1}
			wave.Attempts[0], wave.Attempts[1] = distribution.AttemptInFlight, distribution.AttemptInFlight
			if err := s.commitDistribution(wave); err != nil {
				t.Fatal(err)
			}
			successor := nw7Successor(t, current, "nw7-wave-successor")
			results := []sourceResult{
				{index: 0, slot: 0, decision: successor, observations: [4]byte{distribution.OutcomeValid}},
				{index: 1, slot: 1, decision: successor, observations: [4]byte{0, distribution.OutcomeValid}},
			}
			beforeFirst := cloneDecisionForOracle(results[0].decision)
			beforeSecond := cloneDecisionForOracle(results[1].decision)
			snapshot, err := s.completeSourceWave(now, s.current, results)
			if err != nil {
				t.Fatalf("completeSourceWave: %v", err)
			}
			if snapshot.Digest != successor.Header.Digest || s.current == nil || s.current.Header.Digest != successor.Header.Digest {
				t.Fatalf("wave did not publish the successor: snapshot=%+v current=%v", snapshot, s.current)
			}
			if s.current != &results[0].decision {
				t.Fatal("wave publication copied the selected result instead of keeping its pointer")
			}
			if s.current.Snapshot != beforeFirst.Snapshot || !bytes.Equal(s.current.EpochBytes, beforeFirst.EpochBytes) ||
				!equalInputs(s.current.Inputs, beforeFirst.Inputs) {
				t.Fatal("published wave current lost the result facts")
			}
			if !reflect.DeepEqual(results[0].decision, beforeFirst) || !reflect.DeepEqual(results[1].decision, beforeSecond) {
				t.Fatal("wave completion mutated the caller's results")
			}
			assertPublishedDecisionBytes(t, s, beforeFirst)
			if s.distribution.ObservedEpochs[0] != successor.Header.Number || s.distribution.ObservedDigests[0] != successor.Header.Digest ||
				s.distribution.ObservedEpochs[1] != successor.Header.Number || s.distribution.ObservedDigests[1] != successor.Header.Digest {
				t.Fatal("wave observations do not match the selected decision")
			}
			if serving {
				newFamily := sha256.Sum256([]byte(successor.Snapshot.DeclaredFamily))
				if held, err := duty.ReadConflict(s.config.localRoles, s.config.clock, successor.Snapshot.NodeID, newFamily); err != nil || !held {
					t.Fatalf("wave successor serving duty missing: held=%t err=%v", held, err)
				}
			}
		})
	}
}

func TestNilDecisionCommitHelpersMatchZeroValue(t *testing.T) {
	startCycle := func(s *State) {
		t.Helper()
		now := s.config.clock()
		wave := s.distribution
		wave.Sequence++
		wave.CycleID++
		wave.CycleActive = true
		wave.CyclePurpose = distribution.Refresh
		wave.CycleStarted = now.Unix()
		wave.CycleDeadline = now.Add(15 * time.Second).Unix()
		wave.CycleSeed = sha256.Sum256([]byte("nw7 nil equivalence cycle"))
		wave.SourceOrder = [2]byte{0, 1}
		wave.Attempts[0], wave.Attempts[1] = distribution.AttemptInFlight, distribution.AttemptInFlight
		if err := s.commitDistribution(wave); err != nil {
			t.Fatal(err)
		}
	}
	summary := sourceWaveSummary{outcomes: [4]byte{distribution.OutcomeValid, distribution.OutcomeValid}}

	t.Run("active commit", func(t *testing.T) {
		nilState, _ := newControlCommitFixture(t, false)
		zeroState, _ := newControlCommitFixture(t, false)
		next := nilState.distribution
		next.Sequence++
		nilErr := nilState.commitActiveDecision(nil, next)
		zeroErr := zeroState.commitActiveDecision(&epoch.Decision{}, next)
		if nilErr == nil || zeroErr == nil || nilErr.Error() != zeroErr.Error() {
			t.Fatalf("nil/zero active commit differs: nil=%v zero=%v", nilErr, zeroErr)
		}
	})
	t.Run("pending commit", func(t *testing.T) {
		nilState, _ := newControlCommitFixture(t, false)
		zeroState, _ := newControlCommitFixture(t, false)
		next := nilState.distribution
		next.Sequence++
		nilErr := nilState.commitPendingDecision(nil, next)
		zeroErr := zeroState.commitPendingDecision(&epoch.Decision{}, next)
		if nilErr == nil || zeroErr == nil || nilErr.Error() != zeroErr.Error() {
			t.Fatalf("nil/zero pending commit differs: nil=%v zero=%v", nilErr, zeroErr)
		}
	})
	t.Run("persist", func(t *testing.T) {
		nilState, _ := newControlCommitFixture(t, false)
		zeroState, _ := newControlCommitFixture(t, false)
		nilErr := persistDecision(nilState.storage, nil, false)
		zeroErr := persistDecision(zeroState.storage, &epoch.Decision{}, false)
		if nilErr == nil || zeroErr == nil || nilErr.Error() != zeroErr.Error() {
			t.Fatalf("nil/zero persist differs: nil=%v zero=%v", nilErr, zeroErr)
		}
	})
	t.Run("deferred wave", func(t *testing.T) {
		nilState, _ := newControlCommitFixture(t, false)
		zeroState, _ := newControlCommitFixture(t, false)
		startCycle(nilState)
		startCycle(zeroState)
		now := nilState.config.clock()
		_, nilErr := nilState.commitDeferredSourceWave(now, nil, summary)
		_, zeroErr := zeroState.commitDeferredSourceWave(now, &epoch.Decision{}, summary)
		if nilErr == nil || zeroErr == nil || nilErr.Error() != zeroErr.Error() {
			t.Fatalf("nil/zero deferred wave differs: nil=%v zero=%v", nilErr, zeroErr)
		}
		if !errors.Is(nilErr, errRefreshUnavailable) || nilState.distribution.NextAutomatic != 0 {
			t.Fatalf("zero ValidFrom moved the automatic bound: %v", nilState.distribution.NextAutomatic)
		}
	})
	t.Run("active wave", func(t *testing.T) {
		nilState, _ := newControlCommitFixture(t, false)
		zeroState, _ := newControlCommitFixture(t, false)
		startCycle(nilState)
		startCycle(zeroState)
		now := nilState.config.clock()
		_, nilErr := nilState.commitActiveSourceWave(now, nil, summary)
		_, zeroErr := zeroState.commitActiveSourceWave(now, &epoch.Decision{}, summary)
		if nilErr == nil || zeroErr == nil || nilErr.Error() != zeroErr.Error() {
			t.Fatalf("nil/zero active wave differs: nil=%v zero=%v", nilErr, zeroErr)
		}
	})
}
