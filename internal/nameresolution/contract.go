package nameresolution

import (
	"crypto/ed25519"
	"net/http"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/namelease"
	"github.com/dianabuilds/ardents-network/internal/namestore"
	"github.com/dianabuilds/ardents-network/internal/naming/namespace"
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
	Apply([]byte, namespace.Proof) (string, uint64, uint64, []byte)
}

type gatewayState struct {
	network     [32]byte
	recordStore *namestore.Store
	policy      namestore.Policy
	minimum     uint64
	epochDigest [32]byte
	admission   *namespace.Admission
	authority   controlAuthority
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
	AdmissionChallenge         namespace.Challenge
}

// controlOperation is the exact naming-side view after OHTTP decapsulation.
// The public module boundary accepts only its strict canonical bytes.
type controlOperation struct {
	Kind               string   `json:"kind"`
	OperationDigest    [32]byte `json:"operation_digest"`
	Network            [32]byte `json:"network"`
	Nonce              [32]byte `json:"nonce"`
	Deadline           int64    `json:"deadline"`
	Name               string   `json:"name"`
	ParentName         string   `json:"parent_name"`
	Generation         uint64   `json:"generation"`
	ExpectedRevision   uint64   `json:"expected_revision"`
	ParentGeneration   uint64   `json:"parent_generation"`
	ParentRevision     uint64   `json:"parent_revision"`
	ChildGeneration    uint64   `json:"child_generation"`
	Authority          [32]byte `json:"authority"`
	SuccessorAuthority [32]byte `json:"successor_authority"`
	Target             [32]byte `json:"target"`
	LeaseNotAfter      int64    `json:"lease_not_after"`
	RecordNotAfter     int64    `json:"record_not_after"`
	PolicyNotBefore    int64    `json:"policy_not_before"`
	RecoveryNotBefore  int64    `json:"recovery_not_before"`
	PolicyID           [32]byte `json:"policy_id"`
	RecoveryStep       string   `json:"recovery_step"`
	OrderingProof      []byte   `json:"ordering_proof"`
	AuthorityProof     []byte   `json:"authority_proof"`
	RecoveryPolicy     []byte   `json:"recovery_policy"`
	RecoveryProof      []byte   `json:"recovery_proof"`
}

// controlResult is the bounded terminal result of one private naming control
// operation. Its fields remain readable by the immediate command caller while
// the concrete result vocabulary stays owned by this module.
type controlResult struct {
	Class      string `json:"class"`
	Generation uint64 `json:"generation"`
	Revision   uint64 `json:"revision"`
	State      []byte `json:"state"`
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
	AdmissionChallenge     namespace.Challenge
	MaterializationPolicy  namestore.Policy
}

type result struct {
	Class   string
	Warning string
	Record  namelease.Record
	Binding namelease.Binding
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

type recordSet struct {
	network      [32]byte
	store        *namestore.Store
	minimumEpoch uint64
}

type gateway struct {
	config       GatewayConfig
	state        gatewayState
	records      recordSet
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
	AdmissionChallenge namespace.Challenge
}
