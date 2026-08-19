package blockedentry

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
