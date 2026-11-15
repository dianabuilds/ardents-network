package join

import (
	"encoding/hex"
)

type retirementResult struct {
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

func (r retirementResult) execution() joinExecution {
	return joinExecution{r.StartedRealtime, r.StartedMonotonic, r.ExitedRealtime, r.ExitedMonotonic, r.PID, r.Code, r.Status}
}

func retirementChallenge(text string) (result [32]byte, err error) {
	raw, err := hex.DecodeString(text)
	if err != nil || len(raw) != len(result) || hex.EncodeToString(raw) != text {
		return result, ErrBinding
	}
	copy(result[:], raw)
	if result == [32]byte{} {
		return result, ErrBinding
	}
	return result, nil
}
