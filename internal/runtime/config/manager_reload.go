package config

import (
	"context"
	"errors"
	"fmt"
	"os"
)

type rollbackFailure struct{ err error }

func (e rollbackFailure) Error() string { return e.err.Error() }
func (e rollbackFailure) Unwrap() error { return e.err }

func (m *Manager) Reload(ctx context.Context) ReloadResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	candidate, err := m.loadCandidate()
	if err != nil {
		return m.recordResult(ReloadResult{Outcome: OutcomeRejectedInvalid, Reason: safeReason(err)})
	}
	sourceChanges := changedPaths(m.candidate, candidate)
	if len(sourceChanges) == 0 {
		return m.recordResult(ReloadResult{Outcome: OutcomeUnchanged})
	}
	effectiveChanges := changedPaths(m.active, candidate)
	immutable, restart, reloadable := classifyChanges(effectiveChanges)
	if len(immutable) > 0 {
		return m.recordResult(ReloadResult{Outcome: OutcomeRejectedImmutable, Immutable: immutable})
	}
	if len(effectiveChanges) == 0 {
		m.candidate = candidate
		m.candidateGeneration++
		m.pendingRestart = nil
		return m.recordResult(ReloadResult{Outcome: OutcomeUnchanged})
	}
	next := applyReloadableChanges(m.active, candidate)
	if len(reloadable) > 0 {
		if result, failed := m.commitReloadable(ctx, next); failed {
			return m.recordResult(result)
		}
	}
	m.candidate = candidate
	m.candidateGeneration++
	m.pendingRestart = append([]string(nil), restart...)
	outcome := OutcomeApplied
	if len(restart) > 0 {
		outcome = OutcomeRestartRequired
	}
	return m.recordResult(ReloadResult{Outcome: outcome, RestartRequired: restart})
}

func (m *Manager) commitReloadable(ctx context.Context, next Document) (ReloadResult, bool) {
	if err := m.applyTransaction(ctx, next); err != nil {
		outcome := OutcomeRolledBack
		var failed rollbackFailure
		if errors.As(err, &failed) {
			outcome = OutcomeRollbackFailed
		}
		return ReloadResult{Outcome: outcome, Reason: safeReason(err)}, true
	}
	m.active = next
	m.activeGeneration++
	m.loadedAt = nowUTC()
	return ReloadResult{}, false
}

func (m *Manager) loadCandidate() (Document, error) {
	candidate, err := decodePath(m.path)
	if err != nil {
		return Document{}, err
	}
	if m.resolver != nil {
		candidate, err = m.resolver(candidate)
		if err != nil {
			return Document{}, err
		}
	}
	for _, validator := range m.validators {
		if err := validator(candidate); err != nil {
			return Document{}, err
		}
	}
	return candidate, nil
}

func (m *Manager) applyTransaction(ctx context.Context, next Document) error {
	for index, applier := range m.appliers {
		if err := applier.Prepare(ctx, m.active, next); err != nil {
			return fmt.Errorf("prepare applier %d: %w", index, err)
		}
	}
	applied := 0
	for index, applier := range m.appliers {
		if err := applier.Apply(ctx, m.active, next); err != nil {
			applyErr := fmt.Errorf("apply applier %d: %w", index, err)
			if rollbackErr := m.rollback(ctx, applied); rollbackErr != nil {
				return rollbackFailure{err: errors.Join(applyErr, rollbackErr)}
			}
			return applyErr
		}
		applied++
	}
	return nil
}

func (m *Manager) rollback(ctx context.Context, count int) error {
	var failures []error
	for index := count - 1; index >= 0; index-- {
		if err := m.appliers[index].Rollback(ctx, m.active); err != nil {
			failures = append(failures, fmt.Errorf("rollback applier %d: %w", index, err))
		}
	}
	return errors.Join(failures...)
}

func (m *Manager) recordResult(result ReloadResult) ReloadResult {
	result.ActiveGeneration = m.activeGeneration
	result.CandidateGeneration = m.candidateGeneration
	m.lastReload = cloneReloadResult(result)
	return result
}

func decodePath(path string) (Document, error) {
	file, err := os.Open(path)
	if err != nil {
		return Document{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return Document{}, err
	}
	if !info.Mode().IsRegular() {
		return Document{}, fmt.Errorf("operator configuration source is not a regular file")
	}
	return Decode(file)
}
