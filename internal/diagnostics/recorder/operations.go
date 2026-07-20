package recorder

import (
	"ardents/internal/diagnostics/operation"
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
