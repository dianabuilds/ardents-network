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

// ClosedProfileView is the narrow immutable State projection consumed by the
// admission owner. It does not expose raw Node Records or a signing capability.
type ClosedProfileView struct {
	NetworkID, StateGeneration, StateDigest [32]byte
	Digest, IssuanceAuthorityKey            [32]byte
	IssuerNodeID                            [32]byte
	IssuerDutyGeneration                    uint64
	Epoch                                   uint64
	NotBefore, NotAfter                     time.Time
	TokenKeyCount                           uint8
	TokenKeys                               [closedprofile.MaxKeys]ClosedProfileTokenKey
}

// ClosedRouteNodeView is one public recipient fact already joined by State to
// the accepted signed closed profile and exact current Node Record. It cannot
// select an alternate recipient, supply an endpoint, or accept profile bytes.
type ClosedRouteNodeView struct {
	NodeID, RecordDigest [32]byte
	RoleDomain, Subrole  uint8
	DutyGeneration       uint64
}

// ClosedRouteView is the forwarding-owner projection of the one accepted
// closed profile. Profile retains the issuer/key projection; Nodes preserves
// only the signed recipient constraints required to reject arbitrary OPEN.
type ClosedRouteView struct {
	Profile   ClosedProfileView
	NodeCount uint8
	Nodes     [closedprofile.MaxNodes]ClosedRouteNodeView
}

// ClosedProfileTokenKey is one immutable public RSA-PSS key/window fact from
// accepted State. It is not an issuer private key or a signing capability.
type ClosedProfileTokenKey struct {
	WindowStart time.Time
	Class       uint8
	SPKI        [346]byte
}

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

// CurrentClosedProfile returns the one durably accepted profile only while it
// still joins the current authenticated closed Route Epoch. A State successor,
// conflict, expiry, or missing persisted profile is unavailable rather than a
// caller-selected fallback.
func (s *networkState) CurrentClosedProfile() (ClosedProfileView, error) {
	if s.resourceGuard != nil {
		if err := s.resourceGuard.Check(); err != nil {
			return ClosedProfileView{}, err
		}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	profile, err := s.currentClosedProfileLocked()
	if err != nil {
		return ClosedProfileView{}, err
	}
	return closedProfileView(profile), nil
}

// CurrentClosedRoute returns only recipient constraints from the same durable
// accepted profile while the State owner and its verified time remain live.
// A successor, conflict, expiry, missing profile or failed owner is unavailable.
func (s *networkState) CurrentClosedRoute() (ClosedRouteView, error) {
	if s.resourceGuard != nil {
		if err := s.resourceGuard.Check(); err != nil {
			return ClosedRouteView{}, err
		}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	profile, err := s.currentClosedProfileLocked()
	if err != nil {
		return ClosedRouteView{}, err
	}
	return closedRouteView(profile), nil
}

// Both runtime projections use one guard while holding State's read lock.
// Offline profile acceptance is separate: possession of persisted signed
// bytes does not permit runtime use after time confidence or its owner fails.
func (s *networkState) currentClosedProfileLocked() (closedprofile.Profile, error) {
	if s.closed || s.current == nil || s.current.Snapshot.Profile != closedRouteProfile || s.distribution.conflicting {
		return closedprofile.Profile{}, errors.New("closed profile is unavailable")
	}
	if err := errors.Join(s.automaticErr, s.resourceErr); err != nil {
		return closedprofile.Profile{}, err
	}
	if s.config.observe == nil {
		return closedprofile.Profile{}, errClockUncertain
	}
	now, err := trustedNow(s.config, s.distribution)
	if err != nil {
		return closedprofile.Profile{}, err
	}
	if now.Before(s.current.Snapshot.EpochValidFrom) || !now.Before(s.current.Snapshot.ValidUntil) {
		return closedprofile.Profile{}, errors.New("closed profile State is not current")
	}
	generation, err := closedProfileGeneration(s.current.Snapshot.Generation)
	if err != nil {
		return closedprofile.Profile{}, err
	}
	stored, raw, err := s.storage.LoadClosedProfile(generation)
	if err != nil || stored == (durable.ClosedProfileState{}) || stored.Conflict != [32]byte{} || stored.Epoch != s.current.Snapshot.Epoch {
		return closedprofile.Profile{}, errors.New("closed profile is unavailable")
	}
	profile, err := s.verifyClosedProfileLocked(raw, generation, now)
	if err != nil || profile.Digest != stored.Accepted || profile.NotBefore.Before(s.current.Snapshot.EpochValidFrom) || profile.NotAfter.After(s.current.Snapshot.ValidUntil) ||
		!matchesClosedProfileCandidates(profile, s.current.Candidates) {
		return closedprofile.Profile{}, errors.New("closed profile is unavailable")
	}
	return profile, nil
}

func closedProfileView(profile closedprofile.Profile) ClosedProfileView {
	view := ClosedProfileView{NetworkID: profile.NetworkID, StateGeneration: profile.StateGeneration, StateDigest: profile.EpochDigest,
		Digest: profile.Digest, IssuanceAuthorityKey: profile.AuthorityKey, IssuerNodeID: profile.IssuerNodeID,
		Epoch: profile.Epoch, NotBefore: profile.NotBefore, NotAfter: profile.NotAfter, TokenKeyCount: uint8(len(profile.Keys))}
	for _, node := range profile.Nodes {
		if node.NodeID == profile.IssuerNodeID {
			view.IssuerDutyGeneration = node.DutyGeneration
			break
		}
	}
	for index, key := range profile.Keys {
		view.TokenKeys[index].WindowStart = time.Unix(int64(key.WindowStart), 0).UTC()
		view.TokenKeys[index].Class = key.Class
		copy(view.TokenKeys[index].SPKI[:], key.SPKI)
	}
	return view
}

func closedRouteView(profile closedprofile.Profile) ClosedRouteView {
	view := ClosedRouteView{Profile: closedProfileView(profile), NodeCount: uint8(len(profile.Nodes))}
	for index, node := range profile.Nodes {
		view.Nodes[index] = ClosedRouteNodeView{NodeID: node.NodeID, RecordDigest: node.RecordDigest,
			RoleDomain: node.RoleDomain, Subrole: node.Subrole, DutyGeneration: node.DutyGeneration}
	}
	return view
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
