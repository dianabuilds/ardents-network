package instance

import (
	"crypto"
	"errors"
	"sync"
	"time"
)

var (
	ErrInvalid = errors.New("invalid Service Instance root")
	ErrBusy    = errors.New("Service Instance root is busy")
	ErrClosed  = errors.New("Service Instance root is closed")
	// ErrPending reports that no exact Authority response has been accepted.
	ErrPending = errors.New("Service Instance response is pending")
	// ErrUnavailable reports a rejected, conflicting, or withdrawn generation.
	ErrUnavailable = errors.New("Service Instance generation is unavailable")
	// ErrSuccessorRequired reports a generation already committed to publication.
	ErrSuccessorRequired = errors.New("Service Instance successor is required")
)

// State is the durable one-generation Instance lifecycle classification.
type State string

const (
	StatePending     State = "pending"
	StateAccepted    State = "accepted"
	StateConsumed    State = "consumed"
	StateWithdrawn   State = "withdrawn"
	StateRejected    State = "rejected"
	StateConflicting State = "conflicting"
)

// InitializeConfig contains the public facts committed by a new host root.
type InitializeConfig struct {
	Root      string
	NetworkID [32]byte
	NotBefore time.Time
	NotAfter  time.Time
}

// RequestView is the complete public credential-request surface.
type RequestView struct {
	NetworkID          [32]byte
	InstancePublic     [32]byte
	IntroductionPublic [32]byte
	NotBefore          int64
	NotAfter           int64
	Commitment         [32]byte
}

// Acceptance is the public result of one exact response transition.
type Acceptance struct {
	State      State
	Generation uint64
}

// Root is the exclusive owner of one durable Service Instance generation.
type Root struct {
	mu          sync.Mutex
	path        string
	lock        *rootLock
	state       durableState
	bindingOpen bool
	closed      bool
}

// Binding is the opened non-exporting Instance authority for one accepted
// generation. It implements crypto.Signer without returning private bytes.
type Binding struct {
	root                 *Root
	credentialGeneration uint64
	closed               bool
}

var _ crypto.Signer = (*Binding)(nil)
