package blockedverify

import (
	"strings"
	"testing"
)

func TestIndependentMatrixRequiresEveryHostileEpisode(t *testing.T) {
	expected := expectedEventIDs()
	if len(hostileMatrix()) != 9 || len(expected) != 450 {
		t.Fatalf("independent hostile contract groups=%d events=%d", len(hostileMatrix()), len(expected))
	}
	delete(expected, "G1-invite/malformed/0")
	if len(expected) != 449 {
		t.Fatal("hostile event identities are not independently addressable")
	}
}

func TestIndependentTerminalContractUsesAcceptedVocabulary(t *testing.T) {
	cases := map[string]string{
		"G1-invite/malformed":                             "invalid",
		"G2-domain-collision/unknown-domain":              "wrong-domain",
		"G3-replay-replacement/retired-replay":            "replay",
		"G4-restart/after-exposure-0":                     "bridge-interrupted",
		"G5-adapter-fault/malformed-pt-control":           "bridge-attempt-exhausted",
		"G6-substitution/target":                          "bridge-local-denial",
		"G7-forbidden-path/deadline-exposure-reset":       "bridge-deadline-exceeded",
		"G8-lifecycle/expiry-revocation":                  "bridge-ineligible",
		"G9-ledger-leakage/unknown-invite-field":          "invalid",
		"G9-ledger-leakage/pipeline-contamination-invite": "",
	}
	for identity, want := range cases {
		parts := strings.Split(identity, "/")
		if got := expectedTerminal(parts[0], parts[1]); got != want {
			t.Fatalf("terminal %s=%s want=%s", identity, got, want)
		}
	}

	allowed := independentAcceptedTerminals()
	for _, group := range hostileMatrix() {
		for _, variant := range group.Variants {
			terminal := expectedTerminal(group.ID, variant)
			if terminal == "" && independentEvidenceOnlyVariant(group.ID, variant) {
				continue
			}
			if !allowed[terminal] {
				t.Fatalf("unaccepted terminal %q for %s/%s", terminal, group.ID, variant)
			}
		}
	}
	if got := expectedTerminal("unknown", "unknown"); got != "" {
		t.Fatalf("unknown terminal=%q", got)
	}
}

func independentEvidenceOnlyVariant(group, variant string) bool {
	return group == "G8-lifecycle" && variantIn(variant, "collector-loss", "blocker-loss") ||
		group == "G9-ledger-leakage" && variantIn(variant, "pipeline-contamination-invite",
			"pipeline-contamination-address", "pipeline-contamination-path",
			"pipeline-contamination-certificate")
}

func independentAcceptedTerminals() map[string]bool {
	return map[string]bool{
		"accepted": true, "already-present": true, "invalid": true, "incompatible": true,
		"wrong-domain": true, "conflicting-role": true, "set-full": true,
		"replacement-rejected": true, "expired": true, "replay": true,
		"bridge-not-configured": true, "bridge-ineligible": true,
		"bridge-attempt-exhausted": true, "bridge-deadline-exceeded": true,
		"bridge-interrupted": true, "bridge-local-denial": true,
	}
}
