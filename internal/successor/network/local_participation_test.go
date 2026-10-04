package network

import (
	"errors"
	"testing"
	"time"
)

func TestLocalParticipationRetainsLiveSourceAcrossOtherProducers(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	source := LocalRestriction{Identity: [32]byte{1}, Family: [32]byte{2}, Class: "direct-source", Phase: "live", NotAfter: now.Add(-time.Second)}
	records := []ParticipationFact{{Producer: [32]byte{3}, Restriction: source}}
	history, err := RestoreLocalParticipation(records)
	if err != nil {
		t.Fatal(err)
	}
	records[0].Restriction.Class = "ordinary-initiator"
	history.Records()[0].Restriction.Class = "ordinary-initiator"
	retained, err := history.Replace([32]byte{4}, nil, now.Add(time.Hour))
	if err != nil || !retained.Conflicts(source.Identity, [32]byte{}, now.Add(time.Hour)) || !retained.Conflicts([32]byte{}, source.Family, now.Add(time.Hour)) {
		t.Fatal("another producer or copied data pruned live Source")
	}
	node := source
	node.Class = "node-duty"
	node.NotAfter = now.Add(2 * time.Hour)
	if _, err := retained.Replace([32]byte{4}, []LocalRestriction{node}, now); !errors.Is(err, ErrParticipationConflict) {
		t.Fatal("live Source collision admitted")
	}
	released, err := retained.Replace([32]byte{3}, nil, now)
	if err != nil || released.Conflicts(source.Identity, source.Family, now) || len(released.Records()) != 0 {
		t.Fatal("producer release did not remove its own Source")
	}
}

func TestLocalParticipationExpiresTimeHeldFactsAndPreservesExemptions(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	node := LocalRestriction{Identity: [32]byte{1}, Family: [32]byte{2}, Class: "node-duty", Phase: "live", NotAfter: now}
	history, err := RestoreLocalParticipation([]ParticipationFact{{Producer: [32]byte{3}, Restriction: node}})
	if err != nil {
		t.Fatal(err)
	}
	if history.Conflicts(node.Identity, node.Family, now) {
		t.Fatal("exclusive expiry retained time-held conflict")
	}
	node.NotAfter = now.Add(time.Hour)
	next, err := history.Replace([32]byte{4}, []LocalRestriction{node}, now)
	if err != nil || len(next.Records()) != 1 {
		t.Fatal("expired predecessor was not pruned")
	}
	ordinary := node
	ordinary.Class = "ordinary-initiator"
	exempt, err := next.Replace([32]byte{5}, []LocalRestriction{ordinary}, now)
	if err != nil || len(exempt.Records()) != 2 {
		t.Fatal("ordinary Initiator lost its explicit local exemption")
	}
	if _, err := exempt.Replace([32]byte{6}, []LocalRestriction{node}, now); !errors.Is(err, ErrParticipationConflict) {
		t.Fatal("ordinary exemption spread to another Node")
	}
}

func TestLocalParticipationSourceExemptionBelongsToOneProducer(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	a := LocalRestriction{Identity: [32]byte{1}, Family: [32]byte{2}, Class: "direct-source", Phase: "live", NotAfter: now.Add(time.Hour)}
	b := a
	b.Identity = [32]byte{3}
	empty, err := RestoreLocalParticipation(nil)
	if err != nil {
		t.Fatal(err)
	}
	history, err := empty.Replace([32]byte{4}, []LocalRestriction{a, b}, now)
	if err != nil {
		t.Fatal("same producer Source pair refused")
	}
	b.Identity = [32]byte{5}
	if _, err := history.Replace([32]byte{6}, []LocalRestriction{b}, now); !errors.Is(err, ErrParticipationConflict) {
		t.Fatal("Source pair exemption spread across producers")
	}
	if _, err := empty.Replace([32]byte{4}, []LocalRestriction{a, a}, now); err == nil {
		t.Fatal("duplicate producer/identity/family accepted")
	}
}

func TestLocalParticipationReportsDistinctFiniteBounds(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	empty, err := RestoreLocalParticipation(nil)
	if err != nil {
		t.Fatal(err)
	}
	entries := make([]LocalRestriction, 65)
	for i := range entries {
		entries[i] = LocalRestriction{Identity: [32]byte{byte(i + 1)}, Family: [32]byte{byte(i + 1)}, Class: "direct-source", Phase: "live", NotAfter: now.Add(time.Hour)}
	}
	if _, err := empty.Replace([32]byte{1}, entries, now); !errors.Is(err, ErrParticipationRecordLimit) {
		t.Fatal("oversized producer replacement has wrong refusal")
	}
	first, err := empty.Replace([32]byte{1}, entries[:32], now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.Replace([32]byte{2}, entries[32:], now); !errors.Is(err, ErrSourceExposureExhausted) {
		t.Fatal("installation Source exhaustion has wrong refusal")
	}
	producers := make([]ParticipationFact, 17)
	for i := range producers {
		producers[i] = ParticipationFact{Producer: [32]byte{byte(i + 1)}, Restriction: entries[i]}
		producers[i].Restriction.Class = "ordinary-initiator"
	}
	if _, err := RestoreLocalParticipation(producers); !errors.Is(err, ErrParticipationProducerLimit) {
		t.Fatal("producer exhaustion has wrong refusal")
	}
}

func TestNodeParticipationCannotChooseExemptionOrExtendAssignment(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	facts := NodeParticipationFacts{Identity: [32]byte{1}, Family: "family", Assignment: "initiator", EpochUntil: now.Add(time.Hour), RecordUntil: now.Add(time.Minute)}
	restriction, err := facts.Restriction("live")
	if err != nil || restriction.Class != "node-duty" || !restriction.NotAfter.Equal(facts.RecordUntil) {
		t.Fatal("Node chose exemption or extended record lifetime")
	}
	facts.Assignment = "direct-source"
	restriction, err = facts.Restriction("live")
	if err != nil || restriction.Class == "direct-source" {
		t.Fatal("Node turned assignment into live Source")
	}
	if _, err := facts.Restriction("exposed"); err == nil {
		t.Fatal("Node chose exposure phase")
	}
	facts.Assignment = "rendezvous"
	facts.EpochUntil = now.Add(time.Second)
	restriction, err = facts.Restriction("prepared")
	if err != nil || restriction.Class != "route-rendezvous" || !restriction.NotAfter.Equal(facts.EpochUntil) {
		t.Fatal("Node classification or Epoch bound changed")
	}
}
