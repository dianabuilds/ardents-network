package node

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/resource"
)

const eventSchema = "ardents-node-event-v1"

// RendezvousDedicatedHostResourceProfile is the only selected native Node
// resource profile and is restricted to the dedicated Rendezvous duty.
const RendezvousDedicatedHostResourceProfile = resource.RendezvousDedicatedHostProfile

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
	DutyRecordGeneration() uint64
	DutyRecordValidFrom() time.Time
	DutyRecordValidUntil() time.Time
	DutyDeclaredFamily() string
	DutyProbeEndpoint() string
	DutyCarrierProfile() string
	DutyProbeCapacity() uint16
	DutyAssignment() string
	DutyAssignmentDigest() [32]byte
	DutyCandidateCount() uint8
	DutyCandidateNodeID(uint8) [32]byte
	DutyCandidatePublicKey(uint8) [32]byte
	DutyCandidateKeyID(uint8) [32]byte
	DutyCandidateFamilyID(uint8) [32]byte
	DutyCandidateRecordDigest(uint8) [32]byte
	DutyCandidateDomainProofDigest(uint8) [32]byte
	DutyCandidateEndpoint(uint8) string
	DutyCandidateCarrierProfile(uint8) string
	DutyCandidateCapacity(uint8) uint16
	DutyCandidateAssignment(uint8) string
	DutyCandidateValidFrom(uint8) time.Time
	DutyCandidateValidUntil(uint8) time.Time
	DutyCandidateAssignmentNotAfter(uint8) time.Time
	DutyAuthorityCount() uint8
	DutyAuthorityID(uint8) [32]byte
	DutyAuthorityPublicKey(uint8) [32]byte
}

// dutyFacts is the Node-owned immutable copy of one DutyView. Its DutyView
// projection is test scope (duty_facts_projection_test.go): production
// supplies state.NodeDutyView through Config.Current, while behavior tests
// supply this snapshot directly without a Network State runtime.
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
	RecordGeneration uint64
	RecordValidFrom  time.Time
	RecordValidUntil time.Time
	DeclaredFamily   string
	ProbeEndpoint    string
	CarrierProfile   string
	ProbeCapacity    uint16
	Assignment       string
	AssignmentDigest [32]byte
	Fresh            bool
	Candidates       [64]dutyCandidate
	CandidateCount   uint8
	Authorities      [16]dutyAuthority
	AuthorityCount   uint8
}

// dutyCandidate is one narrow State-authorized peer fact. It deliberately
// contains no source, address history, target, or complete route material.
type dutyCandidate struct {
	NodeID, PublicKey, KeyID, FamilyID, RecordDigest, DomainProofDigest [32]byte
	Endpoint, CarrierProfile, Assignment                                string
	Capacity                                                            uint16
	ValidFrom, ValidUntil, AssignmentNotAfter                           time.Time
}

// dutyAuthority is one current authenticated Network State verification key.
// It is copied only for Node-local Transit Grant verification.
type dutyAuthority struct{ ID, PublicKey [32]byte }

