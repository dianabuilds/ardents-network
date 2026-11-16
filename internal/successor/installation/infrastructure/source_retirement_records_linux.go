package infrastructure

import (
	"github.com/dianabuilds/ardents-network/internal/successor/installation/journal"
)

// Only root's explicit-stop producer writes this original completion. The
// domain frame, exact archive, native exit and ordered physical-close journal
// are all required; none is independently a restart token.
type sourceRetirementResult struct {
	Schema           string `json:"schema"`
	Attempt          uint32 `json:"attempt"`
	Generation       string `json:"generation"`
	BindingSHA256    string `json:"binding_sha256"`
	StartSHA256      string `json:"start_sha256"`
	InvocationID     string `json:"invocation_id"`
	PID              uint32 `json:"pid"`
	Manager          string `json:"manager"`
	Challenge        string `json:"challenge"`
	CompletionSHA256 string `json:"completion_sha256"`
	StartedRealtime  uint64 `json:"started_realtime"`
	StartedMonotonic uint64 `json:"started_monotonic"`
	ExitedRealtime   uint64 `json:"exited_realtime"`
	ExitedMonotonic  uint64 `json:"exited_monotonic"`
	Code             int32  `json:"code"`
	Status           int32  `json:"status"`
}

func (r sourceRetirementResult) execution() sourceExecution {
	return sourceExecution{r.StartedRealtime, r.StartedMonotonic, r.ExitedRealtime, r.ExitedMonotonic, r.PID, r.Code, r.Status}
}

func decodeSourceRetirementRecords(raw []byte, directory, start, challenge, completed, result string, gid uint32) (map[string]sourceStartEntry, error) {
	records, err := journal.DecodeSourceRetirementRecords(raw, directory, start, challenge, completed, result, gid)
	if err != nil {
		return nil, ErrBinding
	}
	return records, nil
}
