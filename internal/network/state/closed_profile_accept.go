package state

import (
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/closedprofile"
	"github.com/dianabuilds/ardents-network/internal/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/network/state/durable"
)

// AcceptClosedProfile verifies and durably accepts the sole profile for the
// current closed Route Epoch. A second valid digest becomes durable conflict;
// callers never select a winner by arrival order.
func (s *networkState) AcceptClosedProfile(raw []byte) (ClosedProfileView, error) {
	return s.acceptClosedProfileWithCommit(raw, s.storage.CommitClosedProfile)
}

func (s *networkState) acceptClosedProfileWithCommit(raw []byte, commit func(durable.ClosedProfileState, []byte) error) (ClosedProfileView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.current == nil || s.current.Snapshot.Profile != closedRouteProfile || s.distribution.conflicting {
		return ClosedProfileView{}, errors.New("closed profile is unavailable")
	}
	generation, err := closedProfileGeneration(s.current.Snapshot.Generation)
	if err != nil {
		return ClosedProfileView{}, err
	}
	now := s.config.clock().UTC()
	profile, err := s.verifyClosedProfileLocked(raw, generation, now)
	if err != nil || profile.NotBefore.Before(s.current.Snapshot.EpochValidFrom) || profile.NotAfter.After(s.current.Snapshot.ValidUntil) || !matchesClosedProfileCandidates(profile, s.current.Candidates) {
		return ClosedProfileView{}, errors.New("closed profile does not match accepted State")
	}
	stored, storedRaw, err := s.storage.LoadClosedProfile(generation)
	if err != nil {
		return ClosedProfileView{}, err
	}
	if stored != (durable.ClosedProfileState{}) {
		if stored.Epoch != profile.Epoch || stored.Accepted != profile.Digest || stored.Conflict != [32]byte{} {
			if stored.Conflict == [32]byte{} && stored.Epoch == profile.Epoch && stored.Accepted != profile.Digest {
				stored.Conflict = profile.Digest
				if err := commit(stored, storedRaw); err != nil {
					// A second verified digest is already known. If its conflict
					// cannot be persisted, the old accepted profile is unsafe to serve.
					s.terminalErr = fmt.Errorf("persist closed profile conflict: %w", err)
					s.retireStateLocked()
					return ClosedProfileView{}, err
				}
			}
			return ClosedProfileView{}, errors.New("closed profile has a durable conflict")
		}
	} else if err := commit(durable.ClosedProfileState{Generation: generation, Epoch: profile.Epoch, Accepted: profile.Digest}, raw); err != nil {
		if errors.Is(err, durable.ErrClosedProfileStateSyncUncertain) {
			s.terminalErr = fmt.Errorf("closed profile publication is uncertain: %w", err)
			s.retireStateLocked()
		}
		return ClosedProfileView{}, err
	}
	return closedProfileView(profile), nil
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

func matchesClosedProfileCandidates(profile closedprofile.Profile, candidates []epoch.Candidate) bool {
	available := make(map[[32]byte]epoch.Candidate, len(candidates))
	for _, candidate := range candidates {
		available[candidate.NodeID] = candidate
	}
	for _, node := range profile.Nodes {
		candidate, exists := available[node.NodeID]
		if !exists || candidate.RecordGeneration != node.DutyGeneration ||
			candidate.RecordDigest != node.RecordDigest ||
			!epoch.CarrierEligible(closedRouteProfile, candidate.CarrierProfile) {
			return false
		}
		// Each candidate's domain was assigned by the verified Epoch from its
		// authenticated family before this signed profile is admitted.
		domain, known := closedRoleDomain(candidate.Domain)
		if !known || domain != node.RoleDomain {
			return false
		}
	}
	return true
}

// closedRoleDomain maps an authenticated Epoch role-domain assignment to the
// contract-fixed closed-profile Role Domain number: Initiator=1, Rendezvous=2,
// Responder=3, Introduction=4 (docs/technical/protected-route-protocol.md).
// The former interactive transit-issuance domain has no closed Role Domain, so
// it is unknown here and its records cannot join a closed profile.
func closedRoleDomain(assignment string) (byte, bool) {
	switch assignment {
	case "initiator":
		return 1, true
	case "rendezvous":
		return 2, true
	case "responder":
		return 3, true
	case "introduction":
		return 4, true
	default:
		return 0, false
	}
}
