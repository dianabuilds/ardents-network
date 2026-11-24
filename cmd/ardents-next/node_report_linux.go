package main

import (
	"encoding/json"
	"io"

	"github.com/dianabuilds/ardents-network/internal/node"
)

func writeNodeEvent(out io.Writer, operation, phase, outcome, first, cleanup, failure string) error {
	return json.NewEncoder(out).Encode(struct {
		FirstFailure string `json:"first_failure,omitempty"`
		Operation    string `json:"operation"`
		Phase        string `json:"phase"`
		Outcome      string `json:"outcome"`
		First        string `json:"first_owner,omitempty"`
		Cleanup      string `json:"cleanup_owner,omitempty"`
	}{failure, operation, phase, outcome, first, cleanup})
}

// Preserve the original one-object/string-field role report. The encoded local
// fact contains no raw error text or private role binding and grants no authority.
func encodeNodeFailureDiagnostic(fact *node.FailureDiagnostic) string {
	if fact == nil {
		return ""
	}
	raw, err := json.Marshal(fact)
	if err != nil {
		return `{"cause":"diagnostic-unavailable"}`
	}
	return string(raw)
}

func reportNodeCompletion(diagnostic io.Writer, operation string, forwarding bool, result node.InvocationCompletion, firstFailure func() *node.FailureDiagnostic) int {
	// Forwarding's established diagnostic contract suppresses a final event
	// on a timed-out but otherwise clean retirement. The other roles report
	// the retained wait failure even when the original close later succeeds.
	if result.Outcome != "canceled" || result.AuthorityReleaseError != nil || result.TimedOut && !forwarding {
		if result.AuthorityReleaseError != nil {
			result.CleanupOwner = "network"
		}
		_ = writeNodeEvent(diagnostic, operation, "joined", "failed", result.FirstOwner, result.CleanupOwner,
			encodeNodeFailureDiagnostic(firstFailure()))
		if result.AnnouncementError != nil && (!forwarding || !result.TimedOut) {
			return 2
		}
		return 1
	}
	if result.TimedOut {
		return 1
	}
	if result.AnnouncementError != nil {
		return 2
	}
	if writeNodeEvent(diagnostic, operation, "joined", "canceled", "", "", "") != nil {
		return 2
	}
	return 130
}

func nodeStartupRefusal(out io.Writer, operation string, cause, cleanup error) int {
	cleanupOwner := ""
	if cleanup != nil {
		cleanupOwner = "network"
	}
	if err := writeNodeEvent(out, operation, "refused", "startup-unavailable", "", cleanupOwner,
		encodeNodeFailureDiagnostic(node.StartupFailure(cause))); err != nil {
		return 2
	}
	return 1
}
