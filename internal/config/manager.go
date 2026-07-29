package config

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
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
	trustClassifier     TrustChangeClassifier
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

func (m *Manager) RegisterTrustChangeClassifier(classifier TrustChangeClassifier) error {
	if classifier == nil {
		return fmt.Errorf("trust change classifier is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.trustClassifier != nil {
		return fmt.Errorf("trust change classifier is already registered")
	}
	m.trustClassifier = classifier
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
	raw, err := json.Marshal(doc)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func cloneReloadResult(in ReloadResult) ReloadResult {
	in.RestartRequired = append([]string(nil), in.RestartRequired...)
	in.Immutable = append([]string(nil), in.Immutable...)
	return in
}

type Outcome string

const (
	OutcomeUnchanged         Outcome = "unchanged"
	OutcomeApplied           Outcome = "applied"
	OutcomeRestartRequired   Outcome = "restart_required"
	OutcomeRejectedInvalid   Outcome = "rejected_invalid"
	OutcomeRejectedImmutable Outcome = "rejected_immutable"
	OutcomeRolledBack        Outcome = "rolled_back"
	OutcomeRollbackFailed    Outcome = "rollback_failed"
)

type Applier interface {
	Prepare(context.Context, Document, Document) error
	Apply(context.Context, Document, Document) error
	Rollback(context.Context, Document) error
}

// TransactionCommitter finalizes resources retained by a successful Applier.
// Commit must not fail; fallible work belongs in Apply.
type TransactionCommitter interface {
	Commit(context.Context)
}

type Validator func(Document) error
type Resolver func(Document) (Document, error)
type TrustChangeClassifier func(TrustConfig, TrustConfig) bool

type Service interface {
	GetEffectiveConfig() EffectiveSnapshot
	ReloadConfig(context.Context) ReloadResult
}

type ReloadResult struct {
	Outcome             Outcome  `json:"outcome"`
	ActiveGeneration    uint64   `json:"active_generation"`
	CandidateGeneration uint64   `json:"candidate_generation"`
	RestartRequired     []string `json:"restart_required,omitempty"`
	Immutable           []string `json:"immutable,omitempty"`
	Reason              string   `json:"reason,omitempty"`
}

type EffectiveSnapshot struct {
	APIVersion          string         `json:"api_version"`
	ActiveGeneration    uint64         `json:"active_generation"`
	CandidateGeneration uint64         `json:"candidate_generation"`
	Fingerprint         string         `json:"fingerprint"`
	LoadedAt            time.Time      `json:"loaded_at"`
	Effective           map[string]any `json:"effective"`
	PendingRestart      []string       `json:"pending_restart,omitempty"`
	LastReload          ReloadResult   `json:"last_reload"`
}

func classifyChanges(paths []string, trustReloadable bool) (immutable, restart, reloadable []string) {
	for _, path := range paths {
		switch {
		case hasPathPrefix(path, "node.name", "node.data_dir", "node.image_reference", "network.private_key_path",
			"authority",
			"privacy.channel_grant_store", "privacy.channel_grant_store_key_file", "privacy.replay_key_file",
			"privacy.discovery.replay_path", "privacy.data.replay_path"):
			immutable = append(immutable, path)
		case hasPathPrefix(path, "trust") && trustReloadable,
			hasPathPrefix(path, "policy", "logging.level", "diagnostics", "network.discovery_refresh_seconds"):
			reloadable = append(reloadable, path)
		default:
			restart = append(restart, path)
		}
	}
	return immutable, restart, reloadable
}

func hasPathPrefix(path string, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if path == prefix || strings.HasPrefix(path, prefix+".") {
			return true
		}
	}
	return false
}

func applyReloadableChanges(active, candidate Document, trustReloadable bool) Document {
	next := active
	if trustReloadable {
		next.Trust = candidate.Trust
	}
	next.Policy = candidate.Policy
	next.Logging.Level = candidate.Logging.Level
	next.Diagnostics = candidate.Diagnostics
	next.Network.DiscoveryRefreshSeconds = candidate.Network.DiscoveryRefreshSeconds
	return next
}

func safeReason(err error) string {
	if err == nil {
		return ""
	}
	if os.IsNotExist(err) || os.IsPermission(err) {
		return "operator configuration source is unavailable"
	}
	text := err.Error()
	text = redactPathTokens(text)
	if len(text) > 240 {
		return text[:240]
	}
	return text
}

func redactPathTokens(text string) string {
	fields := strings.Fields(text)
	for index, field := range fields {
		trimmed := strings.Trim(field, `"'(),:`)
		if strings.Contains(trimmed, `\`) || strings.HasPrefix(trimmed, "/") {
			fields[index] = strings.Replace(field, trimmed, "[redacted-path]", 1)
		}
	}
	return strings.Join(fields, " ")
}

var nowUTC = func() time.Time { return time.Now().UTC() }

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
	trustReloadable := m.trustClassifier != nil && m.trustClassifier(m.active.Trust, candidate.Trust)
	immutable, restart, reloadable := classifyChanges(effectiveChanges, trustReloadable)
	if len(immutable) > 0 {
		return m.recordResult(ReloadResult{Outcome: OutcomeRejectedImmutable, Immutable: immutable})
	}
	if len(effectiveChanges) == 0 {
		m.candidate = candidate
		m.candidateGeneration++
		m.pendingRestart = nil
		return m.recordResult(ReloadResult{Outcome: OutcomeUnchanged})
	}
	next := applyReloadableChanges(m.active, candidate, trustReloadable)
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
		if _, ok := errors.AsType[rollbackFailure](err); ok {
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
	candidate, err := Load(m.path)
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
	for index := applied - 1; index >= 0; index-- {
		if committer, ok := m.appliers[index].(TransactionCommitter); ok {
			committer.Commit(ctx)
		}
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

func Load(path string) (document Document, returnErr error) {
	file, err := os.Open(path)
	if err != nil {
		return Document{}, err
	}
	defer func() {
		if err := file.Close(); err != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("close operator configuration: %w", err))
		}
	}()
	info, err := file.Stat()
	if err != nil {
		return Document{}, err
	}
	if !info.Mode().IsRegular() {
		return Document{}, fmt.Errorf("operator configuration source is not a regular file")
	}
	return Decode(file)
}
