package resolution

import (
	"crypto/ed25519"
	"net/http"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/naming/namespace"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/admission"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/authority"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace/record"
	"github.com/openpcc/ohttp"
)

const (
	fixedMessageSize           = 4096
	initiatorDomain            = "initiator"
	rendezvousDomain           = "rendezvous"
	resolvedClass              = "resolved"
	policyDenialClass          = "local authorization or policy denial"
	resolutionUnavailableClass = "private resolution unavailable"
	invalidEvidenceClass       = "invalid naming evidence"
)

// GatewayConfig is the complete trusted startup input for one naming Gateway.
// It contains no Endpoint or Isolation Context information.
type GatewayConfig struct {
	NodeID             [32]byte
	Family             string
	Domain             string
	AssignmentNotAfter time.Time
	MaximumPending     uint16
	IdentityKey        ed25519.PrivateKey
	Clock              func() time.Time
	State              gatewayState
}

type controlAuthority interface {
	Submit(authority.Submission, admission.Proof) string
}

type gatewayState struct {
	namespace *namespace.ResolutionGateway
	authority controlAuthority
}

// Selection identifies the three authenticated roles needed by one private
// resolution and its future Service Connection.
type Selection struct {
	At                         time.Time
	Deadline                   time.Time
	RelayNodeID                [32]byte
	GatewayNodeID              [32]byte
	ConnectionRendezvousNodeID [32]byte
	ExcludedIdentities         [][32]byte
	ExcludedFamilies           []string
	AdmissionChallenge         admission.Challenge
}

// controlResult is the bounded terminal result of one private naming control
// operation. Its fields remain readable by the immediate command caller while
// the concrete result vocabulary stays owned by this module.
type controlResult struct {
	Class string `json:"class"`
}

// GatewayProfile is the finite common OHTTP configuration authenticated by the
// Gateway Node identity recorded in Network State.
type GatewayProfile struct {
	NetworkID          [32]byte
	NodeID             [32]byte
	KeyConfig          []byte
	KeyConfigDigest    [32]byte
	AssignmentNotAfter time.Time
	Signature          []byte
}

type position struct {
	NodeID             [32]byte
	PublicKey          [32]byte
	Family             string
	Endpoint           string
	Domain             string
	AssignmentNotAfter time.Time
}

type plan struct {
	NetworkID              [32]byte
	Generation             string
	Epoch                  uint64
	EpochDigest            [32]byte
	ViewRoot               [32]byte
	SelectionAt            int64
	Deadline               int64
	Relay                  position
	Gateway                position
	ConnectionRendezvous   position
	GatewayKeyConfig       []byte
	GatewayKeyConfigDigest [32]byte
	ExcludedIdentities     [][32]byte
	ExcludedFamilies       []string
	AdmissionChallenge     admission.Challenge
	NamespaceVerifier      *namespace.ResolutionVerifier
}

type result struct {
	Class   string
	Warning string
	Binding record.Binding
}

type resolverObservation struct {
	Requests uint32
	Resolved uint32
	Failed   uint32
}

type relayObservation struct {
	Requests      uint32
	RequestBytes  uint64
	ResponseBytes uint64
}

type gatewayObservation struct {
	Requests        uint32
	Resolved        uint32
	Rejected        uint32
	ControlRequests uint32
	ControlAccepted uint32
	ControlDenied   uint32
}

type gateway struct {
	config       GatewayConfig
	state        gatewayState
	profile      GatewayProfile
	handler      http.Handler
	mu           sync.Mutex
	seen         map[[32]byte]int64
	observation  gatewayObservation
	roleEvidence []gatewayRoleEvidence
}

type relay struct {
	gateway      string
	client       *http.Client
	mu           sync.Mutex
	observation  relayObservation
	roleEvidence []relayRoleEvidence
}

type resolver struct {
	plan         plan
	client       *http.Client
	transport    *ohttp.Transport
	mu           sync.Mutex
	used         bool
	observation  resolverObservation
	roleEvidence resolverRoleEvidence
}

type controlClient struct {
	plan      controlPlan
	client    *http.Client
	transport *ohttp.Transport
	mu        sync.Mutex
	used      bool
}

type controlPlan struct {
	NetworkID          [32]byte
	SelectionAt        int64
	Deadline           int64
	Relay              position
	Gateway            position
	GatewayKeyConfig   []byte
	AdmissionChallenge admission.Challenge
}
