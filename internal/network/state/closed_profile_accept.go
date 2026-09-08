package state

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"
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
	TokenKeys                               [maximumClosedProfileKeys]ClosedProfileTokenKey
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
	Nodes     [maximumClosedProfileNodes]ClosedRouteNodeView
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
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.current == nil || s.currentDecision == nil || s.current.Profile != closedRouteProfile || s.distribution.conflicting {
		return ClosedProfileView{}, errors.New("closed profile is unavailable")
	}
	generation, err := closedProfileGeneration(s.current.Generation)
	if err != nil {
		return ClosedProfileView{}, err
	}
	now := s.config.clock().UTC()
	profile, err := parseClosedProfile(raw, generation, s.current.NetworkID, s.current.Digest, s.current.Epoch, s.config.closedProfileAuthority, now)
	if err != nil || profile.notBefore.Before(s.current.EpochValidFrom) || profile.notAfter.After(s.current.ValidUntil) || !matchesClosedProfileCandidates(profile, s.currentDecision.verified.accepted) {
		return ClosedProfileView{}, errors.New("closed profile does not match accepted State")
	}
	stored, storedRaw, err := s.storage.loadClosedProfile(generation)
	if err != nil {
		return ClosedProfileView{}, err
	}
	if stored != (closedProfileState{}) {
		if stored.epoch != profile.epoch || stored.accepted != profile.digest || stored.conflict != [32]byte{} {
			if stored.conflict == [32]byte{} && stored.epoch == profile.epoch && stored.accepted != profile.digest {
				stored.conflict = profile.digest
				if err := s.storage.commitClosedProfile(stored, storedRaw); err != nil {
					return ClosedProfileView{}, err
				}
			}
			return ClosedProfileView{}, errors.New("closed profile has a durable conflict")
		}
	} else if err := s.storage.commitClosedProfile(closedProfileState{generation: generation, epoch: profile.epoch, accepted: profile.digest}, raw); err != nil {
		return ClosedProfileView{}, err
	}
	return closedProfileView(profile), nil
}

// CurrentClosedProfile returns the one durably accepted profile only while it
// still joins the current authenticated closed Route Epoch. A State successor,
// conflict, expiry, or missing persisted profile is unavailable rather than a
// caller-selected fallback.
func (s *networkState) CurrentClosedProfile() (ClosedProfileView, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed || s.current == nil || s.currentDecision == nil || s.current.Profile != closedRouteProfile || s.distribution.conflicting {
		return ClosedProfileView{}, errors.New("closed profile is unavailable")
	}
	generation, err := closedProfileGeneration(s.current.Generation)
	if err != nil {
		return ClosedProfileView{}, err
	}
	stored, raw, err := s.storage.loadClosedProfile(generation)
	if err != nil || stored == (closedProfileState{}) || stored.conflict != [32]byte{} || stored.epoch != s.current.Epoch {
		return ClosedProfileView{}, errors.New("closed profile is unavailable")
	}
	now := s.config.clock().UTC()
	profile, err := parseClosedProfile(raw, generation, s.current.NetworkID, s.current.Digest, s.current.Epoch, s.config.closedProfileAuthority, now)
	if err != nil || profile.digest != stored.accepted || profile.notBefore.Before(s.current.EpochValidFrom) || profile.notAfter.After(s.current.ValidUntil) ||
		!matchesClosedProfileCandidates(profile, s.currentDecision.verified.accepted) {
		return ClosedProfileView{}, errors.New("closed profile is unavailable")
	}
	return closedProfileView(profile), nil
}

// CurrentClosedRoute returns only recipient constraints from the same durable
// accepted profile while it still joins the current closed Route Epoch. A
// successor, conflict, expiry, or missing profile is unavailable; callers
// cannot retain an old route projection or manufacture a recipient.
func (s *networkState) CurrentClosedRoute() (ClosedRouteView, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed || s.current == nil || s.currentDecision == nil || s.current.Profile != closedRouteProfile || s.distribution.conflicting {
		return ClosedRouteView{}, errors.New("closed route is unavailable")
	}
	generation, err := closedProfileGeneration(s.current.Generation)
	if err != nil {
		return ClosedRouteView{}, err
	}
	stored, raw, err := s.storage.loadClosedProfile(generation)
	if err != nil || stored == (closedProfileState{}) || stored.conflict != [32]byte{} || stored.epoch != s.current.Epoch {
		return ClosedRouteView{}, errors.New("closed route is unavailable")
	}
	now := s.config.clock().UTC()
	profile, err := parseClosedProfile(raw, generation, s.current.NetworkID, s.current.Digest, s.current.Epoch, s.config.closedProfileAuthority, now)
	if err != nil || profile.digest != stored.accepted || profile.notBefore.Before(s.current.EpochValidFrom) || profile.notAfter.After(s.current.ValidUntil) ||
		!matchesClosedProfileCandidates(profile, s.currentDecision.verified.accepted) {
		return ClosedRouteView{}, errors.New("closed route is unavailable")
	}
	return closedRouteView(profile), nil
}

func closedProfileView(profile closedProfile) ClosedProfileView {
	view := ClosedProfileView{NetworkID: profile.networkID, StateGeneration: profile.stateGeneration, StateDigest: profile.epochDigest,
		Digest: profile.digest, IssuanceAuthorityKey: profile.authorityKey, IssuerNodeID: profile.issuerNodeID,
		Epoch: profile.epoch, NotBefore: profile.notBefore, NotAfter: profile.notAfter, TokenKeyCount: uint8(len(profile.keys))}
	for _, node := range profile.nodes {
		if node.nodeID == profile.issuerNodeID {
			view.IssuerDutyGeneration = node.generation
			break
		}
	}
	for index, key := range profile.keys {
		view.TokenKeys[index].WindowStart = time.Unix(int64(key.windowStart), 0).UTC()
		view.TokenKeys[index].Class = key.class
		copy(view.TokenKeys[index].SPKI[:], key.spki)
	}
	return view
}

func closedRouteView(profile closedProfile) ClosedRouteView {
	view := ClosedRouteView{Profile: closedProfileView(profile), NodeCount: uint8(len(profile.nodes))}
	for index, node := range profile.nodes {
		view.Nodes[index] = ClosedRouteNodeView{NodeID: node.nodeID, RecordDigest: node.recordDigest,
			RoleDomain: node.domain, Subrole: node.subrole, DutyGeneration: node.generation}
	}
	return view
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

func matchesClosedProfileCandidates(profile closedProfile, records []nodeRecord) bool {
	available := make(map[[32]byte]nodeRecord, len(records))
	for _, record := range records {
		available[record.nodeID] = record
	}
	for _, node := range profile.nodes {
		record, exists := available[node.nodeID]
		if !exists || record.generation != node.generation || recordDigest(record) != node.recordDigest || !validCarrierForEpoch(closedRouteProfile, record.carrier) {
			return false
		}
	}
	return true
}

func recordDigest(record nodeRecord) [32]byte { return sha256.Sum256(record.raw) }
