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
	Digest, IssuanceAuthorityKey [32]byte
	IssuerNodeID                 [32]byte
	Epoch                        uint64
	NotBefore, NotAfter          time.Time
	TokenKeyCount                uint8
	TokenKeys                    [maximumClosedProfileKeys]ClosedProfileTokenKey
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

func closedProfileView(profile closedProfile) ClosedProfileView {
	view := ClosedProfileView{Digest: profile.digest, IssuanceAuthorityKey: profile.authorityKey, IssuerNodeID: profile.issuerNodeID,
		Epoch: profile.epoch, NotBefore: profile.notBefore, NotAfter: profile.notAfter, TokenKeyCount: uint8(len(profile.keys))}
	for index, key := range profile.keys {
		view.TokenKeys[index].WindowStart = time.Unix(int64(key.windowStart), 0).UTC()
		view.TokenKeys[index].Class = key.class
		copy(view.TokenKeys[index].SPKI[:], key.spki)
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
