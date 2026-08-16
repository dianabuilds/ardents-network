package blockedentry

import "strconv"

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

func eventID(group, variant string, episode int) string {
	return group + "/" + variant + "/" + strconv.Itoa(episode)
}
