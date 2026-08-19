package blockedverify

import "strconv"

var fixtureModes = map[string]bool{
	"pass": true, "candidate-fail": true, "harness-invalid": true, "candidate-canary": true,
	"pipeline-canary": true, "candidate-residual": true, "candidate-forbidden": true,
	"cell-inventory-missing": true, "inventory-missing": true, "candidate-fail-harness-invalid": true,
	"collector-loss": true, "blocker-loss": true,
	"forbidden-owner-mismatch": true,
	"candidate-canary-invite":  true, "candidate-canary-address": true,
	"candidate-canary-path": true, "candidate-canary-certificate": true,
	"pipeline-canary-invite": true, "pipeline-canary-address": true,
	"pipeline-canary-path": true, "pipeline-canary-certificate": true,
}

type hostileGroup struct {
	ID       string
	Variants []string
}

func hostileMatrix() []hostileGroup {
	return []hostileGroup{
		{"G1-invite", []string{"malformed", "non-canonical", "oversized", "duplicate-field", "trailing-field",
			"wrong-signature", "wrong-network", "wrong-epoch", "wrong-profile", "expired", "not-yet-valid",
			"insufficient-time-confidence"}},
		{"G2-domain-collision", []string{"responder", "introduction", "rendezvous", "resolution", "unknown-domain",
			"conflicting-retained-family", "direct-source-exposure", "interior-live-route", "drain", "quarantine"}},
		{"G3-replay-replacement", []string{"active-reimport", "retired-replay", "same-generation-different-bytes",
			"skipped-generation", "wrong-replacement-id", "third-generation", "full-set", "cross-slot-replacement"}},
		{"G4-restart", []string{"after-import", "after-regime-publication", "after-exposure-0", "after-exposure-1",
			"after-exposure-2", "after-exposure-3", "after-adapter-start", "after-readiness", "after-useful-work-prefix",
			"after-terminal-record", "during-cleanup"}},
		{"G5-adapter-fault", []string{"slow-partial-handshake", "malformed-pt-control", "wrong-socks-listener-method",
			"child-exit", "sigterm", "sigkill", "accept-then-stall", "malformed-carriage", "evidence-write-exhaustion"}},
		{"G6-substitution", []string{"target", "instance-generation", "network", "route-profile", "isolation-context",
			"route-generation", "attachment", "application-canary"}},
		{"G7-forbidden-path", []string{"dns", "environment-proxy", "ordinary-entry", "direct-target", "alternate-address",
			"alternate-candidate", "shorter-route", "cached-success", "deadline-exposure-reset"}},
		{"G8-lifecycle", []string{"cancellation", "expiry-revocation", "endpoint-restart", "bridge-restart", "collector-loss",
			"blocker-loss", "clock-discontinuity", "residual-injection"}},
		{"G9-ledger-leakage", []string{"unknown-invite-field", "regime-oscillation", "slot1-before-slot0",
			"retry-before-initial", "duplicate-ordinal", "ledger-reset-restart", "ledger-reset-new-operation",
			"candidate-leak-invite", "candidate-leak-address", "candidate-leak-path", "candidate-leak-certificate",
			"pipeline-contamination-invite", "pipeline-contamination-address", "pipeline-contamination-path",
			"pipeline-contamination-certificate"}},
	}
}

func expectedEventIDs() map[string]eventExpectation {
	result := make(map[string]eventExpectation)
	for _, expectation := range expectedEventSequence() {
		result[expectation.id] = expectation
	}
	return result
}

type eventExpectation struct {
	id, group, variant, terminal string
	episode                      int
}

func expectedEventSequence() []eventExpectation {
	result := make([]eventExpectation, 0, 450)
	for _, group := range hostileMatrix() {
		for _, variant := range group.Variants {
			for episode := range 5 {
				id := group.ID + "/" + variant + "/" + strconv.Itoa(episode)
				result = append(result, eventExpectation{id: id, group: group.ID, variant: variant,
					episode: episode, terminal: expectedTerminal(group.ID, variant)})
			}
		}
	}
	return result
}

func expectedFinalEventSequence() []eventExpectation {
	result := make([]eventExpectation, 0, 420)
	for _, value := range expectedEventSequence() {
		if !finalEvidenceMutationVariant(value.group, value.variant) {
			result = append(result, value)
		}
	}
	return result
}

func finalEvidenceMutationVariant(group, variant string) bool {
	return group == "G8-lifecycle" && variantIn(variant, "collector-loss", "blocker-loss") ||
		group == "G9-ledger-leakage" && variantIn(variant, "pipeline-contamination-invite",
			"pipeline-contamination-address", "pipeline-contamination-path",
			"pipeline-contamination-certificate")
}

func expectedTerminal(group, variant string) string {
	switch group {
	case "G1-invite":
		if variantIn(variant, "wrong-network", "wrong-epoch", "wrong-profile", "not-yet-valid",
			"insufficient-time-confidence") {
			return "incompatible"
		}
		if variant == "expired" {
			return "expired"
		}
		return "invalid"
	case "G2-domain-collision":
		if variantIn(variant, "responder", "introduction", "rendezvous", "resolution", "unknown-domain") {
			return "wrong-domain"
		}
		return "conflicting-role"
	case "G3-replay-replacement":
		switch variant {
		case "active-reimport":
			return "already-present"
		case "retired-replay", "same-generation-different-bytes":
			return "replay"
		case "full-set":
			return "set-full"
		default:
			return "replacement-rejected"
		}
	case "G4-restart":
		if variant == "after-import" {
			return "success"
		}
		if variant == "after-terminal-record" {
			return "opened"
		}
		return "bridge-interrupted"
	case "G5-adapter-fault":
		if variant == "evidence-write-exhaustion" {
			return "bridge-local-denial"
		}
		return "bridge-attempt-exhausted"
	case "G6-substitution":
		if variantIn(variant, "network", "route-profile") {
			return "incompatible"
		}
		return "bridge-local-denial"
	case "G7-forbidden-path":
		if variant == "deadline-exposure-reset" {
			return "bridge-deadline-exceeded"
		}
		return "bridge-attempt-exhausted"
	case "G8-lifecycle":
		switch variant {
		case "collector-loss", "blocker-loss":
			return ""
		case "cancellation":
			return "bridge-deadline-exceeded"
		case "expiry-revocation", "clock-discontinuity":
			return "bridge-ineligible"
		case "endpoint-restart", "bridge-restart":
			return "bridge-interrupted"
		default:
			return "bridge-local-denial"
		}
	case "G9-ledger-leakage":
		if variant == "unknown-invite-field" {
			return "invalid"
		}
		if variantIn(variant, "pipeline-contamination-invite", "pipeline-contamination-address",
			"pipeline-contamination-path", "pipeline-contamination-certificate") {
			return ""
		}
		return "bridge-local-denial"
	default:
		return ""
	}
}

func variantIn(value string, wanted ...string) bool {
	for _, candidate := range wanted {
		if value == candidate {
			return true
		}
	}
	return false
}
