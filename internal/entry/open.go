package entry

import (
	"errors"
	"path/filepath"
)

// Open claims one Entry-owned root, verifies durable state, and retires an
// Invite that no longer matches current authenticated State before returning.
func Open(input Config) (*owner, error) {
	config, err := copyConfig(input)
	if err != nil {
		return nil, err
	}
	root, err := filepath.Abs(config.Root)
	if err != nil {
		return nil, err
	}
	if err := inspectRoot(root); err != nil {
		return nil, err
	}
	if err := verifyRootCandidate(root); err != nil {
		return nil, err
	}
	lease, err := acquireRootLease(root)
	if err != nil {
		return nil, err
	}
	opened := false
	defer func() {
		if !opened {
			_ = lease.release()
		}
	}()
	if err := verifyRootClaim(root); err != nil {
		return nil, err
	}
	if err := validateRootPermissions(root); err != nil {
		return nil, err
	}
	if err := prepareRoot(root); err != nil {
		return nil, err
	}
	state, current, err := loadState(root)
	if err != nil {
		return nil, err
	}
	if err := cleanupGenerations(root, current, state.Previous); err != nil {
		return nil, err
	}
	owner := &owner{root: root, lease: lease, config: config, state: state, current: current}
	next := owner.state.clone()
	changed := false
	interrupted := false
	ended := int64(0)
	for index := range next.Contacts {
		contact := &next.Contacts[index]
		if contact.Outcome == "" {
			contact.Outcome = "interrupted"
			contact.Terminal = contact.Started
			changed = true
		}
		if !contact.Cleanup {
			interrupted = true
		}
		if contact.Terminal > ended {
			ended = contact.Terminal
		}
	}
	if next.Attempt != nil && (next.Attempt.Terminal == "" || interrupted) {
		if ended < next.Attempt.Started {
			ended = next.Attempt.Started
		}
		next.Attempt.Terminal, next.Attempt.Ended = "entry-interrupted", ended
		changed = true
	}
	for index := range next.Records {
		record := &next.Records[index]
		if record.Status != memberActive && record.Status != memberDraining && record.Status != memberVerified {
			continue
		}
		decoded, _, class, validateErr := owner.validate(record.Invite)
		if validateErr != nil {
			return nil, validateErr
		}
		if class != Accepted || decoded.id != record.InviteID || decoded.nodeID != record.Identity || decoded.familyID != record.Family {
			retireMember(record)
			changed = true
		}
	}
	if next.Attempt != nil && next.Attempt.Terminal != "" {
		if err := owner.retireInvalidVerifiedLocked(&next); err != nil {
			return nil, err
		}
		changed = next.settleReplacements() || changed
	}
	if changed {
		if err := owner.commit(next, false); err != nil {
			return nil, err
		}
	}
	opened = true
	return owner, nil
}

// Close releases the exclusive Entry root lease. It is idempotent.
func (owner *owner) Close() error {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.closed {
		return nil
	}
	owner.closed = true
	return owner.lease.release()
}

func copyConfig(input Config) (Config, error) {
	if input.Root == "" || input.Current == nil || input.Conflict == nil || input.Clock == nil || input.TimeConfident == nil {
		return Config{}, errors.New("entry configuration is incomplete")
	}
	if input.Clock().IsZero() {
		return Config{}, errors.New("entry clock is invalid")
	}
	return input, nil
}
