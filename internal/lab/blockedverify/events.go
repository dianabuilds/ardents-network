package blockedverify

import "strings"

func verifyEvents(events []event, canaryCommitments map[string]string, attributions map[string]attributionFact,
	encodedCanaries []string, finalCampaign bool,
) (
	invalid, failures []string, candidateCanaries map[string]bool,
) {
	expected := expectedEventSequence()
	if finalCampaign {
		expected = expectedFinalEventSequence()
	}
	candidateCanaries = make(map[string]bool)
	if len(events) != len(expected) {
		invalid = append(invalid, "mandatory five-episode hostile matrix is incomplete")
	}
	for index, observed := range events {
		if index >= len(expected) {
			invalid = append(invalid, "event sequence contains an unexpected trailing identity")
			continue
		}
		want := expected[index]
		if observed.ID != want.id || observed.Group != want.group || observed.Variant != want.variant ||
			observed.Episode != want.episode || observed.ExpectedTerminal != want.terminal {
			invalid = append(invalid, "event sequence contains a missing, reordered, replayed, or mismatched identity")
			continue
		}
		if observed.CanarySetHash != canaryCommitments[observed.Variant] {
			invalid = append(invalid, "event canary-set binding is invalid: "+observed.ID)
		}
		if observed.TerminalOffsetMillis < observed.StartedOffsetMillis ||
			observed.TerminalOffsetMillis-observed.StartedOffsetMillis > 15_000 ||
			observed.CleanupOffsetMillis < observed.TerminalOffsetMillis ||
			observed.CleanupOffsetMillis-observed.TerminalOffsetMillis > 15_000 ||
			observed.AdapterCleanupMillis > 6_000 {
			invalid = append(invalid, "event monotonic offsets or cleanup bound are invalid: "+observed.ID)
		}
		if !observed.EvidenceTrustworthy {
			invalid = append(invalid, "event evidence is explicitly untrustworthy: "+observed.ID)
			continue
		}
		attribution, attributed := attributions[observed.ID]
		if !attributed || observed.FaultOwner != attribution.owner ||
			observed.AttributionEvidence != attribution.commitment {
			invalid = append(invalid, "event fault owner is not proven by its committed attribution: "+observed.ID)
			continue
		}
		if attribution.owner == "candidate" && observed.Diagnostic != "" {
			for _, canary := range encodedCanaries {
				if strings.Contains(observed.Diagnostic, canary) {
					candidateCanaries[canary] = true
				}
			}
		}
		failed := !observed.GatePassed || observed.ObservedTerminal != want.terminal
		if failed && attribution.owner == "candidate" {
			failures = append(failures, "candidate gate failure: "+observed.ID)
		} else if failed {
			invalid = append(invalid, "unattributed or harness-owned event failure: "+observed.ID)
		}
	}
	return invalid, failures, candidateCanaries
}
