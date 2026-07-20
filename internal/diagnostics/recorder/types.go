package recorder

import (
	"ardents/internal/diagnostics/event"
	"ardents/internal/diagnostics/health"
	"ardents/internal/diagnostics/operation"
	"ardents/internal/diagnostics/reason"
	"sync"
)

const PersistenceFailureCode = "diagnostics.persistence.failed"

type Snapshot struct {
	Health            health.Summary     `json:"health"`
	RecentEvents      []event.Record     `json:"recent_events,omitempty"`
	PendingOperations []operation.Record `json:"pending_operations,omitempty"`
}

type Recorder struct {
	mu           sync.Mutex
	path         string
	maxEvents    int
	maxClosedOps int
	detailLevel  string
	seq          int64
	events       []event.Record
	operations   map[string]operation.Record
	subsystems   map[string]health.SubsystemStatus
	primary      *reason.Reason
	primarySet   bool
	primaryState string
	health       health.Summary
	retained     health.Summary
}

type CorruptLedgerError struct {
	Err   error
	Fatal bool
}

func (e *CorruptLedgerError) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *CorruptLedgerError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func IsCorruptLedger(err error) (*CorruptLedgerError, bool) {
	if target, ok := err.(*CorruptLedgerError); ok {
		return target, true
	}
	target, ok := isCorruptLedger(err)
	if !ok {
		return nil, false
	}
	return target, true
}
