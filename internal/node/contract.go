package node

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"time"

	"github.com/dianabuilds/ardents-network/internal/resource"
	"github.com/dianabuilds/ardents-network/internal/route"
)

const eventSchema = "ardents-node-event-v1"

// DutyView is the narrow authenticated input required to decide one Node duty.
// It does not expose Network State persistence, source, retry, or pending
// metadata.
type DutyView interface {
	DutyGeneration() string
	DutyNetworkID() [32]byte
	DutyEpoch() uint64
	DutyDigest() [32]byte
	DutyEpochValidFrom() time.Time
	DutyValidUntil() time.Time
	DutyProfile() string
	DutyFresh() bool
	DutyConflicting() bool
	DutyRecordPresent() bool
	DutyNodeID() [32]byte
	DutyNodePublicKey() [32]byte
	DutyRecordValidFrom() time.Time
	DutyRecordValidUntil() time.Time
	DutyDeclaredFamily() string
	DutyProbeEndpoint() string
	DutyProbeCapacity() uint16
	DutyAssignment() string
	DutyAssignmentDigest() [32]byte
	DutyCandidateCount() uint8
	DutyCandidateNodeID(uint8) [32]byte
	DutyCandidatePublicKey(uint8) [32]byte
	DutyCandidateEndpoint(uint8) string
	DutyCandidateAssignment(uint8) string
	DutyCandidateValidFrom(uint8) time.Time
	DutyCandidateValidUntil(uint8) time.Time
}

// dutyFacts is the Node-owned immutable copy of one DutyView. It is also useful to
// behavior-test the lifecycle without a Network State runtime.
type dutyFacts struct {
	Generation       string
	NetworkID        [32]byte
	Epoch            uint64
	Digest           [32]byte
	EpochValidFrom   time.Time
	ValidUntil       time.Time
	Profile          string
	Conflicting      bool
	RecordPresent    bool
	NodeID           [32]byte
	NodePublicKey    [32]byte
	RecordValidFrom  time.Time
	RecordValidUntil time.Time
	DeclaredFamily   string
	ProbeEndpoint    string
	ProbeCapacity    uint16
	Assignment       string
	AssignmentDigest [32]byte
	Fresh            bool
	Candidates       [64]dutyCandidate
	CandidateCount   uint8
}

// dutyCandidate is one narrow State-authorized peer fact. It deliberately
// contains no source, address history, target, or complete route material.
type dutyCandidate struct {
	NodeID, PublicKey     [32]byte
	Endpoint, Assignment  string
	ValidFrom, ValidUntil time.Time
}

// Config binds one local identity, authenticated duty facts, and private role-probe listener.
type Config struct {
	NetworkID          [32]byte
	NodeID             [32]byte
	IdentityKey        ed25519.PrivateKey
	Current            func() (DutyView, error)
	Probe              ProbeConfig
	Rendezvous         RendezvousProfile
	Initiator          InitiatorProfile
	PollInterval       time.Duration
	Quarantine         time.Duration
	ResourceProfile    string
	LocalRoleStateRoot string
	// ResourceMeasure and CheckPlacement are behavior-test seams. Maintained
	// runtime callers leave them nil and use ResourceProfile's platform adapter.
	ResourceMeasure func() (resource.Sample, error)
	Now             func() time.Time
	CheckPlacement  func() error
	// Emit must honor ctx cancellation and return before its deadline.
	Emit func(context.Context, Event) error
}

// RendezvousProfile contains the local cryptographic material and explicit
// finite reservations used to materialize one State-assigned Rendezvous duty.
// State supplies the listener, epoch, peers, identities, and expiry; this
// profile never discovers or selects them.
type RendezvousProfile struct {
	Certificate                             tls.Certificate
	HandshakeLimit, WaitingLimit, PairLimit uint16
	PairByteLimit                           uint64
	DrainTimeout                            time.Duration
}

// InitiatorProfile contains the local cryptographic material, bounded
// reservations, and narrow Entry admission port for one Initiator duty. State
// supplies its endpoint, Rendezvous identity, and expiry.
type InitiatorProfile struct {
	Certificate                tls.Certificate
	Admit                      route.EntryBindingAdmitter
	HandshakeLimit, RelayLimit uint16
	RelayByteLimit             uint64
	DrainTimeout               time.Duration
}

