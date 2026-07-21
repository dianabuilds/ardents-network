package diagnostics

import (
	"ardents/internal/diagnostics/event"
	"ardents/internal/diagnostics/health"
	"ardents/internal/diagnostics/operation"
	"errors"
	"path/filepath"
	"sync"
	"time"
)

func New(path string) *Recorder {
	r := &Recorder{
		path:         path,
		maxEvents:    64,
		maxClosedOps: 32,
		detailLevel:  "standard",
		operations:   map[string]operation.Record{},
		subsystems:   map[string]health.SubsystemStatus{},
	}
	r.refreshHealthLocked(time.Now().UTC())
	return r
}

func NewInDir(dir string) *Recorder {
	return New(filepath.Join(dir, "operations.json"))
}

func (r *Recorder) SetPath(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.path = path
}

func (r *Recorder) Load() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.path == "" {
		return nil
	}
	stored, err := Load(r.path)
	if err != nil {
		return err
	}
	nonFatal := r.restoreLedgerLocked(stored)
	if nonFatal {
		if err := r.saveLocked(); err != nil {
			return err
		}
		return &CorruptLedgerError{Err: errors.New("diagnostics ledger contained invalid operation entries"), Fatal: false}
	}
	return nil
}

func (r *Recorder) restoreLedgerLocked(stored Ledger) bool {
	r.seq = stored.Seq
	r.events = CloneEventRecords(stored.Events)
	if len(r.events) > r.maxEvents {
		r.events = append([]event.Record(nil), r.events[len(r.events)-r.maxEvents:]...)
	}
	r.restoreHealthLocked(stored.Health)
	r.retained = health.CloneSummary(stored.RetainedHealth)
	r.operations = map[string]operation.Record{}

	nonFatal := false
	for _, item := range operation.Compact(stored.Operations, r.maxClosedOps) {
		normalized, changed := operation.Normalize(item, time.Now().UTC())
		if changed {
			nonFatal = true
		}
		if operation.IsOpen(normalized.State) {
			normalized = operation.MarkRecovering(normalized, "operation recovered after restart", time.Now().UTC())
		}
		r.operations[normalized.ID] = normalized
	}
	return nonFatal
}

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
	primary      *Reason
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
	var target *CorruptLedgerError
	if errors.As(err, &target) {
		return target, true
	}
	target, ok := isCorruptLedger(err)
	if !ok {
		return nil, false
	}
	return target, true
}

func (r *Recorder) SetMaxEvents(limit int) {
	if limit <= 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.maxEvents = limit
	if len(r.events) > limit {
		r.events = append([]event.Record(nil), r.events[len(r.events)-limit:]...)
	}
}

func (r *Recorder) SetDetailLevel(level string) {
	if level != "minimal" && level != "standard" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.detailLevel = level
}
