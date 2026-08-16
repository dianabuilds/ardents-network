package localroles

import (
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
	Version    uint8        `json:"version"`
	Generation uint64       `json:"generation"`
	Previous   string       `json:"previous,omitempty"`
	Duties     []dutyRecord `json:"duties"`
}

type dutyRecord struct {
	Producer [32]byte `json:"producer"`
	Identity [32]byte `json:"identity"`
	Family   [32]byte `json:"family"`
	Class    string   `json:"class"`
	State    string   `json:"state"`
	NotAfter int64    `json:"not_after"`
}

const maximumStateBytes = 64 << 10