func (facts dutyFacts) DutyGeneration() string          { return facts.Generation }
func (facts dutyFacts) DutyNetworkID() [32]byte         { return facts.NetworkID }
func (facts dutyFacts) DutyEpoch() uint64               { return facts.Epoch }
func (facts dutyFacts) DutyDigest() [32]byte            { return facts.Digest }
func (facts dutyFacts) DutyEpochValidFrom() time.Time   { return facts.EpochValidFrom }
func (facts dutyFacts) DutyValidUntil() time.Time       { return facts.ValidUntil }
func (facts dutyFacts) DutyProfile() string             { return facts.Profile }
func (facts dutyFacts) DutyFresh() bool                 { return facts.Fresh }
func (facts dutyFacts) DutyConflicting() bool           { return facts.Conflicting }
func (facts dutyFacts) DutyRecordPresent() bool         { return facts.RecordPresent }
func (facts dutyFacts) DutyNodeID() [32]byte            { return facts.NodeID }
func (facts dutyFacts) DutyNodePublicKey() [32]byte     { return facts.NodePublicKey }
func (facts dutyFacts) DutyRecordValidFrom() time.Time  { return facts.RecordValidFrom }
func (facts dutyFacts) DutyRecordValidUntil() time.Time { return facts.RecordValidUntil }
func (facts dutyFacts) DutyDeclaredFamily() string      { return facts.DeclaredFamily }
func (facts dutyFacts) DutyProbeEndpoint() string       { return facts.ProbeEndpoint }
func (facts dutyFacts) DutyProbeCapacity() uint16       { return facts.ProbeCapacity }
func (facts dutyFacts) DutyAssignment() string          { return facts.Assignment }
func (facts dutyFacts) DutyAssignmentDigest() [32]byte  { return facts.AssignmentDigest }
func (facts dutyFacts) DutyCandidateCount() uint8       { return facts.CandidateCount }
func (facts dutyFacts) DutyCandidateNodeID(index uint8) [32]byte {
	if index >= facts.CandidateCount {
		return [32]byte{}
	}
	return facts.Candidates[index].NodeID
}
func (facts dutyFacts) DutyCandidatePublicKey(index uint8) [32]byte {
	if index >= facts.CandidateCount {
		return [32]byte{}
	}
	return facts.Candidates[index].PublicKey
}
func (facts dutyFacts) DutyCandidateEndpoint(index uint8) string {
	if index >= facts.CandidateCount {
		return ""
	}
	return facts.Candidates[index].Endpoint
}
func (facts dutyFacts) DutyCandidateAssignment(index uint8) string {
	if index >= facts.CandidateCount {
		return ""
	}
	return facts.Candidates[index].Assignment
}
func (facts dutyFacts) DutyCandidateValidFrom(index uint8) time.Time {
	if index >= facts.CandidateCount {
		return time.Time{}
	}
	return facts.Candidates[index].ValidFrom
}
func (facts dutyFacts) DutyCandidateValidUntil(index uint8) time.Time {
	if index >= facts.CandidateCount {
		return time.Time{}
	}
	return facts.Candidates[index].ValidUntil
}

// Event is one bounded external observation of Node lifecycle state.
type Event struct {
	Schema           string           `json:"schema"`
	Kind             string           `json:"kind"`
	State            string           `json:"state"`
	At               time.Time        `json:"at"`
	Epoch            uint64           `json:"epoch,omitempty"`
	Generation       string           `json:"generation,omitempty"`
	Assignment       string           `json:"assignment,omitempty"`
	AssignmentDigest [32]byte         `json:"assignment_digest,omitempty"`
	Reason           string           `json:"reason,omitempty"`
	Resource         *resource.Sample `json:"resource,omitempty"`
}

// Result describes the terminal state after the listener and accepted work are gone.
type Result struct {
	State            string
	Epoch            uint64
	Assignment       string
	AssignmentDigest [32]byte
	Reason           string
}

type runtimeConfig struct {
	Config
	now      func() time.Time
	probe    *probePlan
	pressure *resource.Guard
}
