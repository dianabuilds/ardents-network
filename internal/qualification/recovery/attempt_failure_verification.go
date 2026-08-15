package recovery

import "time"

type replacementFailureObservation struct {
	Kind                               string
	EventIndex                         int
	ExpectedOffset, ObservedOffset     uint32
	LastDeliveryNanos, ObservedAtNanos int64
}

type replacementFailureEvidence struct {
	Failure replacementFailureObservation
	Cell    replacementCell
	Faults  map[string]processFaultEvidence
}

func verifyReplacementAttemptFailure(receipt replacementAttemptReceipt, cellManifest replacementAttemptCell,
	manifest replacementAttemptManifest, hostScope hostScopeEvidence,
	candidates map[string][]replacementCandidate) Result {
	var evidence replacementFailureEvidence
	if err := decodeAttemptValue(receipt.Evidence, 4<<20, &evidence); err != nil {
		return invalid("decode replacement failure evidence: " + err.Error())
	}
	cell, failure := evidence.Cell, evidence.Failure
	if cell.Direction != cellManifest.Direction || cell.Mode != cellManifest.Mode ||
		cell.CellManifestDigest != cellManifest.ManifestDigest || receipt.ActiveNanos != cell.TerminalNanos ||
		failure.ObservedAtNanos != cell.TerminalNanos || failure.EventIndex < 0 ||
		failure.EventIndex > len(cell.FaultOffsets) {
		return invalid("replacement failure differs from its immutable attempt manifest")
	}
	manifestCell := cell
	manifestCell.Events = make([]replacementEvent, len(cell.FaultOffsets))
	if result := verifyReplacementManifest(manifestCell); result.Verdict != "pass" {
		return result
	}
	if !validDockerReplacementFailureProcesses(cell, hostScope) ||
		!validReplacementFailurePrefix(cell, evidence.Faults, candidates, manifest.RouteCase, hostScope) {
		return invalid("replacement failure process prefix is invalid")
	}
	if err := verifyReplacementDockerTopology(manifest.Topology); err != nil {
		return invalid(err.Error())
	}
	expectedReason := ""
	switch failure.Kind {
	case "progress":
		expectedReason = "receiver did not drain to the exact replacement gate"
		if failure.EventIndex >= len(cell.FaultOffsets) ||
			failure.ExpectedOffset != cell.FaultOffsets[failure.EventIndex] ||
			failure.ObservedOffset >= failure.ExpectedOffset || failure.LastDeliveryNanos != 0 ||
			len(evidence.Faults) != len(cell.Events) {
			return invalid("replacement progress failure observation is invalid")
		}
	case "canary-missing", "canary-late":
		if failure.EventIndex >= len(cell.FaultOffsets) ||
			failure.ExpectedOffset != cell.FaultOffsets[failure.EventIndex]+32 ||
			failure.LastDeliveryNanos <= 0 || failure.ObservedAtNanos-failure.LastDeliveryNanos < int64(5*time.Second) ||
			len(evidence.Faults) != len(cell.Events)+1 || !validCurrentFailureFault(cell, failure, evidence.Faults) {
			return invalid("replacement canary failure observation is invalid")
		}
		if failure.Kind == "canary-missing" && failure.ObservedOffset < failure.ExpectedOffset {
			expectedReason = "replacement recovery canary was not observed within five seconds"
		} else if failure.Kind == "canary-late" {
			expectedReason = "replacement recovery missed five seconds"
		} else {
			return invalid("replacement canary failure kind differs from its observed fact")
		}
	case "lifetime":
		expectedReason = "replacement process exceeded its candidate lifetime"
		if failure.EventIndex != len(cell.FaultOffsets) || failure.ObservedAtNanos < cell.LifetimeNanos ||
			len(evidence.Faults) != len(cell.Events) {
			return invalid("replacement lifetime failure observation is invalid")
		}
	default:
		return invalid("replacement failure observation kind is invalid")
	}
	if receipt.Reason != expectedReason {
		return invalid("replacement failure reason differs from its observed fact")
	}
	return fail(expectedReason)
}

func failureHostEnd(raw []byte) int64 {
	var evidence replacementFailureEvidence
	if decodeAttemptValue(raw, 4<<20, &evidence) != nil {
		return 0
	}
	return evidence.Cell.ActiveStartedAtNanos + evidence.Cell.TerminalNanos
}
