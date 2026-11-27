package command

import (
	"encoding/json"
	"io"

	noderuntime "github.com/dianabuilds/ardents-network/internal/domain/node/runtime"
)

// forwardingOperation identifies the Forwarding report and its established compatibility rules.
const forwardingOperation = "node.forwarding"

// writeEvent encodes the bounded CLI event for an original Node operation.
func writeEvent(out io.Writer, operation, phase, outcome, first, cleanup, failure string) error {
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
func failureDiagnostic(fact *noderuntime.FailureDiagnostic) string {
	if fact == nil {
		return ""
	}
	raw, err := json.Marshal(fact)
	if err != nil {
		return `{"cause":"diagnostic-unavailable"}`
	}
	return string(raw)
}

// reportCompletion projects a physically joined invocation onto the CLI report and exit status.
func reportCompletion(diagnostic io.Writer, operation string, result noderuntime.InvocationCompletion, firstFailure func() *noderuntime.FailureDiagnostic) int {
	forwarding := operation == forwardingOperation
	// Forwarding's established diagnostic contract suppresses a final event
	// on a timed-out but otherwise clean retirement. The other roles report
	// the retained wait failure even when the original close later succeeds.
	if result.Outcome != "canceled" || result.AuthorityReleaseError != nil || result.TimedOut && !forwarding {
		if result.AuthorityReleaseError != nil {
			result.CleanupOwner = "network"
		}
		_ = writeEvent(diagnostic, operation, "joined", "failed", result.FirstOwner, result.CleanupOwner,
			failureDiagnostic(firstFailure()))
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
	if writeEvent(diagnostic, operation, "joined", "canceled", "", "", "") != nil {
		return 2
	}
	return 130
}

// startupRefusal reports the original startup and State release outcomes.
func startupRefusal(out io.Writer, operation string, cause, cleanup error) int {
	cleanupOwner := ""
	if cleanup != nil {
		cleanupOwner = "network"
	}
	if err := writeEvent(out, operation, "refused", "startup-unavailable", "", cleanupOwner,
		failureDiagnostic(noderuntime.StartupFailure(cause))); err != nil {
		return 2
	}
	return 1
}

// reportWaitExpired reports unavailability while the original invocation still joins.
func reportWaitExpired(diagnostic io.Writer, operation string, firstFailure func() *noderuntime.FailureDiagnostic) {
	detail := ""
	if operation == forwardingOperation {
		detail = failureDiagnostic(firstFailure())
	}
	_ = writeEvent(diagnostic, operation, "retiring", "completion-unavailable", "", "", detail)
}
