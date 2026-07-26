package discovery

import (
	"fmt"

	discoveryrecord "ardents/internal/discovery/records"
	identitytrust "ardents/internal/identity/trust"
)

func (s *Service) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.path == "" {
		return nil
	}
	var persisted Snapshot
	found, err := LoadSnapshot(s.path, &persisted)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	if persisted.SchemaVersion != 2 {
		return fmt.Errorf("discovery snapshot schema is unsupported")
	}
	if err := validateSnapshotState(persisted); err != nil {
		return err
	}
	validated, err := s.validateSnapshotEntries(persisted.Records, true)
	if err != nil {
		return fmt.Errorf("persisted discovery snapshot is invalid: %w", err)
	}
	refreshEvidence := len(validated) != len(persisted.Records)
	for index := range validated {
		if index >= len(persisted.Records) || validated[index].Evidence != persisted.Records[index].Evidence {
			refreshEvidence = true
			break
		}
	}
	if refreshEvidence {
		refreshed := persisted
		refreshed.Records = CloneEntries(validated)
		if err := s.persist(s.path, refreshed); err != nil {
			return fmt.Errorf("persist refreshed discovery evidence: %w", err)
		}
	}
	s.records = validated
	s.state = persisted.State
	s.reason = persisted.Reason
	return nil
}

func (s *Service) ApplyTrustRegistry(registry *identitytrust.Registry) error {
	transaction, err := s.BeginTrustRegistry(registry)
	if err != nil {
		return err
	}
	transaction.Commit()
	return nil
}

// TrustTransaction keeps Discovery mutations serialized until the surrounding
// configuration transaction either commits or restores the prior truth.
type TrustTransaction struct {
	service         *Service
	registry        *identitytrust.Registry
	previousRecords []Entry
	previousState   string
	previousReason  string
	active          bool
}

// BeginTrustRegistry persists candidate trust effects without publishing the
// candidate registry. The caller must finish the transaction.
func (s *Service) BeginTrustRegistry(
	registry *identitytrust.Registry,
) (*TrustTransaction, error) {
	s.mu.Lock()
	transaction := &TrustTransaction{
		service:         s,
		registry:        registry,
		previousRecords: CloneEntries(s.records),
		previousState:   s.state,
		previousReason:  s.reason,
		active:          true,
	}
	if err := s.applyTrustRegistryLocked(registry, s.records, s.state, s.reason); err != nil {
		transaction.finish()
		return nil, err
	}
	return transaction, nil
}

func (t *TrustTransaction) Commit() {
	if t == nil || !t.active {
		return
	}
	t.service.trust.ReplaceRegistry(t.registry)
	t.finish()
}

func (t *TrustTransaction) Rollback(
	registry *identitytrust.Registry,
) error {
	if t == nil || !t.active {
		return fmt.Errorf("discovery trust transaction is not active")
	}
	defer t.finish()
	candidate := Snapshot{
		SchemaVersion: 2,
		Records:       CloneEntries(t.previousRecords),
		State:         t.previousState,
		Reason:        t.previousReason,
	}
	if candidate.State == "" {
		candidate.State = "ready"
	}
	if err := validateSnapshotState(candidate); err != nil {
		return err
	}
	if err := t.service.applyTrustRegistryLocked(
		registry,
		candidate.Records,
		candidate.State,
		candidate.Reason,
	); err != nil {
		return err
	}
	t.service.trust.ReplaceRegistry(registry)
	return nil
}

func (t *TrustTransaction) finish() {
	if !t.active {
		return
	}
	t.active = false
	t.service.mu.Unlock()
}

func (s *Service) applyTrustRegistryLocked(
	registry *identitytrust.Registry,
	records []Entry,
	state string,
	reason string,
) error {
	candidateTrust := NewTrustEvaluator(registry)
	validated, err := s.validateSnapshotEntriesWithTrust(records, false, candidateTrust)
	if err != nil {
		return fmt.Errorf("refresh discovery trust: %w", err)
	}
	if s.path != "" {
		refreshed := Snapshot{
			SchemaVersion: 2,
			Records:       CloneEntries(validated),
			State:         state,
			Reason:        reason,
		}
		if err := s.persist(s.path, refreshed); err != nil {
			return fmt.Errorf("persist refreshed discovery trust: %w", err)
		}
	}
	s.records = validated
	s.state = state
	s.reason = reason
	return nil
}

func (s *Service) validateSnapshotEntries(entries []Entry, persisted bool) ([]Entry, error) {
	return s.validateSnapshotEntriesWithTrust(entries, persisted, s.trust)
}

func (s *Service) validateSnapshotEntriesWithTrust(
	entries []Entry,
	persisted bool,
	trust *TrustEvaluator,
) ([]Entry, error) {
	seenID := make(map[string]struct{}, len(entries))
	seenSubject := make(map[string]struct{}, len(entries))
	validated := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		if persisted {
			if err := discoveryrecord.ValidateEvidence(entry.Record, entry.Evidence); err != nil {
				return nil, fmt.Errorf("record evidence is invalid: %w", err)
			}
		}
		freshEvidence, err := trust.VerifyRetained(entry.Record)
		if err != nil {
			return nil, fmt.Errorf("record is invalid: %w", err)
		}
		if persisted && entry.Evidence.TrustGeneration == freshEvidence.TrustGeneration && entry.Evidence.Trusted != freshEvidence.Trusted {
			return nil, fmt.Errorf("record evidence trust result is invalid")
		}
		if !discoveryrecord.ValidSource(entry.Source) || entry.SeenAt.IsZero() {
			return nil, fmt.Errorf("entry metadata is invalid")
		}
		id, subjectKey := entry.Record.RecordID(), entry.Record.Kind()+"\x00"+entry.Record.Subject()
		if _, duplicate := seenID[id]; duplicate {
			return nil, fmt.Errorf("record id is duplicated")
		}
		if _, duplicate := seenSubject[subjectKey]; duplicate {
			return nil, fmt.Errorf("record subject is duplicated")
		}
		seenID[id], seenSubject[subjectKey] = struct{}{}, struct{}{}
		if entry.Source == discoveryrecord.Bootstrap && !freshEvidence.Trusted {
			continue
		}
		entry.Record = entry.Record.Clone()
		entry.Evidence = freshEvidence
		validated = append(validated, entry)
	}
	return validated, nil
}

func validateSnapshotState(snapshot Snapshot) error {
	switch snapshot.State {
	case "new":
		if len(snapshot.Records) != 0 || snapshot.Reason != "" {
			return fmt.Errorf("new discovery snapshot contains state")
		}
	case "ready":
		if snapshot.Reason != "" {
			return fmt.Errorf("ready discovery snapshot contains a reason")
		}
	case "degraded":
		if snapshot.Reason == "" {
			return fmt.Errorf("degraded discovery snapshot lacks a reason")
		}
	default:
		return fmt.Errorf("discovery snapshot state is unsupported")
	}
	return nil
}

func (s *Service) Degrade(reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = "degraded"
	s.reason = reason
	return s.saveLocked()
}

func (s *Service) Ready() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = "ready"
	s.reason = ""
	return s.saveLocked()
}

func (s *Service) State() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

func (s *Service) Reason() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reason
}

func (s *Service) Records() []Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Record, 0, len(s.records))
	for _, item := range s.records {
		out = append(out, item.Record.Clone())
	}
	return out
}

func (s *Service) Entries() []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	return CloneEntries(s.records)
}

func (s *Service) markReadyLocked() {
	if s.state == "degraded" && s.reason != "" {
		return
	}
	s.state = "ready"
	s.reason = ""
}
