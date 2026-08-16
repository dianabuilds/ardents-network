package bridge

import (
	"crypto/sha256"
	"errors"
)

// Evidence returns a copy of the finite retained attempt/contact result.
func (owner *owner) Evidence() (AttemptEvidence, error) {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	if owner.closed || owner.failed != nil {
		return AttemptEvidence{}, errors.New("bridge evidence is unavailable")
	}
	if owner.state.Attempt == nil || owner.state.Regime == nil || len(owner.state.Contacts) > 4 {
		return AttemptEvidence{}, errors.New("bridge attempt evidence is unavailable")
	}
	hash := sha256.New()
	hash.Write([]byte("ardents-h3-bridge-evidence-attempt-v1\x00"))
	hash.Write(owner.state.Regime.Manifest[:])
	hash.Write(owner.state.Attempt.AttemptID[:])
	result := AttemptEvidence{Terminal: owner.state.Attempt.Terminal,
		DeadlineOffset: owner.state.Attempt.DeadlineOffset,
		TerminalOffset: owner.state.Attempt.TerminalOffset,
		ContactStarts:  uint8(len(owner.state.Contacts)), CleanupComplete: len(owner.state.Contacts) > 0}
	copy(result.AttemptDigest[:], hash.Sum(nil))
	for _, contact := range owner.state.Contacts {
		if contact.Outcome == "" || !contact.Cleanup {
			result.CleanupComplete = false
		}
	}
	return result, nil
}
