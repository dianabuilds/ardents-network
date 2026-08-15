package campaign

import (
	"context"
	"encoding/json"
	"time"
)

const (
	candidatePass   = "pass"
	candidateFail   = "fail"
	candidateNotRun = "not-run"

	observationComplete = "complete"
	observationInvalid  = "invalid"

	cleanupComplete = "complete"
	cleanupInvalid  = "invalid"
)

// CellInput identifies one immutable attempt and its receipt location.
type CellInput struct {
	CellID, AttemptID, ManifestDigest string
	ReceiptRoot                       string
}

// CellObservation is the candidate result at the exact observed terminal
// boundary. TerminalAt and the timestamp returned by Release share one host
// monotonic clock.
type CellObservation struct {
	Candidate  string
	Reason     string
	TerminalAt time.Time
}

// FrozenCell is the candidate result and bounded common evidence obtained
// after the active interval.
type FrozenCell struct {
	Candidate string
	Reason    string
	Evidence  json.RawMessage
}

// CellAdapter supplies one prepared qualification scenario at the execution
// seam. Management errors are returned as errors; candidate failure is
// reported only through FrozenCell.
type CellAdapter interface {
	Prepare(context.Context) error
	Arm(context.Context) error
	Release(context.Context) (time.Time, error)
	Observe(context.Context) (CellObservation, error)
	Freeze(context.Context) (FrozenCell, error)
	Cleanup(context.Context) (json.RawMessage, error)
}

// CellReceipt retains three independent results for one attempt.
type CellReceipt struct {
	Schema, CellID, AttemptID, ManifestDigest string
	Candidate, Observation, Cleanup           string
	Reason                                    string          `json:",omitempty"`
	ActiveNanos                               int64           `json:"active_nanos"`
	Evidence                                  json.RawMessage `json:",omitempty"`
	CleanupEvidence                           json.RawMessage `json:"cleanup_evidence,omitempty"`
}