// Config binds one local identity, authenticated duty facts, and private role-probe listener.
type Config struct {
	// HostingRoot is the single installed provider period shared by all closed duties on this host.
	HostingRoot string
	// ClosedListenOverride is an optional private local bind address behind an
	// operator-owned relay. State remains authoritative for the advertised
	// endpoint used by every peer.
	ClosedListenOverride string
	NetworkID            [32]byte
	NodeID               [32]byte
	IdentityKey          ed25519.PrivateKey
	Current              func() (DutyView, error)
	Probe                ProbeConfig
	ClosedIssuer         ClosedIssuerProfile
	// ClosedForwarding supplies the isolated receiving spend journal and Node
	// TLS key for an accepted generation-3 adjacent/interior forwarding duty.
	// State still selects the endpoint, peer and recipient assignment.
	ClosedForwarding   ClosedForwardingProfile
	ClosedResolution   ClosedResolutionProfile
	ClosedIntroduction ClosedIntroductionProfile
	ClosedDataJoin     ClosedDataJoinProfile
	// CurrentClosedProfile exposes only State's already accepted closed
	// profile. It is unavailable instead of choosing profile bytes or a trust
	// root from the Node plan.
	CurrentClosedProfile func() (state.ClosedProfileView, bool)
	// CurrentClosedRoute exposes the same accepted profile's recipient facts.
	// It is unavailable rather than permitting Node to manufacture a recipient
	// digest, role-domain or duty generation.
	CurrentClosedRoute func() (state.ClosedRouteView, error)
	PollInterval       time.Duration
	Quarantine         time.Duration
	ResourceProfile    string
	NetworkStateRoot   string
	LocalRoleStateRoot string
	// ResourceMeasure and CheckPlacement are behavior-test seams. Maintained
	// runtime callers leave them nil and use ResourceProfile's platform adapter.
	ResourceMeasure func() (resource.Sample, error)
	Now             func() time.Time
	CheckPlacement  func() error
	// Emit must honor ctx cancellation and return before its deadline.
	Emit func(context.Context, Event) error
}

// ClosedIssuerProfile contains the isolated RSA-PSS issuer root and bounded
// direct role listener reservation. State selects its endpoint, carrier and
// current issuer profile; this local profile cannot select a recipient.
type ClosedIssuerProfile struct {
	Root            string
	AdmissionRoot   string
	Certificate     tls.Certificate
	ConnectionLimit uint16
	DrainTimeout    time.Duration
}

// ClosedForwardingProfile contains the local material for one closed Route
// forwarding duty. Root is exclusively owned by its current receiving duty;
// it must not share an issuer root or survive a changed duty generation.
type ClosedForwardingProfile struct {
	Root            string
	Certificate     tls.Certificate
	ConnectionLimit uint16
	DrainTimeout    time.Duration
	// HostingRoot names the installed shared provider-period ledger. It is
	// local operator configuration, never State or token material.
	HostingRoot string
	// CarrierRelayEndpoint is an optional operator-owned transparent relay
	// address for this forwarding Node's State-selected next Carrier. It cannot
	// change the selected Node identity, key, duty, profile, or TLS verification.
	CarrierRelayEndpoint string
	AdmissionTraffic     resource.HostingTraffic
	TerminationTraffic   resource.HostingTraffic
	host                 closedForwardingHost
}

// Event is one bounded external observation of Node lifecycle state.
type Event struct {
	Elapsed          time.Duration           `json:"elapsed,omitempty"`
	Hosting          *resource.HostingSample `json:"hosting,omitempty"`
	Schema           string                  `json:"schema"`
	Kind             string                  `json:"kind"`
	State            string                  `json:"state"`
	At               time.Time               `json:"at"`
	Epoch            uint64                  `json:"epoch,omitempty"`
	Generation       string                  `json:"generation,omitempty"`
	Assignment       string                  `json:"assignment,omitempty"`
	CarrierProfile   string                  `json:"carrier_profile,omitempty"`
	AssignmentDigest [32]byte                `json:"assignment_digest,omitempty"`
	Reason           string                  `json:"reason,omitempty"`
	Resource         *resource.Sample        `json:"resource,omitempty"`
}

// Result describes the observed terminal lifecycle outcome. FAILED may report
// cleanup that did not complete or could not be proven inside its bound.
type Result struct {
	State            string
	Epoch            uint64
	Assignment       string
	CarrierProfile   string
	AssignmentDigest [32]byte
	Reason           string
}

type runtimeConfig struct {
	measurementOrigin time.Time
	hostingSample     *resource.HostingSample
	hostingUsage      resource.Sample
	host              closedForwardingHost
	hostingNext       time.Time
	hostingLevel      pressureLevel
	Config
	now      func() time.Time
	probe    *probePlan
	pressure *resource.Guard
}
