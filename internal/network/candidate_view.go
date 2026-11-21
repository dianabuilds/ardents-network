package network

import (
	"bytes"
	"errors"
	"sort"
	"time"
)

// RecordAuthentication describes the format owner's examination of one input.
// The zero value is malformed. None of these values admits Network State.
type RecordAuthentication uint8

const (
	MalformedRecord RecordAuthentication = iota
	InvalidRecordSignature
	AuthenticatedRecord
)

// CandidateInput contains decoded public facts and the authentication result.
// Malformed inputs need no facts; a decoded input retains its Network identity
// even when its signature fails, preserving the committed rejection precedence.
type CandidateInput struct {
	Authentication            RecordAuthentication
	Network, Node, Key        [32]byte
	ValidFrom, ValidUntil     time.Time
	Family, Endpoint, Carrier string
	Capability                uint8
	Capacity                  uint16
}

// CandidatePolicy is the authenticated Epoch's eligibility context. Retained
// profile names are compatibility inputs, not additional accepting modes.
type CandidatePolicy struct {
	Network         [32]byte
	ValidFrom       time.Time
	Profile         string
	AuthorityKeyIDs [][32]byte
}

const (
	candidateMalformed          = uint16(1)
	candidateWrongNetwork       = uint16(2)
	candidateInvalidSignature   = uint16(3)
	candidateTime               = uint16(4)
	candidateCapability         = uint16(5)
	candidateCapacity           = uint16(6)
	candidateAuthorityCollision = uint16(7)
	candidateNodeCollision      = uint16(8)
	candidateKeyCollision       = uint16(10)
	candidateEndpointCollision  = uint16(11)
	candidateCarrier            = uint16(12)
)

// CandidateRejection identifies an input and its retained wire reason. Reason 9
// remains reserved: duplicate Nodes take precedence over duplicate generations.
type CandidateRejection struct {
	Index  uint32
	Reason uint16
}

// CandidateSummary describes the accepted View independently of its encoding.
type CandidateSummary struct {
	Count, Capacity             uint32
	FamilyCount, MaxFamilyCount uint16
	MaxFamilyCapacity           uint32
}

// CandidateView owns deterministic input disposition and family totals. It
// does not own raw bytes, signatures, Merkle proofs or current State authority.
type CandidateView struct {
	accepted []int
	rejected []CandidateRejection
	families map[string][2]uint32
	summary  CandidateSummary
}

// EvaluateCandidates rejects individual inputs before counting collisions.
// Every otherwise eligible member of a collision is excluded; input order
// cannot choose a winner. Accepted indices are ordered by Node identity.
func EvaluateCandidates(policy CandidatePolicy, inputs []CandidateInput) (CandidateView, error) {
	if len(inputs) > 64 || policy.Network == [32]byte{} || policy.ValidFrom.IsZero() {
		return CandidateView{}, errors.New("candidate context exceeds its bounds or has no identity")
	}
	var capability uint8
	switch policy.Profile {
	case "h3-role-probe-v1":
		capability = 1
	case "ardents-interactive-route-v2", "ardents-route-v3":
		capability = 2
	default:
		return CandidateView{}, errors.New("candidate profile is unknown")
	}
	authorities := make(map[[32]byte]bool, len(policy.AuthorityKeyIDs))
	for _, key := range policy.AuthorityKeyIDs {
		authorities[key] = true
	}
	reasons := make([]uint16, len(inputs))
	nodes, keys, endpoints := make(map[[32]byte]int), make(map[[32]byte]int), make(map[string]int)
	for index, input := range inputs {
		switch {
		case input.Authentication != AuthenticatedRecord && input.Authentication != InvalidRecordSignature:
			reasons[index] = candidateMalformed
		case input.Network != policy.Network:
			reasons[index] = candidateWrongNetwork
		case input.Authentication != AuthenticatedRecord:
			reasons[index] = candidateInvalidSignature
		case policy.ValidFrom.Before(input.ValidFrom) || !policy.ValidFrom.Before(input.ValidUntil):
			reasons[index] = candidateTime
		case input.Capability != capability:
			reasons[index] = candidateCapability
		case input.Capacity == 0 || input.Capacity > 1024:
			reasons[index] = candidateCapacity
		case !CandidateCarrierEligible(policy.Profile, input.Carrier):
			reasons[index] = candidateCarrier
		case authorities[input.Key]:
			reasons[index] = candidateAuthorityCollision
		default:
			nodes[input.Node]++
			keys[input.Key]++
			endpoints[input.Endpoint]++
		}
	}
	view := CandidateView{families: make(map[string][2]uint32)}
	for index, input := range inputs {
		if reasons[index] == 0 {
			switch {
			case nodes[input.Node] > 1:
				reasons[index] = candidateNodeCollision
			case keys[input.Key] > 1:
				reasons[index] = candidateKeyCollision
			case endpoints[input.Endpoint] > 1:
				reasons[index] = candidateEndpointCollision
			}
		}
		if reasons[index] != 0 {
			view.rejected = append(view.rejected, CandidateRejection{Index: uint32(index), Reason: reasons[index]})
			continue
		}
		view.accepted = append(view.accepted, index)
		summary := view.families[input.Family]
		summary[0]++
		summary[1] += uint32(input.Capacity)
		view.families[input.Family] = summary
		view.summary.Capacity += uint32(input.Capacity)
	}
	sort.Slice(view.accepted, func(i, j int) bool {
		return bytes.Compare(inputs[view.accepted[i]].Node[:], inputs[view.accepted[j]].Node[:]) < 0
	})
	view.summary.Count, view.summary.FamilyCount = uint32(len(view.accepted)), uint16(len(view.families))
	for _, family := range view.families {
		view.summary.MaxFamilyCount = max(view.summary.MaxFamilyCount, uint16(family[0]))
		view.summary.MaxFamilyCapacity = max(view.summary.MaxFamilyCapacity, family[1])
	}
	return view, nil
}

func (view CandidateView) AcceptedIndices() []int { return append([]int(nil), view.accepted...) }
func (view CandidateView) Rejections() []CandidateRejection {
	return append([]CandidateRejection(nil), view.rejected...)
}
func (view CandidateView) Summary() CandidateSummary { return view.summary }

// CandidateCarrierEligible preserves the selected closed Carrier set. Earlier
// Carrier identities are understood only for retained profile interpretation.
func CandidateCarrierEligible(profile, carrier string) bool {
	if profile == "ardents-route-v3" {
		return carrier == "ardents-carrier-tcp-tls-v2" || carrier == "ardents-carrier-quic-v2"
	}
	return carrier == "ardents-carrier-tcp-tls-v1" || carrier == "ardents-carrier-quic-v1" || carrier == "ardents-carrier-tcp-tls-v2" || carrier == "ardents-carrier-quic-v2"
}
