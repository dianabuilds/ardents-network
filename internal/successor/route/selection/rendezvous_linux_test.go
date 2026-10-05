//go:build linux

package selection

import (
	"bytes"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route"
)

func TestRendezvousDrawDoesNotWeightFirstByAlternativeCount(t *testing.T) {
	candidates := []network.Member{
		{NodeID: [32]byte{1}, PublicKey: [32]byte{11}, FamilyID: [32]byte{21}},
		{NodeID: [32]byte{2}, PublicKey: [32]byte{12}, FamilyID: [32]byte{21}},
		{NodeID: [32]byte{3}, PublicKey: [32]byte{13}, FamilyID: [32]byte{23}},
	}
	// Enumerate the three equiprobable first indices, not random frequencies.
	// Member 3 has two possible alternatives; members 1 and 2 have only one.
	for index := range candidates {
		pair, err := chooseRendezvous(candidates, bytes.NewReader([]byte{byte(index), 0}))
		if err != nil || len(pair) != 2 || pair[0] != candidates[index] {
			t.Fatalf("first draw %d was weighted by pair count: %v / %v", index, pair, err)
		}
		if pair[0].NodeID == pair[1].NodeID || pair[0].FamilyID == pair[1].FamilyID {
			t.Fatal("alternative conflicts with initial member")
		}
	}
	if pair, err := chooseRendezvous(candidates[:1], bytes.NewReader([]byte{0})); err != nil || len(pair) != 1 {
		t.Fatalf("one eligible Rendezvous must be usable: %v / %v", pair, err)
	}
	if _, err := chooseRendezvous(candidates, bytes.NewReader(nil)); err == nil {
		t.Fatal("unavailable randomness selected a participant")
	}
	if selected, err := chooseRendezvous(candidates, bytes.NewReader([]byte{2})); err == nil || len(selected) != 1 || selected[0] != candidates[2] {
		t.Fatal("failed alternative draw discarded an already selected first member")
	}
}

