package instance

import (
	"errors"
	"sync"
	"time"
)

var (
	ErrInvalid = errors.New("invalid Service Instance root")
	ErrBusy    = errors.New("Service Instance root is busy")
	ErrClosed  = errors.New("Service Instance root is closed")
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

// Root is the exclusive owner of one durable Service Instance generation.
type Root struct {
	mu     sync.Mutex
	path   string
	lock   *rootLock
	state  durableState
	closed bool
}
