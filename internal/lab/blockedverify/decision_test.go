package blockedverify

import (
	"strings"
	"testing"
)

func TestEventDecisionDistinguishesCandidateFailureFromInvalidEvidence(t *testing.T) {
	events := completeEvents()
	commitments := testCanaryCommitments()
	attributions := testAttributions(events)
	invalid, failures, _ := verifyEvents(events, commitments, attributions, nil, false)
	if len(invalid) != 0 || len(failures) != 0 {
		t.Fatalf("complete events invalid=%v failures=%v", invalid, failures)
	}
	events[0].GatePassed = false
	events[0].FaultOwner = "candidate"
	attributions = testAttributions(events)
	invalid, failures, _ = verifyEvents(events, commitments, attributions, nil, false)
	if len(invalid) != 0 || len(failures) != 1 {
		t.Fatalf("candidate failure invalid=%v failures=%v", invalid, failures)
	}
	events[0].FaultOwner = "harness"
	invalid, failures, _ = verifyEvents(events, commitments, attributions, nil, false)
	if len(invalid) == 0 || len(failures) != 0 {
		t.Fatalf("harness failure invalid=%v failures=%v", invalid, failures)
	}
}

func TestEventDecisionRejectsClockAndCardinalityTamper(t *testing.T) {
	events := completeEvents()
	commitments := testCanaryCommitments()
	events[0].CleanupOffsetMillis = events[0].TerminalOffsetMillis + 15_001
	invalid, _, _ := verifyEvents(events, commitments, testAttributions(events), nil, false)
	if len(invalid) == 0 {
		t.Fatal("cleanup deadline tamper was accepted")
	}
	events = completeEvents()[1:]
	invalid, _, _ = verifyEvents(events, commitments, testAttributions(events), nil, false)
	if len(invalid) == 0 {
		t.Fatal("missing event was accepted")
	}
}

func TestEventDecisionRejectsEveryOrderMutation(t *testing.T) {
	for name, mutate := range map[string]func([]event){
		"swap": func(events []event) { events[0], events[1] = events[1], events[0] },
		"reverse": func(events []event) {
			for left, right := 0, len(events)-1; left < right; left, right = left+1, right-1 {
				events[left], events[right] = events[right], events[left]
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			events := completeEvents()
			mutate(events)
			invalid, _, _ := verifyEvents(events, testCanaryCommitments(), testAttributions(events), nil, false)
			if len(invalid) == 0 {
				t.Fatal("event order mutation was accepted")
			}
		})
	}
}

func TestObserverAndCleanupDecisionAreFailClosed(t *testing.T) {
	observers := make([]observer, 0, len(requiredBoundaries))
	for _, boundary := range requiredBoundaries {
		observers = append(observers, observer{Boundary: boundary, IPv4UDPControl: true,
			IPv6UDPControl: true, IPv4TCPControl: true, Attribution: "exact", ForbiddenOwner: "none",
			ObservationCompleted: true})
	}
	if invalid, failures := verifyObservers(observers); len(invalid) != 0 || len(failures) != 0 {
		t.Fatalf("complete observers invalid=%v failures=%v", invalid, failures)
	}
	observers[0].Attribution = "ambiguous"
	if invalid, _ := verifyObservers(observers); len(invalid) == 0 {
		t.Fatal("ambiguous observer was accepted")
	}
	observers[0].Attribution = "exact"
	observers[0].ForbiddenPackets = 1
	observers[0].ForbiddenOwner = "candidate"
	if invalid, failures := verifyObservers(observers); len(invalid) != 0 || len(failures) != 1 {
		t.Fatalf("forbidden packet invalid=%v failures=%v", invalid, failures)
	}
	observers[0].ForbiddenOwner = "harness"
	if invalid, failures := verifyObservers(observers); len(invalid) == 0 || len(failures) != 0 {
		t.Fatalf("harness packet invalid=%v failures=%v", invalid, failures)
	}
	cleanup := cleanupInventory{Complete: true}
	for _, kind := range requiredResiduals {
		cleanup.Items = append(cleanup.Items, residual{Kind: kind, Owner: "none"})
	}
	invalid, failures := verifyCleanup(cleanup, nil)
	if len(invalid) != 0 || len(failures) != 0 {
		t.Fatalf("complete cleanup invalid=%v failures=%v", invalid, failures)
	}
	cleanup.Items[0].Count, cleanup.Items[0].Owner = 1, "candidate"
	cleanup.Items[0].AttributionEvidence = "candidate-attribution"
	attributions := map[string]attributionFact{"event": {owner: "candidate", commitment: "candidate-attribution"}}
	invalid, failures = verifyCleanup(cleanup, attributions)
	if len(invalid) != 0 || len(failures) != 1 {
		t.Fatalf("candidate residue invalid=%v failures=%v", invalid, failures)
	}
}

func completeEvents() []event {
	result := make([]event, 0, len(expectedEventSequence()))
	commitments := testCanaryCommitments()
	for _, identity := range expectedEventSequence() {
		result = append(result, event{ID: identity.id, Group: identity.group, Variant: identity.variant,
			Episode: identity.episode, ExpectedTerminal: identity.terminal,
			ObservedTerminal: identity.terminal, GatePassed: true, EvidenceTrustworthy: true,
			FaultOwner: "none", TerminalOffsetMillis: 1, CleanupOffsetMillis: 2,
			CanarySetHash: commitments[identity.variant], AttributionEvidence: strings.Repeat("0", 64)})
	}
	return result
}

func testCanaryCommitments() map[string]string {
	result := make(map[string]string, 8)
	for _, variant := range secretVariants() {
		result[variant] = "commitment-" + variant
	}
	return result
}

func testAttributions(events []event) map[string]attributionFact {
	result := make(map[string]attributionFact, len(events))
	for _, value := range events {
		result[value.ID] = attributionFact{owner: value.FaultOwner, commitment: value.AttributionEvidence}
	}
	return result
}
