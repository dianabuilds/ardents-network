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

// DestinationResolutionGateway is the one State-selected Gateway fact for a
// private Target lookup. Its Profile bytes are authenticated by the Epoch but
// remain Reachability-owned until the Endpoint verifies their self-signature.
type DestinationResolutionGateway struct {
	NodeID, PublicKey, Family [32]byte
	Profile                   []byte
	AssignmentNotAfter        time.Time
}

// TransitIssuer is the one State-selected membership Transit Grant issuer
// fact. Its Profile remains opaque until the credential owner verifies its
// Node self-signature and purpose-scoped Grant signer declaration.
type TransitIssuer struct {
	NodeID, PublicKey, Family [32]byte
	Profile                   []byte
	AssignmentNotAfter        time.Time
}

// PublisherTransitPeer is one exact authenticated transit attachment selected
// for a Publisher. Family is the opaque family identity used by the local
// acquisition owner; no domain name or candidate ordering escapes State.
type PublisherTransitPeer struct {
	NodeID, PublicKey, Family [32]byte
	Endpoint                  string
}

// PublisherAttachment is the complete State projection for one Publisher
// lifecycle. It is available only as one indivisible Introduction,
// Rendezvous, and Responder triple.
type PublisherAttachment struct {
	NetworkID, Digest                   [32]byte
	Epoch                               uint64
	Introduction, Rendezvous, Responder PublisherTransitPeer
	NotAfter                            time.Time
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

// Gateway returns the exact Destination Resolution Gateway only when State is
// fresh and that candidate remains valid throughout the complete lookup
// window. It contains no candidate ordering, alternate Gateway, or URL.
func (view ResolutionView) Gateway(at, deadline time.Time) (DestinationResolutionGateway, bool) {
	if _, available := view.Epoch(at, deadline); !available {
		return DestinationResolutionGateway{}, false
	}
	snapshot := view.snapshot
	if snapshot.Profile != interactiveRouteProfile || snapshot.DestinationResolutionNodeID == [32]byte{} ||
		snapshot.DestinationResolutionProfileSize == 0 || int(snapshot.DestinationResolutionProfileSize) > len(snapshot.DestinationResolutionProfile) {
		return DestinationResolutionGateway{}, false
	}
	for _, candidate := range snapshot.Candidates[:snapshot.CandidateCount] {
		if candidate.NodeID != snapshot.DestinationResolutionNodeID || candidate.Domain != destinationResolutionDomain {
			continue
		}
		valid := candidate.Capacity > 0 && candidate.PublicKey != [32]byte{} && candidate.FamilyID != [32]byte{} &&
			!at.Before(candidate.ValidFrom) && deadline.Before(candidate.ValidUntil) && !candidate.AssignmentNotAfter.Before(deadline)
		if !valid {
			return DestinationResolutionGateway{}, false
		}
		profile := make([]byte, snapshot.DestinationResolutionProfileSize)
		copy(profile, snapshot.DestinationResolutionProfile[:snapshot.DestinationResolutionProfileSize])
		return DestinationResolutionGateway{NodeID: candidate.NodeID, PublicKey: candidate.PublicKey, Family: candidate.FamilyID,
			Profile: profile, AssignmentNotAfter: candidate.AssignmentNotAfter}, true
	}
	return DestinationResolutionGateway{}, false
}

// CredentialIssuer returns the sole current State-selected issuer for the
// closed membership-level Transit Grant operation. There is no candidate list,
// endpoint literal, caller override, or fallback.
func (view ResolutionView) CredentialIssuer(at, deadline time.Time) (TransitIssuer, bool) {
	if _, available := view.Epoch(at, deadline); !available {
		return TransitIssuer{}, false
	}
	snapshot := view.snapshot
	if snapshot.Profile != interactiveRouteProfile || snapshot.TransitIssuanceNodeID == [32]byte{} ||
		snapshot.TransitIssuanceProfileSize == 0 || int(snapshot.TransitIssuanceProfileSize) > len(snapshot.TransitIssuanceProfile) {
		return TransitIssuer{}, false
	}
	for _, candidate := range snapshot.Candidates[:snapshot.CandidateCount] {
		if candidate.NodeID != snapshot.TransitIssuanceNodeID || candidate.Domain != transitIssuanceDomain {
			continue
		}
		valid := candidate.Capacity > 0 && candidate.PublicKey != [32]byte{} && candidate.FamilyID != [32]byte{} &&
			!at.Before(candidate.ValidFrom) && deadline.Before(candidate.ValidUntil) && !candidate.AssignmentNotAfter.Before(deadline)
		if !valid {
			return TransitIssuer{}, false
		}
		profile := make([]byte, snapshot.TransitIssuanceProfileSize)
		copy(profile, snapshot.TransitIssuanceProfile[:snapshot.TransitIssuanceProfileSize])
		return TransitIssuer{NodeID: candidate.NodeID, PublicKey: candidate.PublicKey, Family: candidate.FamilyID,
			Profile: profile, AssignmentNotAfter: candidate.AssignmentNotAfter}, true
	}
	return TransitIssuer{}, false
}

// PublisherAttachment returns exactly one current candidate for every
// Publisher transit role. A missing or repeated role, a conflicting State, or
// a candidate that does not cover the complete requested window makes the
// indivisible projection unavailable.
func (view ResolutionView) PublisherAttachment(at, deadline time.Time) (PublisherAttachment, bool) {
	epoch, available := view.Epoch(at, deadline)
	if !available || view.snapshot.Profile != interactiveRouteProfile {
		return PublisherAttachment{}, false
	}
	selected := make(map[string]PublisherTransitPeer, 3)
	for _, candidate := range view.snapshot.Candidates[:view.snapshot.CandidateCount] {
		switch candidate.Domain {
		case "introduction", "rendezvous", "responder":
		default:
			continue
		}
		if _, repeated := selected[candidate.Domain]; repeated || candidate.Capacity == 0 || candidate.NodeID == [32]byte{} ||
			candidate.PublicKey == [32]byte{} || candidate.FamilyID == [32]byte{} || candidate.Endpoint == "" ||
			at.Before(candidate.ValidFrom) || !deadline.Before(candidate.ValidUntil) ||
			candidate.AssignmentNotAfter.Before(deadline) {
			return PublisherAttachment{}, false
		}
		selected[candidate.Domain] = PublisherTransitPeer{NodeID: candidate.NodeID, PublicKey: candidate.PublicKey,
			Family: candidate.FamilyID, Endpoint: candidate.Endpoint}
	}
	introduction, introductionOK := selected["introduction"]
	rendezvous, rendezvousOK := selected["rendezvous"]
	responder, responderOK := selected["responder"]
	if !introductionOK || !rendezvousOK || !responderOK || !distinctPublisherAttachmentPeers(introduction, rendezvous, responder) {
		return PublisherAttachment{}, false
	}
	return PublisherAttachment{NetworkID: epoch.NetworkID, Digest: epoch.Digest, Epoch: epoch.Number,
		Introduction: introduction, Rendezvous: rendezvous, Responder: responder, NotAfter: deadline}, true
}

func distinctPublisherAttachmentPeers(peers ...PublisherTransitPeer) bool {
	for index, peer := range peers {
		for prior := 0; prior < index; prior++ {
			if peer.NodeID == peers[prior].NodeID || peer.Family == peers[prior].Family {
				return false
			}
		}
	}
	return true
}
