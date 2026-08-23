package state

import (
	"errors"
	"time"
)

// ResolutionView is the authenticated Network State projection consumed by
// private naming Resolution. It keeps freshness, selection-window, and
// candidate validity interpretation inside State.
type ResolutionView struct{ snapshot Snapshot }

// ResolutionEpoch is the bounded epoch fact needed to bind one Resolution
// plan and construct its Namespace proof verifier.
type ResolutionEpoch struct {
	Generation  string
	NetworkID   [32]byte
	Number      uint64
	Digest      [32]byte
	ViewRoot    [32]byte
	Authorities []ResolutionAuthority
	Threshold   uint8
}

// ResolutionAuthority is one threshold key from the authenticated Epoch.
type ResolutionAuthority struct {
	ID        [32]byte
	PublicKey [32]byte
}

// ResolutionCandidate is one authenticated candidate that remains valid for
// the requested complete Resolution window.
type ResolutionCandidate struct {
	NodeID             [32]byte
	PublicKey          [32]byte
	Family             string
	Endpoint           string
	Domain             string
	AssignmentNotAfter time.Time
}

// Resolution returns the current Snapshot as a private-Resolution view. It
// rejects malformed Epoch trust state before a consumer can derive a policy.
func (snapshot Snapshot) Resolution() (ResolutionView, error) {
	if snapshot.Generation == "" || snapshot.NetworkID == [32]byte{} || snapshot.Epoch == 0 ||
		snapshot.Digest == [32]byte{} || snapshot.ViewRoot == [32]byte{} ||
		snapshot.EpochAuthorityCount == 0 || snapshot.EpochAuthorityCount > 16 ||
		snapshot.EpochThreshold == 0 || snapshot.EpochThreshold > snapshot.EpochAuthorityCount {
		return ResolutionView{}, errors.New("network state resolution view is invalid")
	}
	return ResolutionView{snapshot: snapshot}, nil
}

// CurrentResolution returns one current authenticated Resolution view.
func (s *networkState) CurrentResolution() (ResolutionView, error) {
	snapshot, err := s.Current()
	if err != nil {
		return ResolutionView{}, err
	}
	return snapshot.Resolution()
}

// Epoch returns the immutable epoch fact only when State is fresh and the
// exact requested window lies within the authenticated validity interval.
func (view ResolutionView) Epoch(at, deadline time.Time) (ResolutionEpoch, bool) {
	snapshot := view.snapshot
	if snapshot.Freshness != "fresh" || snapshot.Conflicting || at.IsZero() || !at.Before(deadline) ||
		deadline.After(at.Add(15*time.Second)) || deadline.After(snapshot.ValidUntil) {
		return ResolutionEpoch{}, false
	}
	authorities := make([]ResolutionAuthority, snapshot.EpochAuthorityCount)
	for index := range authorities {
		authorities[index] = ResolutionAuthority{ID: snapshot.EpochAuthorityIDs[index], PublicKey: snapshot.EpochAuthorityKeys[index]}
	}
	return ResolutionEpoch{Generation: snapshot.Generation, NetworkID: snapshot.NetworkID, Number: snapshot.Epoch,
		Digest: snapshot.Digest, ViewRoot: snapshot.ViewRoot, Authorities: authorities, Threshold: snapshot.EpochThreshold}, true
}

// Candidate returns the exact authenticated candidate valid throughout the
// requested Resolution window.
func (view ResolutionView) Candidate(nodeID [32]byte, at, deadline time.Time) (ResolutionCandidate, bool) {
	if nodeID == [32]byte{} {
		return ResolutionCandidate{}, false
	}
	for _, candidate := range view.snapshot.Candidates[:view.snapshot.CandidateCount] {
		if candidate.NodeID != nodeID {
			continue
		}
		valid := candidate.Capacity > 0 && candidate.Family != "" && candidate.Endpoint != "" && candidate.Domain != "" &&
			!at.Before(candidate.ValidFrom) && deadline.Before(candidate.ValidUntil) && !candidate.AssignmentNotAfter.Before(deadline)
		return ResolutionCandidate{NodeID: candidate.NodeID, PublicKey: candidate.PublicKey, Family: candidate.Family,
			Endpoint: candidate.Endpoint, Domain: candidate.Domain, AssignmentNotAfter: candidate.AssignmentNotAfter}, valid
	}
	return ResolutionCandidate{}, false
}
