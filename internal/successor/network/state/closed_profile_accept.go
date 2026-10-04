package state

import (
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	networkdomain "github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/network/closedprofile"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state/durable"
)

// ProfileReceipt confirms a committed profile identity, never current authority.
// Consumers obtain runtime facts from the opened owner's CurrentRuntime.
type ProfileReceipt struct {
	Generation, EpochDigest, Digest [32]byte
	Epoch                           uint64
}

// AcceptClosedProfile verifies and durably accepts the sole profile for the
// current closed Route Epoch. A second valid digest becomes durable conflict;
// callers never select a winner by arrival order.
func (s *networkState) AcceptClosedProfile(raw []byte) (ProfileReceipt, error) {
	return s.acceptClosedProfileWithCommit(raw, s.storage.CommitClosedProfile)
}

func (s *networkState) acceptClosedProfileWithCommit(raw []byte, commit func(durable.ClosedProfileState, []byte) error) (ProfileReceipt, error) {
	if s.config.permitWork != nil {
		if err := s.config.permitWork(); err != nil {
			return ProfileReceipt{}, err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.current == nil || s.current.Snapshot.Profile != closedRouteProfile || s.distribution.conflicting {
		return ProfileReceipt{}, errors.New("closed profile is unavailable")
	}
	generation, err := closedProfileGeneration(s.current.Snapshot.Generation)
	if err != nil {
		return ProfileReceipt{}, err
	}
	now := s.config.clock().UTC()
	profile, err := s.verifyClosedProfileLocked(raw, generation, now)
	if err != nil {
		return ProfileReceipt{}, errors.New("closed profile does not match accepted State")
	}
	if _, err := s.bindCurrentMembership(profile); err != nil {
		return ProfileReceipt{}, err
	}
	stored, storedRaw, err := s.storage.LoadClosedProfile(generation)
	if err != nil {
		return ProfileReceipt{}, err
	}
	if stored == (durable.ClosedProfileState{}) {
		stored.Generation, stored.Epoch = generation, profile.Epoch
	}
	history, err := networkdomain.RestoreProfileHistory(stored.Generation, stored.Epoch, stored.Accepted, stored.Conflict)
	if err != nil {
		return ProfileReceipt{}, err
	}
	decision, err := history.Consider(profileBinding(profile))
	if err != nil {
		return ProfileReceipt{}, err
	}
	if decision.NeedsCommit() {
		body := raw
		if decision.Conflicted() {
			body = storedRaw
		}
		stored.Accepted, stored.Conflict = decision.AcceptedDigest(), decision.ConflictDigest()
		if err := commit(stored, body); err != nil {
			if decision.Conflicted() {
				s.terminalErr = fmt.Errorf("persist closed profile conflict: %w", err)
				s.retireStateLocked()
			} else if errors.Is(err, durable.ErrClosedProfileStateSyncUncertain) {
				s.terminalErr = fmt.Errorf("closed profile publication is uncertain: %w", err)
				s.retireStateLocked()
			}
			return ProfileReceipt{}, err
		}
	}
	if decision.Conflicted() {
		return ProfileReceipt{}, errors.New("closed profile has a durable conflict")
	}
	return ProfileReceipt{Generation: profile.StateGeneration, EpochDigest: profile.EpochDigest, Digest: profile.Digest, Epoch: profile.Epoch}, nil
}

// verifyClosedProfileLocked supplies the exact current authenticated Epoch
// context; the grammar verifier itself has no State acceptance authority.
func (s *networkState) verifyClosedProfileLocked(raw []byte, generation [32]byte, now time.Time) (closedprofile.Profile, error) {
	return closedprofile.Verify(raw, closedprofile.Context{
		StateGeneration: generation, NetworkID: s.current.Snapshot.NetworkID,
		EpochDigest: s.current.Snapshot.Digest, Epoch: s.current.Snapshot.Epoch,
		Authority: s.config.closedProfileAuthority, Now: now,
	})
}

func closedProfileGeneration(encoded string) ([32]byte, error) {
	if len(encoded) != 64 {
		return [32]byte{}, errors.New("state generation is invalid")
	}
	decoded, err := hex.DecodeString(encoded)
	if err != nil || hex.EncodeToString(decoded) != encoded {
		return [32]byte{}, errors.New("state generation is invalid")
	}
	var generation [32]byte
	copy(generation[:], decoded)
	return generation, nil
}
