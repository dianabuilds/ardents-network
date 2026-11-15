package introduction

import (
	"encoding/hex"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/completion"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
)

// Only the original first invocation is admitted here. Reserved future attempt
// paths are not restart permission or an accepted invocation chain.
type retirementNumber uint32

const retirementLimit retirementNumber = 1

func (n retirementNumber) retirement(root string) string {
	if n != 0 {
		return ""
	}
	return filepath.Join(root, "introduction-retirement-00000000")
}

func (n retirementNumber) retirementJournal(root string) string {
	if n != 0 {
		return ""
	}
	return root + ".introduction-retirement-00000000-journal"
}

func introductionCleanExit(actual introductionExecution, original startArchive) bool {
	return original.PID != 0 && original.StartedRealtime != 0 && original.StartedMonotonic != 0 &&
		actual.pid == original.PID && actual.startedRealtime == original.StartedRealtime && actual.startedMonotonic == original.StartedMonotonic &&
		actual.exitedRealtime >= actual.startedRealtime && actual.exitedMonotonic >= actual.startedMonotonic && actual.code == 1 && actual.status == 0
}

// Matching native exit is necessary after the original kernel join. It cannot
// substitute for Introduction's challenge-bound domain report or original pidfd.
func verifyIntroductionExited(unit, service systemd.Properties, r Request, generation string, archive startArchive, id [16]byte) (introductionExecution, error) {
	if id == [16]byte{} || archive.InvocationID != hex.EncodeToString(id[:]) || archive.Generation != generation {
		return introductionExecution{}, ErrBinding
	}
	if err := verifyIntroductionConfiguration(unit, service, r, generation); err != nil {
		return introductionExecution{}, err
	}
	for _, field := range []struct {
		name, signature string
		value           any
	}{
		{"ActiveState", "s", "inactive"}, {"SubState", "s", "dead"}, {"TriggeredBy", "as", []string{}},
	} {
		if !systemd.Matches(unit, field.name, field.signature, field.value) {
			return introductionExecution{}, ErrBinding
		}
	}
	if !systemd.Matches(unit, "InvocationID", "ay", id) && !systemd.Matches(unit, "InvocationID", "ay", []any{}) {
		return introductionExecution{}, ErrBinding
	}
	for _, field := range []struct {
		name, signature string
		value           any
	}{
		{"MainPID", "u", uint32(0)}, {"ControlPID", "u", uint32(0)}, {"NRestarts", "u", uint32(0)},
		{"DynamicUser", "b", false}, {"StandardInput", "s", "null"},
	} {
		if !systemd.Matches(service, field.name, field.signature, field.value) {
			return introductionExecution{}, ErrBinding
		}
	}
	if !systemd.Matches(service, "ControlGroup", "s", "") && !systemd.Matches(service, "ControlGroup", "s", "/system.slice/ardents-introduction.service") {
		return introductionExecution{}, ErrBinding
	}
	actual, err := introductionExecutionStatus(service)
	if err != nil || !introductionCleanExit(actual, archive) {
		return introductionExecution{}, ErrBinding
	}
	return actual, nil
}

func decodeRetirementResult(raw []byte, number retirementNumber, b binding, bindingSHA, startSHA string, original startArchive, id [16]byte) (retirementResult, error) {
	var r retirementResult
	if number >= retirementLimit || !validStartDigest(bindingSHA) || !validStartDigest(startSHA) || !canonicalStartRecord(raw, &r) {
		return r, ErrBinding
	}
	if r.Schema != "ardents-introduction-retirement-v1" || r.Attempt != uint32(number) || r.Generation != b.Generation || r.Generation != original.Generation ||
		r.BindingSHA256 != bindingSHA || r.BindingSHA256 != original.BindingSHA256 || r.StartSHA256 != startSHA || r.PID != original.PID ||
		id == [16]byte{} || r.InvocationID != hex.EncodeToString(id[:]) || r.InvocationID != original.InvocationID || r.Manager == "" || r.Manager != original.Manager || !introductionCleanExit(r.execution(), original) {
		return retirementResult{}, ErrBinding
	}
	challenge, err := retirementChallenge(r.Challenge)
	if err != nil {
		return retirementResult{}, err
	}
	start, err := completion.Encode(id, b.Generation, bindingSHA)
	if err != nil {
		return retirementResult{}, err
	}
	frame := retirementFrame(start, challenge)
	if r.CompletionSHA256 != digest(frame[:]) {
		return retirementResult{}, ErrBinding
	}
	return r, nil
}
