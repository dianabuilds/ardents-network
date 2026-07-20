package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type Manager struct {
	mu                  sync.Mutex
	path                string
	active              Document
	candidate           Document
	activeGeneration    uint64
	candidateGeneration uint64
	loadedAt            time.Time
	pendingRestart      []string
	lastReload          ReloadResult
	appliers            []Applier
	validators          []Validator
	resolver            Resolver
}

func (m *Manager) RegisterResolver(resolver Resolver) error {
	if resolver == nil {
		return fmt.Errorf("configuration resolver is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.resolver != nil {
		return fmt.Errorf("configuration resolver is already registered")
	}
	m.resolver = resolver
	return nil
}

func (m *Manager) RegisterValidator(validator Validator) error {
	if validator == nil {
		return fmt.Errorf("configuration validator is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.validators = append(m.validators, validator)
	return nil
}

func NewManager(path string, initial Document, appliers ...Applier) (*Manager, error) {
	if path == "" {
		return nil, fmt.Errorf("operator configuration path is required")
	}
	if err := Validate(initial); err != nil {
		return nil, err
	}
	return &Manager{
		path: path, active: initial, candidate: initial,
		activeGeneration: 1, candidateGeneration: 1, loadedAt: time.Now().UTC(),
		lastReload: ReloadResult{Outcome: OutcomeUnchanged, ActiveGeneration: 1, CandidateGeneration: 1},
		appliers:   append([]Applier(nil), appliers...),
	}, nil
}

func (m *Manager) RegisterApplier(applier Applier) error {
	if applier == nil {
		return fmt.Errorf("configuration applier is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.appliers = append(m.appliers, applier)
	return nil
}

func (m *Manager) Snapshot() EffectiveSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	return EffectiveSnapshot{
		APIVersion: Version, ActiveGeneration: m.activeGeneration,
		CandidateGeneration: m.candidateGeneration, Fingerprint: documentFingerprint(m.active),
		LoadedAt: m.loadedAt, Effective: redactDocument(m.active),
		PendingRestart: append([]string(nil), m.pendingRestart...), LastReload: cloneReloadResult(m.lastReload),
	}
}

func documentFingerprint(doc Document) string {
	raw, _ := json.Marshal(doc)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func cloneReloadResult(in ReloadResult) ReloadResult {
	in.RestartRequired = append([]string(nil), in.RestartRequired...)
	in.Immutable = append([]string(nil), in.Immutable...)
	return in
}
