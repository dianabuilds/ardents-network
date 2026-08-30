package duty

import (
	"errors"
	"sync"
	"time"
)

// Config identifies one owner-only local-role root. Create is reserved for an
// explicit Endpoint initialization or a maintained role producer.
type Config struct {
	Root   string
	Clock  func() time.Time
	Create bool
}

// Duty is one authenticated or locally retained conflict fact with a finite
// terminal bound. Family is the canonical family digest.
type Duty struct {
	Identity [32]byte
	Family   [32]byte
	Class    string
	State    string
	NotAfter time.Time
}

// store serializes one bounded durable generation and owns its root lease.
type store struct {
	mu      sync.Mutex
	root    string
	clock   func() time.Time
	lease   rootLease
	state   durableState
	current string
	closed  bool
	failed  error
}

type durableState struct {
	Version            uint8               `json:"version"`
	Generation         uint64              `json:"generation"`
	Previous           string              `json:"previous,omitempty"`
	Duties             []dutyRecord        `json:"duties"`
	TransitGrantSpends []transitGrantSpend `json:"transit_grant_spends"`
	TransitGrantIssuer *transitGrantIssuer `json:"transit_grant_issuer,omitempty"`
}

type dutyRecord struct {
	Producer [32]byte `json:"producer"`
	Identity [32]byte `json:"identity"`
	Family   [32]byte `json:"family"`
	Class    string   `json:"class"`
	State    string   `json:"state"`
	NotAfter int64    `json:"not_after"`
}

// transitGrantSpend is one Node-local, finite, irreversible consumption of an
// already State-authorized transit admission capability. It has no Target,
// Service, or client material.
type transitGrantSpend struct {
	NodeID   [32]byte `json:"node_id"`
	GrantID  [32]byte `json:"grant_id"`
	NotAfter int64    `json:"not_after"`
}

// TransitGrantIssuerScope is one exact State-authenticated online signing
// duty. GrantSignerID identifies only the purpose-scoped Transit Grant key;
// it is not an Epoch authority identifier.
type TransitGrantIssuerScope struct {
	NetworkID, Digest, IssuerNodeID, GrantSignerID [32]byte
	Epoch                                          uint64
	NotAfter                                       time.Time
}

// ErrTransitGrantIssuerExhausted and ErrTransitGrantIssuerWithdrawn are the
// two authenticated terminal reservation classes. A Request ID conflict is
// deliberately only unavailable to its caller.
var (
	ErrTransitGrantIssuerExhausted = errors.New("transit grant issuer budget is exhausted")
	ErrTransitGrantIssuerWithdrawn = errors.New("transit grant issuer duty is withdrawn")
	ErrTransitGrantRequestConflict = errors.New("transit grant request identifier conflicts")
)

type transitGrantIssuer struct {
	ProfileDigest   [32]byte                  `json:"profile_digest,omitempty"`
	Profile         []byte                    `json:"profile,omitempty"`
	NetworkID       [32]byte                  `json:"network_id"`
	Digest          [32]byte                  `json:"digest"`
	IssuerNodeID    [32]byte                  `json:"issuer_node_id"`
	GrantSignerID   [32]byte                  `json:"grant_signer_id"`
	Epoch           uint64                    `json:"epoch"`
	NotAfter        int64                     `json:"not_after"`
	Budget          uint16                    `json:"budget"`
	Withdrawn       bool                      `json:"withdrawn"`
	PrivateMaterial []byte                    `json:"private_material"`
	Reservations    []transitGrantReservation `json:"reservations"`
}

type transitGrantReservation struct {
	RequestID     [32]byte `json:"request_id"`
	RequestDigest [32]byte `json:"request_digest"`
	GrantID       [32]byte `json:"grant_id"`
}

const (
	maximumStateBytes               = 64 << 10
	maximumTransitGrantSpends       = 64
	maximumTransitGrantBudget       = 64
	maximumTransitGrantProfileBytes = 4096
)
