package diagnostics

import (
	"ardents/internal/diagnostics/event"
	"ardents/internal/diagnostics/health"
	"ardents/internal/diagnostics/operation"
	db "ardents/internal/storage"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

func (r *Recorder) PendingOperations() []operation.Record {
	r.mu.Lock()
	defer r.mu.Unlock()
	return operation.Clone(r.pendingOperationsLocked())
}

func (r *Recorder) BeginOperation(kind, domain, resource string, recoverable bool, recoveryAction string) operation.Record {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	item := operation.New(kind, domain, resource, recoverable, recoveryAction, now)
	r.operations[item.ID] = item
	r.persistLocked()
	return item
}

func (r *Recorder) CompleteOperation(id, reason string) {
	r.transitionOperation(id, operation.Completed, reason)
}
func (r *Recorder) FailOperation(id, reason string) {
	r.transitionOperation(id, operation.Failed, reason)
}
func (r *Recorder) RecoverOperation(id, reason string) {
	r.transitionOperation(id, operation.Recovering, reason)
}
func (r *Recorder) AbandonOperation(id, reason string) {
	r.transitionOperation(id, operation.Abandoned, reason)
}

func (r *Recorder) MarkRecoveringExcept(excludeID, reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	changed := false
	for id, item := range r.operations {
		if id == excludeID || !operation.IsOpen(item.State) {
			continue
		}
		r.operations[id] = operation.MarkRecovering(item, reason, now)
		changed = true
	}
	if changed {
		r.persistLocked()
	}
}

func (r *Recorder) transitionOperation(id, state, reasonText string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	item, ok := r.operations[id]
	if !ok {
		return
	}
	r.operations[id] = operation.Transition(item, state, reasonText, time.Now().UTC())
	r.persistLocked()
}

func (r *Recorder) pendingOperationsLocked() []operation.Record {
	return operation.PendingItems(r.operations)
}

type Ledger struct {
	Seq            int64              `json:"seq,omitempty"`
	Health         health.Summary     `json:"health"`
	RetainedHealth health.Summary     `json:"retained_health"`
	Events         []event.Record     `json:"events,omitempty"`
	Operations     []operation.Record `json:"operations"`
}

const maxClosedOperations = 32

func Load(path string) (Ledger, error) {
	if path == "" {
		return Ledger{}, nil
	}
	raw, found, err := db.ReadProtectedFile(path)
	if err != nil {
		return Ledger{}, err
	}
	if !found {
		return Ledger{}, nil
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return Ledger{}, nil
	}
	return Decode(raw)
}

func Decode(raw []byte) (Ledger, error) {
	var stored Ledger
	if err := json.Unmarshal(raw, &stored); err != nil {
		return Ledger{}, &CorruptLedgerError{Err: fmt.Errorf("diagnostics ledger decode failed: %w", err), Fatal: true}
	}
	return stored, nil
}

func Save(path string, ledger Ledger) error {
	if path == "" {
		return nil
	}
	items := operation.Compact(operation.Clone(ledger.Operations), maxClosedOperations)
	sort.Slice(items, func(i, j int) bool {
		if items[i].StartedAt.Equal(items[j].StartedAt) {
			return items[i].ID < items[j].ID
		}
		return items[i].StartedAt.Before(items[j].StartedAt)
	})
	raw, err := json.MarshalIndent(Ledger{
		Seq:            ledger.Seq,
		Health:         health.CloneSummary(ledger.Health),
		RetainedHealth: health.CloneSummary(ledger.RetainedHealth),
		Events:         event.Clone(ledger.Events),
		Operations:     items,
	}, "", "  ")
	if err != nil {
		return err
	}
	return db.AtomicWritePrivateFile(path, raw)
}

func isCorruptLedger(err error) (*CorruptLedgerError, bool) {
	if target, ok := errors.AsType[*CorruptLedgerError](err); ok {
		return target, true
	}
	return nil, false
}
