package main

import (
	"crypto/sha256"
	"time"

	endpointapi "github.com/dianabuilds/ardents-network/internal/endpoint"
	"github.com/dianabuilds/ardents-network/internal/node"
	"github.com/dianabuilds/ardents-network/internal/route"
)

// fixtureDutyView is a test-owned substitute for an already authenticated
// current-State projection. It permits the process tracer to exercise node.Run's
// native lifecycle, durable Entry/grant admission, and State-selected peers;
// it is not a State source or participant configuration.
type fixtureDutyView struct {
	generation                         string
	network, digest, nodeID, nodeKey   [32]byte
	epoch                              uint64
	validFrom, validUntil              time.Time
	endpoint, assignment, family       string
	assignmentDigest, authorityID, key [32]byte
	candidates                         []fixtureDutyCandidate
}

type fixtureDutyCandidate struct {
	nodeID, keyID, public, familyID, record, proof [32]byte
	endpoint, assignment                           string
}

func newFixtureDutyView(input config, local endpointapi.TransitPeer, assignment string, peers []fixtureDutyCandidate) (node.DutyView, error) {
	network, err := fixed(input.Network)
	if err != nil {
		return nil, err
	}
	digest, err := fixed(input.Digest)
	if err != nil {
		return nil, err
	}
	authority, err := fixed(input.TransitAuthority)
	if err != nil {
		return nil, err
	}
	deadline, err := input.deadline()
	if err != nil {
		return nil, err
	}
	validFrom := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	return fixtureDutyView{generation: "fixture-current", network: network, digest: digest, epoch: input.Epoch,
		nodeID: local.NodeID, nodeKey: local.PublicKey, validFrom: validFrom, validUntil: deadline, endpoint: local.Endpoint,
		assignment: assignment, family: "fixture-" + assignment, assignmentDigest: fixtureDutyDigest("assignment", local.NodeID),
		authorityID: sha256.Sum256(authority[:]), key: authority, candidates: peers}, nil
}

func fixtureCandidate(peer endpointapi.TransitPeer, assignment string) fixtureDutyCandidate {
	return fixtureDutyCandidate{nodeID: peer.NodeID, public: peer.PublicKey, endpoint: peer.Endpoint, assignment: assignment,
		keyID: fixtureDutyDigest("key", peer.NodeID), familyID: peer.Family, record: fixtureDutyDigest("record", peer.NodeID),
		proof: fixtureDutyDigest("proof", peer.NodeID)}
}

func fixtureDutyDigest(prefix string, value [32]byte) [32]byte {
	return sha256.Sum256(append(append([]byte("reference-c2-"), []byte(prefix)...), value[:]...))
}

func (view fixtureDutyView) DutyGeneration() string          { return view.generation }
func (view fixtureDutyView) DutyNetworkID() [32]byte         { return view.network }
func (view fixtureDutyView) DutyEpoch() uint64               { return view.epoch }
func (view fixtureDutyView) DutyDigest() [32]byte            { return view.digest }
func (view fixtureDutyView) DutyEpochValidFrom() time.Time   { return view.validFrom }
func (view fixtureDutyView) DutyValidUntil() time.Time       { return view.validUntil }
func (view fixtureDutyView) DutyProfile() string             { return route.Profile }
func (view fixtureDutyView) DutyFresh() bool                 { return true }
func (view fixtureDutyView) DutyConflicting() bool           { return false }
func (view fixtureDutyView) DutyRecordPresent() bool         { return true }
func (view fixtureDutyView) DutyNodeID() [32]byte            { return view.nodeID }
func (view fixtureDutyView) DutyNodePublicKey() [32]byte     { return view.nodeKey }
func (view fixtureDutyView) DutyRecordValidFrom() time.Time  { return view.validFrom }
func (view fixtureDutyView) DutyRecordValidUntil() time.Time { return view.validUntil }
func (view fixtureDutyView) DutyDeclaredFamily() string      { return view.family }
func (view fixtureDutyView) DutyProbeEndpoint() string       { return view.endpoint }
func (view fixtureDutyView) DutyProbeCapacity() uint16       { return 1 }
func (view fixtureDutyView) DutyAssignment() string          { return view.assignment }
func (view fixtureDutyView) DutyAssignmentDigest() [32]byte  { return view.assignmentDigest }
func (view fixtureDutyView) DutyCandidateCount() uint8       { return uint8(len(view.candidates)) }
func (view fixtureDutyView) DutyAuthorityCount() uint8       { return 1 }
func (view fixtureDutyView) DutyAuthorityID(index uint8) [32]byte {
	if index != 0 {
		return [32]byte{}
	}
	return view.authorityID
}
func (view fixtureDutyView) DutyAuthorityPublicKey(index uint8) [32]byte {
	if index != 0 {
		return [32]byte{}
	}
	return view.key
}
func (view fixtureDutyView) candidate(index uint8) fixtureDutyCandidate {
	if int(index) >= len(view.candidates) {
		return fixtureDutyCandidate{}
	}
	return view.candidates[index]
}
func (view fixtureDutyView) DutyCandidateNodeID(index uint8) [32]byte {
	return view.candidate(index).nodeID
}
func (view fixtureDutyView) DutyCandidatePublicKey(index uint8) [32]byte {
	return view.candidate(index).public
}
func (view fixtureDutyView) DutyCandidateKeyID(index uint8) [32]byte {
	return view.candidate(index).keyID
}
func (view fixtureDutyView) DutyCandidateFamilyID(index uint8) [32]byte {
	return view.candidate(index).familyID
}
func (view fixtureDutyView) DutyCandidateRecordDigest(index uint8) [32]byte {
	return view.candidate(index).record
}
func (view fixtureDutyView) DutyCandidateDomainProofDigest(index uint8) [32]byte {
	return view.candidate(index).proof
}
func (view fixtureDutyView) DutyCandidateEndpoint(index uint8) string {
	return view.candidate(index).endpoint
}
func (view fixtureDutyView) DutyCandidateCapacity(index uint8) uint16 {
	if view.candidate(index).nodeID == [32]byte{} {
		return 0
	}
	return 1
}
func (view fixtureDutyView) DutyCandidateAssignment(index uint8) string {
	return view.candidate(index).assignment
}
func (view fixtureDutyView) DutyCandidateValidFrom(index uint8) time.Time  { return view.validFrom }
func (view fixtureDutyView) DutyCandidateValidUntil(index uint8) time.Time { return view.validUntil }
func (view fixtureDutyView) DutyCandidateAssignmentNotAfter(index uint8) time.Time {
	return view.validUntil
}
