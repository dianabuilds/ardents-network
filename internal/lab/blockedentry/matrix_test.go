package blockedentry

import (
	"strings"
	"testing"
)

func TestHostileMatrixFixesNineFiveEpisodeGroups(t *testing.T) {
	groups := hostileMatrix()
	if len(groups) != 9 {
		t.Fatalf("hostile groups=%d", len(groups))
	}
	seen, events := make(map[string]bool), 0
	for _, group := range groups {
		if group.ID == "" || len(group.Variants) == 0 || seen[group.ID] {
			t.Fatalf("invalid hostile group: %+v", group)
		}
		seen[group.ID] = true
		for _, variant := range group.Variants {
			for episode := range 5 {
				if id := eventID(group.ID, variant, episode); seen[id] {
					t.Fatalf("duplicate hostile event: %s", id)
				} else {
					seen[id] = true
					events++
				}
			}
		}
	}
	if events != 450 {
		t.Fatalf("hostile event floor=%d want=450", events)
	}
}

func TestHostileTerminalContractUsesAcceptedOwnerAndRouteVocabulary(t *testing.T) {
	cases := map[string]string{
		"G1-invite/malformed":                             "invalid",
		"G1-invite/wrong-network":                         "incompatible",
		"G1-invite/expired":                               "expired",
		"G2-domain-collision/unknown-domain":              "wrong-domain",
		"G2-domain-collision/responder":                   "wrong-domain",
		"G3-replay-replacement/active-reimport":           "already-present",
		"G3-replay-replacement/retired-replay":            "replay",
		"G3-replay-replacement/full-set":                  "set-full",
		"G3-replay-replacement/skipped-generation":        "replacement-rejected",
		"G4-restart/after-import":                         "success",
		"G4-restart/after-exposure-0":                     "bridge-interrupted",
		"G4-restart/after-terminal-record":                "opened",
		"G5-adapter-fault/malformed-pt-control":           "bridge-attempt-exhausted",
		"G5-adapter-fault/evidence-write-exhaustion":      "bridge-local-denial",
		"G6-substitution/network":                         "incompatible",
		"G6-substitution/target":                          "bridge-local-denial",
		"G7-forbidden-path/deadline-exposure-reset":       "bridge-deadline-exceeded",
		"G7-forbidden-path/dns":                           "bridge-attempt-exhausted",
		"G8-lifecycle/cancellation":                       "bridge-deadline-exceeded",
		"G8-lifecycle/expiry-revocation":                  "bridge-ineligible",
		"G8-lifecycle/endpoint-restart":                   "bridge-interrupted",
		"G8-lifecycle/collector-loss":                     "",
		"G9-ledger-leakage/unknown-invite-field":          "invalid",
		"G9-ledger-leakage/candidate-leak-invite":         "bridge-local-denial",
		"G9-ledger-leakage/pipeline-contamination-invite": "",
	}
	for identity, want := range cases {
		parts := strings.Split(identity, "/")
		if got := expectedTerminal(parts[0], parts[1]); got != want {
			t.Fatalf("terminal %s=%s want=%s", identity, got, want)
		}
	}
}

func TestEveryHostileTerminalIsInAcceptedVocabulary(t *testing.T) {
	allowed := acceptedTerminals()
	for _, group := range hostileMatrix() {
		for _, variant := range group.Variants {
			terminal := expectedTerminal(group.ID, variant)
			if terminal == "" && evidenceOnlyVariant(group.ID, variant) {
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

func evidenceOnlyVariant(group, variant string) bool {
	return group == "G8-lifecycle" && variantIn(variant, "collector-loss", "blocker-loss") ||
		group == "G9-ledger-leakage" && variantIn(variant, "pipeline-contamination-invite",
			"pipeline-contamination-address", "pipeline-contamination-path",
			"pipeline-contamination-certificate")
}

func acceptedTerminals() map[string]bool {
	return map[string]bool{
		"accepted": true, "success": true, "opened": true, "already-present": true, "invalid": true, "incompatible": true,
		"wrong-domain": true, "conflicting-role": true, "set-full": true,
		"replacement-rejected": true, "expired": true, "replay": true,
		"bridge-not-configured": true, "bridge-ineligible": true,
		"bridge-attempt-exhausted": true, "bridge-deadline-exceeded": true,
		"bridge-interrupted": true, "bridge-local-denial": true,
	}
}
