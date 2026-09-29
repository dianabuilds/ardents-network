package state

import (
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/closedprofile"
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

// CurrentClosedProfile returns the one durably accepted profile only while it
// still joins the current authenticated closed Route Epoch. A State successor,
// conflict, expiry, or missing persisted profile is unavailable rather than a
// caller-selected fallback.
func (s *networkState) CurrentClosedProfile() (ClosedProfileView, error) {
	profile, err := s.currentClosedProfile()
	if err != nil {
		return ClosedProfileView{}, err
	}
	return closedProfileView(profile), nil
}

// CurrentClosedRoute returns the accepted profile and its exact recipient
// constraints while the State owner and its verified time remain live.
// A successor, conflict, expiry, missing profile or failed owner is unavailable.
func (s *networkState) CurrentClosedRoute() (ClosedRouteView, error) {
	profile, err := s.currentClosedProfile()
	if err != nil {
		return ClosedRouteView{}, err
	}
	return closedRouteView(profile), nil
}

// Both runtime projections check the resource guard, then verify the persisted
// profile under State's read lock. Offline profile acceptance is separate:
// signed bytes alone do not permit runtime use after time or owner failure.
func (s *networkState) currentClosedProfile() (closedprofile.Profile, error) {
	if s.resourceGuard != nil {
		if err := s.resourceGuard.Check(); err != nil {
			return closedprofile.Profile{}, err
		}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
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
