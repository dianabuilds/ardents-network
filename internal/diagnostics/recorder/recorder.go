package recorder

import (
	"ardents/internal/diagnostics/event"
	"ardents/internal/diagnostics/health"
	"ardents/internal/diagnostics/operation"
	"errors"
	"path/filepath"
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
