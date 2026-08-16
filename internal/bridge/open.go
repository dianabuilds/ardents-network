package bridge

import (
	"errors"
	"path/filepath"
)

// Open claims one exclusive state root, verifies its durable generation, and
// revalidates or retires every retained Invite before returning.
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
	changed := false
	terminalOffset := uint64(0)
	for index := range owner.state.Contacts {
		if owner.state.Contacts[index].Outcome == "" {
			owner.state.Contacts[index].Outcome = "interrupted"
			owner.state.Contacts[index].Terminal = owner.state.Contacts[index].Started
			changed = true
		}
		if owner.state.Contacts[index].Terminal > terminalOffset {
			terminalOffset = owner.state.Contacts[index].Terminal
		}
	}
	if owner.state.Attempt != nil && owner.state.Attempt.Terminal == "" {
		owner.state.Attempt.Terminal = "bridge-interrupted"
		owner.state.Attempt.TerminalOffset = terminalOffset
		changed = true
	}
	if owner.state.Attempt != nil && owner.state.Attempt.Terminal != "" {
		owner.state.settleReplacements()
	}
	for index := range owner.state.Records {
		record := &owner.state.Records[index]
		if record.Status != memberActive && record.Status != memberDraining && record.Status != memberVerified {
			continue
		}
		invite, class, validateErr := owner.validate(record.Invite)
		if validateErr != nil {
			return nil, validateErr
		}
		if class != classAccepted || invite.id != record.InviteID || invite.commitment != record.Commitment ||
			invite.adapterProfile != record.ProfileID {
			record.Status = memberRetired
			record.Invite = nil
			record.Commitment = [32]byte{}
			record.ProfileID = ""
			changed = true
		}
	}
	if changed {
		if err := owner.commit(owner.state, false); err != nil {
			return nil, err
		}
	}
	opened = true
	return owner, nil
}

// Close releases the exclusive root lease. It is idempotent.
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
	if input.Root == "" || input.ValidateCandidate == nil || input.RouteProfile == "" ||
		input.CurrentNetwork == nil || input.RoleConflict == nil || input.Clock == nil {
		return Config{}, errors.New("bridge configuration is incomplete")
	}
	if input.Clock().IsZero() {
		return Config{}, errors.New("bridge clock is invalid")
	}
	return input, nil
}
