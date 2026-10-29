package forwarding

import (
	"encoding/hex"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/completion"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
)

// Native exit facts are compared only after the original scope has joined.
// This pure comparison supplies neither that join nor the command's domain
// completion. The stop owner must establish both independently.
func verifyForwardingExited(unit, service systemd.Properties, r Request, generation string, original startArchive, id [16]byte) (forwardingExecution, error) {
	if original.PID == 0 || id == [16]byte{} || original.InvocationID != hex.EncodeToString(id[:]) || original.Generation != generation {
		return forwardingExecution{}, ErrBinding
	}
	if err := verifyForwardingConfiguration(unit, service, r, generation); err != nil {
		return forwardingExecution{}, err
	}
	if !systemd.Matches(unit, "ActiveState", "s", "inactive") || !systemd.Matches(unit, "SubState", "s", "dead") ||
		(!systemd.Matches(unit, "InvocationID", "ay", id) && !systemd.Matches(unit, "InvocationID", "ay", []any{})) ||
		!systemd.Matches(unit, "TriggeredBy", "as", []string{}) || !systemd.Matches(service, "MainPID", "u", uint32(0)) ||
		!systemd.Matches(service, "ControlPID", "u", uint32(0)) || !systemd.Matches(service, "NRestarts", "u", uint32(0)) ||
		!systemd.Matches(service, "DynamicUser", "b", false) || !systemd.Matches(service, "StandardInput", "s", "null") ||
		(!systemd.Matches(service, "ControlGroup", "s", "") && !systemd.Matches(service, "ControlGroup", "s", "/system.slice/ardents-forwarding.service")) {
		return forwardingExecution{}, ErrBinding
	}
	status, err := forwardingExecutionStatus(service)
	if err != nil || !forwardingCleanExit(status, original) {
		return forwardingExecution{}, ErrBinding
	}
	return status, nil
}

func forwardingCleanExit(status forwardingExecution, original startArchive) bool {
	return original.PID != 0 && original.StartedRealtime != 0 && original.StartedMonotonic != 0 &&
		status.pid == original.PID && status.startedRealtime == original.StartedRealtime && status.startedMonotonic == original.StartedMonotonic &&
		status.exitedRealtime >= status.startedRealtime && status.exitedMonotonic >= status.startedMonotonic &&
		status.code == 1 && status.status == 0 // CLD_EXITED; a signal is not a clean command result
}

// A newly loaded collected unit has lost its old execution properties. Only a
// consumer retaining independently committed retirement may use this comparison;
// zero native properties themselves never supply that missing proof.
func verifyForwardingRestartStopped(unit, service systemd.Properties, r Request, generation string, original startArchive, id [16]byte, retired forwardingExecution) error {
	if !forwardingCleanExit(retired, original) || id == [16]byte{} || original.InvocationID != hex.EncodeToString(id[:]) || original.Generation != generation {
		return ErrBinding
	}
	if verifyForwardingStopped(unit, service, r, generation) == nil {
		if !systemd.Matches(unit, "TriggeredBy", "as", []string{}) || !systemd.Matches(service, "ControlPID", "u", uint32(0)) || !systemd.Matches(service, "NRestarts", "u", uint32(0)) || !systemd.Matches(service, "DynamicUser", "b", false) || !systemd.Matches(service, "StandardInput", "s", "null") {
			return ErrBinding
		}
		return nil
	}
	actual, err := verifyForwardingExited(unit, service, r, generation, original, id)
	if err != nil || actual != retired {
		return ErrBinding
	}
	return nil
}

// A replacement changes loaded configuration, never the archived execution.
// Only the complete original predecessor chain supplies retirement. This pure
// comparison additionally requires an exact newly loaded zero-execution unit;
// it cannot reinterpret old execution clocks as a run of the new generation.
func verifyForwardingReplacementStopped(unit, service systemd.Properties, selected generationSelection, previous startArchive, number attemptNumber, id [16]byte, retired retirementResult, retirementSHA string) error {
	r := selected.replacement
	if r == nil || selected.binding.Generation == previous.Generation || r.PredecessorGeneration != previous.Generation ||
		r.PredecessorBindingSHA256 != previous.BindingSHA256 || r.PredecessorAttempt != uint32(number) || r.RetirementSHA256 != retirementSHA ||
		retired.Generation != previous.Generation || retired.BindingSHA256 != previous.BindingSHA256 || retired.Attempt != uint32(number) ||
		retired.InvocationID != previous.InvocationID || retired.Manager != previous.Manager ||
		id == [16]byte{} || previous.InvocationID != hex.EncodeToString(id[:]) || !forwardingCleanExit(retired.execution(), previous) {
		return ErrBinding
	}
	if err := verifyForwardingStopped(unit, service, selected.binding.Request, selected.binding.Generation); err != nil {
		return err
	}
	if !systemd.Matches(unit, "TriggeredBy", "as", []string{}) || !systemd.Matches(service, "ControlPID", "u", uint32(0)) ||
		!systemd.Matches(service, "NRestarts", "u", uint32(0)) || !systemd.Matches(service, "DynamicUser", "b", false) || !systemd.Matches(service, "StandardInput", "s", "null") {
		return ErrBinding
	}
	return nil
}

// Decode only canonical detached result bytes. Even an exact result still needs
// the full independent native transcript, original inodes and absent blocker.
func decodeRetirementResult(raw []byte, number attemptNumber, b binding, bindingSHA, startSHA string, original startArchive, id [16]byte) (retirementResult, error) {
	var result retirementResult
	if number >= attemptLimit || !validStartDigest(bindingSHA) || !validStartDigest(startSHA) || !canonicalStartRecord(raw, &result) {
		return retirementResult{}, ErrBinding
	}
	if result.Schema != "ardents-forwarding-retirement-v1" || result.Attempt != uint32(number) ||
		result.Generation != b.Generation || result.Generation != original.Generation || result.BindingSHA256 != bindingSHA ||
		result.BindingSHA256 != original.BindingSHA256 || result.StartSHA256 != startSHA || result.PID != original.PID ||
		id == [16]byte{} || result.InvocationID != hex.EncodeToString(id[:]) || result.InvocationID != original.InvocationID ||
		result.Manager == "" || result.Manager != original.Manager || !forwardingCleanExit(result.execution(), original) {
		return retirementResult{}, ErrBinding
	}
	challenge, err := retirementChallenge(result.Challenge)
	if err != nil {
		return retirementResult{}, err
	}
	start, err := completion.Encode(id, b.Generation, bindingSHA)
	if err != nil {
		return retirementResult{}, err
	}
	frame := retirementFrame(start, challenge)
	if result.CompletionSHA256 != digest(frame[:]) {
		return retirementResult{}, ErrBinding
	}
	return result, nil
}
