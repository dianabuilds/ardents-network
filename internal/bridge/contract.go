package bridge

import (
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

// Config supplies one owned state root and the authenticated facts required to
// validate every active Invite. Inputs are copied by Open.
type Config struct {
	Root              string
	RouteProfile      string
	CurrentNetwork    func() (state.Snapshot, error)
	Clock             func() time.Time
	RoleConflict      func([32]byte, [32]byte) (bool, error)
	ValidateCandidate func([]byte, [32]byte) ([32]byte, string, error)
}

// Class is one closed local import outcome. Values are serialized as the exact
// R-035 strings; callers cannot select or mutate state with them.
type Class string

// Result is the bounded local classification of one import attempt.
type Result struct {
	Class      Class    `json:"class"`
	InviteID   [32]byte `json:"invite_id"`
	Slot       uint8    `json:"slot"`
	Generation uint8    `json:"generation"`
}

type owner struct {
	mu      sync.Mutex
	root    string
	lease   rootLease
	config  Config
	state   durableState
	current string
	closed  bool
	failed  error
}