func TestRendezvousRetainsDrawAcrossPostSelectionFailure(t *testing.T) {
	now := time.Unix(1900000000, 0).UTC()
	view, source, _ := rendezvousModel(t, now, nil)
	calls := 0
	failure := errors.New("observation lost after selection")
	owner, err := NewRendezvous(source, func() (network.RuntimeView, error) {
		calls++
		if calls == 2 {
			return network.RuntimeView{}, failure
		}
		return view, nil
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Duty(0, nil); !errors.Is(err, failure) {
		t.Fatal("post-selection observation was not required", err)
	}
	retained := append([]network.RetainedDuty(nil), owner.duties...)
	if len(retained) != 2 {
		t.Fatal("failed return discarded the preselected pair")
	}
	for slot, want := range retained {
		got, err := owner.Duty(uint8(slot), nil)
		if err != nil || got != want {
			t.Fatal("failure redrew or lost a retained slot", err)
		}
	}
	selected, _ := view.Member(retained[0].NodeID, now)
	if _, err := owner.Duty(0, []route.Member{{FamilyID: selected.FamilyID}}); err == nil {
		t.Fatal("newly learned conflict accepted")
	}
	if owner.duties[0] != retained[0] || owner.duties[1] != retained[1] {
		t.Fatal("conflict expanded or changed retained choices")
	}
	if _, err := owner.Duty(2, nil); err == nil {
		t.Fatal("third choice accepted")
	}
}

func TestRendezvousExactIncomingDutyUsesResponderAndAllRetainedPeers(t *testing.T) {
	now := time.Unix(1900000000, 0).UTC()
	view, _, responder := rendezvousModel(t, now, nil)
	got, err := responder.RendezvousDuty(view, [32]byte{9}, 1, nil)
	if err != nil || got.NodeID != [32]byte{9} {
		t.Fatal("exact eligible incoming choice refused", err)
	}
	if _, err := responder.RendezvousDuty(view, [32]byte{9}, 2, nil); err == nil {
		t.Fatal("incoming duty generation was silently rebound")
	}
	for _, conflict := range []route.Member{{NodeID: [32]byte{9}}, {PublicKey: [32]byte{29}}, {FamilyID: [32]byte{49}}} {
		withConflict := responder
		withConflict.known = append(append([]route.Member(nil), responder.known...), conflict)
		if _, err := withConflict.RendezvousDuty(view, [32]byte{9}, 1, nil); err == nil {
			t.Fatal("retained alternative conflict ignored")
		}
	}
	if _, err := responder.RendezvousDuty(view, [32]byte{1}, 1, nil); err == nil {
		t.Fatal("forwarding duty accepted as Rendezvous")
	}
}

func TestRendezvousRetentionExpiryAndClockCannotRenewChoice(t *testing.T) {
	for _, lifetime := range []time.Duration{5 * time.Minute, time.Hour} {
		t.Run(lifetime.String(), func(t *testing.T) {
			now := time.Unix(1900000000, 0).UTC()
			adjust := func(_ *network.ProfileFacts, records []network.NodeRecord, _ []network.Assignment) {
				for i := 8; i < 11; i++ {
					records[i].ValidUntil = now.Add(lifetime)
				}
			}
			view, source, _ := rendezvousModel(t, now, adjust)
			owner, err := NewRendezvous(source, func() (network.RuntimeView, error) { return view, nil }, nil)
			if err != nil || !owner.NotAfter().IsZero() {
				t.Fatal("construction selected before current observation", err)
			}
			original, err := owner.Duty(0, nil)
			if err != nil {
				t.Fatal(err)
			}
			until := now.Add(min(lifetime, 30*time.Minute))
			if !owner.NotAfter().Equal(until) {
				t.Fatal("wrong public-choice retention", owner.NotAfter(), until)
			}
			view, _, _ = rendezvousModelAt(t, now, until.Add(-time.Second), adjust)
			if got, err := owner.Duty(0, nil); err != nil || got != original {
				t.Fatal("unexpired retained duty lost", err)
			}
			view, _, _ = rendezvousModelAt(t, now, now, adjust)
			if _, err := owner.Duty(0, nil); err == nil {
				t.Fatal("regressed clock accepted")
			}
			view, _, _ = rendezvousModelAt(t, now, until, adjust)
			if _, err := owner.Duty(0, nil); err == nil {
				t.Fatal("expired choice redrawn or renewed")
			}
			if owner.duties[0] != original || !owner.NotAfter().Equal(until) {
				t.Fatal("refusal mutated original choice or bound")
			}
		})
	}
}

func TestRendezvousRecipientChangeCannotRebindOrExpandSet(t *testing.T) {
	for _, change := range []string{"duty", "Record", "key", "family", "address", "Carrier", "role", "profile", "expiry", "ambiguous key"} {
		t.Run(change, func(t *testing.T) {
			now := time.Unix(1900000000, 0).UTC()
			view, source, _ := rendezvousModel(t, now, nil)
			owner, err := NewRendezvous(source, func() (network.RuntimeView, error) { return view, nil }, nil)
			if err != nil {
				t.Fatal(err)
			}
			original, err := owner.Duty(0, nil)
			if err != nil {
				t.Fatal(err)
			}
			other := owner.duties[1]
			view, _, _ = rendezvousModelAt(t, now, now.Add(time.Second), func(profile *network.ProfileFacts, records []network.NodeRecord, assignments []network.Assignment) {
				i := int(original.NodeID[0]) - 1
				switch change {
				case "duty":
					records[i].Generation++
					assignments[i].Generation++
				case "Record":
					records[i].RecordDigest = [32]byte{200}
					assignments[i].RecordDigest = records[i].RecordDigest
				case "key":
					records[i].PublicKey = [32]byte{201}
				case "family":
					records[i].FamilyID = [32]byte{202}
				case "address":
					records[i].Endpoint = "127.0.0.1:2001"
				case "Carrier":
					records[i].CarrierProfile = "ardents-carrier-quic-v2"
				case "role":
					assignments[i].Subrole = 5
				case "profile":
					profile.Digest = [32]byte{203}
				case "expiry":
					records[i].ValidUntil = now.Add(time.Second)
				case "ambiguous key":
					records[11].PublicKey = records[i].PublicKey
				}
			})
			if _, err := owner.Duty(0, nil); err == nil {
				t.Fatal("changed recipient or profile accepted")
			}
			if owner.duties[0] != original || owner.duties[1] != other || len(owner.duties) != 2 {
				t.Fatal("failed recipient expanded or replaced choices")
			}
		})
	}
}

func TestRendezvousInitialConflictsAndOwnership(t *testing.T) {
	now := time.Unix(1900000000, 0).UTC()
	view, source, responder := rendezvousModel(t, now, nil)
	current := func() (network.RuntimeView, error) { return view, nil }
	if _, err := NewRendezvous(responder, current, nil); err == nil {
		t.Fatal("Responder selected a substitute Rendezvous")
	}
	if _, err := NewRendezvous(source, nil, nil); err == nil {
		t.Fatal("missing genuine observer accepted")
	}
	excluded := []route.Member{{NodeID: [32]byte{9}}, {PublicKey: [32]byte{30}}, {FamilyID: [32]byte{51}}}
	owner, err := NewRendezvous(source, current, excluded)
	if err != nil {
		t.Fatal(err)
	}
	clear(excluded)
	if _, err := owner.Duty(0, nil); err == nil || len(owner.duties) != 0 {
		t.Fatal("caller mutation removed copied exclusions")
	}
	// Exclude both unused alternatives from each retained Source pair, as well
	// as a supplied local observation, leaving exactly one eligible recipient.
	view, source, _ = rendezvousModel(t, now, func(_ *network.ProfileFacts, records []network.NodeRecord, _ []network.Assignment) {
		records[8].FamilyID = records[1].FamilyID
		records[9].FamilyID = records[3].FamilyID
	})
	owner, err = NewRendezvous(source, current, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := owner.Duty(0, nil); err != nil || got.NodeID != [32]byte{11} {
		t.Fatal("retained Entry/Interior alternatives were not excluded", err)
	}
	if _, err := owner.Duty(1, nil); err == nil {
		t.Fatal("missing alternative caused another selection")
	}
}

func TestRendezvousConcurrentRequestsRetainOneChoice(t *testing.T) {
	now := time.Unix(1900000000, 0).UTC()
	view, source, _ := rendezvousModel(t, now, nil)
	owner, err := NewRendezvous(source, func() (network.RuntimeView, error) { return view, nil }, nil)
	if err != nil {
		t.Fatal(err)
	}
	results := make(chan network.RetainedDuty, 16)
	var workers sync.WaitGroup
	for range cap(results) {
		workers.Go(func() {
			duty, err := owner.Duty(0, nil)
			if err != nil {
				t.Error(err)
			}
			results <- duty
		})
	}
	workers.Wait()
	close(results)
	for duty := range results {
		if duty != owner.duties[0] {
			t.Fatal("concurrent request selected an independent set")
		}
	}
}

func TestRendezvousReplacementLegKeepsContextChoiceAndDeadline(t *testing.T) {
	now := time.Unix(1900000000, 0).UTC()
	withoutThird := func(_ *network.ProfileFacts, _ []network.NodeRecord, assignments []network.Assignment) {
		assignments[10].Subrole = 5
	}
	view, source, _ := rendezvousModel(t, now, withoutThird)
	source.NotAfter = now.Add(time.Minute)
	owner, err := NewRendezvous(source, func() (network.RuntimeView, error) { return view, nil }, nil)
	if err != nil {
		t.Fatal(err)
	}
	var original [2]network.RetainedDuty
	for slot := range original {
		original[slot], err = owner.Duty(uint8(slot), nil)
		if err != nil {
			t.Fatal(err)
		}
	}
	until := now.Add(30 * time.Minute)
	if !owner.NotAfter().Equal(until) {
		t.Fatal("context choice was bounded by the first physical prefix")
	}
	// The old physical leg has expired. A fresh leg uses the other retained
	// Entry/Interior members, and Network now contains a third eligible duty.
	var replacement Leg
	view, replacement, _ = rendezvousModelAt(t, now, now.Add(2*time.Minute), nil)
	replacement.Entry, err = view.RetainDuty([32]byte{2}, view.ObservedAt())
	if err != nil {
		t.Fatal(err)
	}
	replacement.Interior, err = view.RetainDuty([32]byte{4}, view.ObservedAt())
	if err != nil {
		t.Fatal(err)
	}
	replacement.EntryMember, _ = view.Member([32]byte{2}, view.ObservedAt())
	replacement.InteriorMember, _ = view.Member([32]byte{4}, view.ObservedAt())
	for slot, want := range original {
		if got, err := owner.DutyForLeg(replacement, uint8(slot), nil); err != nil || got != want {
			t.Fatal("eligible replacement lost or redrew the context choice", err)
		}
	}
	if !owner.NotAfter().Equal(until) {
		t.Fatal("physical reopen reset the original context deadline")
	}
	if _, err := owner.Duty(0, nil); err == nil {
		t.Fatal("default Duty silently rebound its original physical leg")
	}
	view, replacement, _ = rendezvousModelAt(t, now, until, nil)
	if _, err := owner.DutyForLeg(replacement, 0, nil); err == nil {
		t.Fatal("fresh physical leg renewed an expired context choice")
	}
	if !owner.NotAfter().Equal(until) {
		t.Fatal("expiry refusal changed the original context deadline")
	}
}

func TestRendezvousReplacementKeepsLearnedConflicts(t *testing.T) {
	for _, location := range []string{"retained leg", "local exclusion"} {
		for _, identity := range []string{"Node", "key", "family"} {
			t.Run(location+"/"+identity, func(t *testing.T) {
				now := time.Unix(1900000000, 0).UTC()
				view, source, _ := rendezvousModel(t, now, nil)
				owner, err := NewRendezvous(source, func() (network.RuntimeView, error) { return view, nil }, nil)
				if err != nil {
					t.Fatal(err)
				}
				first, err := owner.Duty(0, nil)
				if err != nil {
					t.Fatal(err)
				}
				alternative, err := owner.Duty(1, nil)
				if err != nil {
					t.Fatal(err)
				}
				until := owner.NotAfter()
				var peer route.Member
				switch identity {
				case "Node":
					peer.NodeID = first.NodeID
				case "key":
					peer.PublicKey = first.PublicKey
				case "family":
					peer.FamilyID = first.FamilyID
				}
				peers := []route.Member{peer}
				replacement := source
				var excluded []route.Member
				if location == "retained leg" {
					replacement.known = peers
				} else {
					excluded = peers
				}
				if _, err := owner.DutyForLeg(replacement, 0, excluded); err == nil {
					t.Fatal("replacement accepted a newly known conflict")
				}
				clear(peers)
				if _, err := owner.Duty(0, nil); err == nil {
					t.Fatal("later call forgot a conflict learned during refusal")
				}
				if got, err := owner.DutyForLeg(source, 1, nil); err != nil || got != alternative {
					t.Fatal("conflict discarded or changed the already retained alternative", err)
				}
				if !owner.NotAfter().Equal(until) {
					t.Fatal("conflict reset context validity")
				}
			})
		}
	}
}

func TestRendezvousReplacementChecksItsOwnAuthorityAndOriginalProfile(t *testing.T) {
	for _, loss := range []string{"observation", "Entry", "Interior", "leg expiry", "role", "profile"} {
		t.Run(loss, func(t *testing.T) {
			now := time.Unix(1900000000, 0).UTC()
			view, source, _ := rendezvousModel(t, now, nil)
			var observationErr error
			owner, err := NewRendezvous(source, func() (network.RuntimeView, error) { return view, observationErr }, nil)
			if err != nil {
				t.Fatal(err)
			}
			first, err := owner.Duty(0, nil)
			if err != nil {
				t.Fatal(err)
			}
			until := owner.NotAfter()
			replacement := source
			replacement.Entry, _ = view.RetainDuty([32]byte{2}, now)
			replacement.Interior, _ = view.RetainDuty([32]byte{4}, now)
			replacement.EntryMember, _ = view.Member([32]byte{2}, now)
			replacement.InteriorMember, _ = view.Member([32]byte{4}, now)
			var fresh, responder Leg
			view, fresh, responder = rendezvousModelAt(t, now, now.Add(10*time.Second), func(profile *network.ProfileFacts, records []network.NodeRecord, assignments []network.Assignment) {
				index := -1
				switch loss {
				case "observation":
					observationErr = errors.New("current observation unavailable")
				case "Entry":
					index = 1
				case "Interior":
					index = 3
				case "profile":
					profile.Digest = [32]byte{200}
				}
				if index >= 0 {
					records[index].Generation++
					assignments[index].Generation++
				}
			})
			switch loss {
			case "leg expiry":
				replacement.NotAfter = view.ObservedAt()
			case "role":
				replacement = responder
			case "profile":
				replacement = fresh
			}
			if _, err := owner.DutyForLeg(replacement, 0, nil); err == nil {
				t.Fatal("unavailable replacement or different context profile accepted")
			}
			observationErr = nil
			// The constructor leg is still current in this observation. Its
			// availability cannot replace the requested physical leg's check.
			view, replacement, _ = rendezvousModelAt(t, now, now.Add(11*time.Second), nil)
			if got, err := owner.DutyForLeg(replacement, 0, nil); err != nil || got != first {
				t.Fatal("refusal discarded or replaced the retained choice", err)
			}
			if !owner.NotAfter().Equal(until) {
				t.Fatal("authority loss reset the retained deadline")
			}
		})
	}
}

func TestRendezvousFailedReplacementKeepsObservationFloor(t *testing.T) {
	now := time.Unix(1900000000, 0).UTC()
	view, source, _ := rendezvousModel(t, now, nil)
	owner, err := NewRendezvous(source, func() (network.RuntimeView, error) { return view, nil }, nil)
	if err != nil {
		t.Fatal(err)
	}
	first, err := owner.Duty(0, nil)
	if err != nil {
		t.Fatal(err)
	}
	view, _, _ = rendezvousModelAt(t, now, now.Add(10*time.Second), nil)
	expired := source
	expired.NotAfter = view.ObservedAt()
	if _, err := owner.DutyForLeg(expired, 0, nil); err == nil {
		t.Fatal("expired physical leg accepted")
	}
	view, _, _ = rendezvousModelAt(t, now, now.Add(5*time.Second), nil)
	if _, err := owner.DutyForLeg(source, 0, nil); err == nil {
		t.Fatal("failed physical leg lost the context observation floor")
	}
	view, _, _ = rendezvousModelAt(t, now, now.Add(11*time.Second), nil)
	if got, err := owner.DutyForLeg(source, 0, nil); err != nil || got != first {
		t.Fatal("clock refusal changed the retained choice", err)
	}
}

// This is a supplied-fact Network model fixture for local Route rules, not
// authenticated State or integrated transport evidence. The command suite
// independently exercises signed input and the genuine CurrentRuntime owner.
func rendezvousModel(t *testing.T, now time.Time, change func(*network.ProfileFacts, []network.NodeRecord, []network.Assignment)) (network.RuntimeView, Leg, Leg) {
	t.Helper()
	return rendezvousModelAt(t, now, now, change)
}

func rendezvousModelAt(t *testing.T, now, observed time.Time, change func(*network.ProfileFacts, []network.NodeRecord, []network.Assignment)) (network.RuntimeView, Leg, Leg) {
	t.Helper()
	binding := network.ProfileBinding{Network: [32]byte{101}, Generation: [32]byte{102}, EpochDigest: [32]byte{103}, Digest: [32]byte{104}, Epoch: 1, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour)}
	profile := network.ProfileFacts{ProfileBinding: binding, IssuanceAuthorityKey: [32]byte{105}, IssuerNodeID: [32]byte{12}, IssuerDutyGeneration: 1, TokenKeys: []network.TokenKey{{WindowStart: now.Truncate(time.Hour), Class: 2, SPKI: [346]byte{1}}}}
	var records []network.NodeRecord
	var assignments []network.Assignment
	for id := byte(1); id <= 12; id++ {
		domain, subrole, name := uint8(1), uint8(1), "initiator"
		if id >= 5 {
			domain, name = 3, "responder"
		}
		if id >= 9 {
			domain, subrole, name = 2, 4, "rendezvous"
		} else if id == 3 || id == 4 || id == 7 || id == 8 {
			subrole = 2
		}
		if id == 12 {
			subrole = 6
		}
		record := network.NodeRecord{NodeID: [32]byte{id}, RecordDigest: [32]byte{id + 60}, PublicKey: [32]byte{id + 20}, FamilyID: [32]byte{id + 40}, Generation: 1, Assignment: name, CarrierProfile: "ardents-carrier-tcp-tls-v2", Endpoint: "127.0.0.1:2000", Capacity: 1, ValidFrom: binding.NotBefore, ValidUntil: binding.NotAfter, AssignmentNotAfter: binding.NotAfter}
		records = append(records, record)
		assignments = append(assignments, network.Assignment{NodeID: record.NodeID, RecordDigest: record.RecordDigest, Generation: 1, RoleDomain: domain, Subrole: subrole})
	}
	if change != nil {
		change(&profile, records, assignments)
	}
	members, err := network.BindMembership(profile.ProfileBinding, assignments, records)
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := network.BindAcceptedState(network.EpochFacts{Network: profile.Network, Number: profile.Epoch, Digest: profile.EpochDigest, ValidFrom: profile.NotBefore, ValidUntil: profile.NotAfter}, profile.Generation, profile, members)
	if err != nil {
		t.Fatal(err)
	}
	clock, err := network.ConfirmTime(network.ClockEvidence{Wall: observed, Independent: observed}, 0)
	if err != nil {
		t.Fatal(err)
	}
	view, err := accepted.Observe(clock)
	if err != nil {
		t.Fatal(err)
	}
	leg := func(first byte) Leg {
		entry, _ := view.RetainDuty([32]byte{first}, now)
		interior, _ := view.RetainDuty([32]byte{first + 2}, now)
		em, _ := view.Member(entry.NodeID, now)
		im, _ := view.Member(interior.NodeID, now)
		var known []route.Member
		for id := first; id < first+4; id++ {
			m, _ := view.Member([32]byte{id}, now)
			known = append(known, route.Member{NodeID: m.NodeID, PublicKey: m.PublicKey, FamilyID: m.FamilyID})
		}
		return Leg{Entry: entry, Interior: interior, EntryMember: em, InteriorMember: im, Profile: view.Profile().ProfileBinding, NotAfter: profile.NotAfter, known: known}
	}
	return view, leg(1), leg(5)
}
