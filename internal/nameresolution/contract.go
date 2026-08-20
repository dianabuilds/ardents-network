package nameresolution

import (
	"crypto/ed25519"
	"net/http"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/namelease"
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
	NetworkID          [32]byte
	NodeID             [32]byte
	Family             string
	Domain             string
	AssignmentNotAfter time.Time
	MaximumPending     uint16
	SignedRecordChains [][][]byte
	IdentityKey        ed25519.PrivateKey
	Clock              func() time.Time
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
}

// Result is the bounded local outcome. A Record is present only for resolved.
type Result struct {
	Class   string
	Warning string
	Record  namelease.Record
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
	Requests uint32
	Resolved uint32
	Rejected uint32
}

type recordSet struct {
	network [32]byte
	chains  map[string]recordChain
}

type recordChain struct {
	head   namelease.Record
	signed [][]byte
}

type gateway struct {
	config      GatewayConfig
	records     recordSet
	profile     GatewayProfile
	handler     http.Handler
	mu          sync.Mutex
	seen        map[[32]byte]int64
	observation gatewayObservation
}

type relay struct {
	gateway     string
	client      *http.Client
	mu          sync.Mutex
	observation relayObservation
}

type resolver struct {
	plan        plan
	client      *http.Client
	transport   *ohttp.Transport
	mu          sync.Mutex
	used        bool
	observation resolverObservation
}
