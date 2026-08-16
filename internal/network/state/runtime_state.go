package state

import (
	"context"
	"crypto/ed25519"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/source"
	statestore "github.com/dianabuilds/ardents-network/internal/network/store"
	"github.com/dianabuilds/ardents-network/internal/resource"
)

// networkState owns verification, finite source work, immutable generations,
// and the durable pointers that publish them.
type networkState struct {
	mu              sync.RWMutex
	config          config
	current         *Snapshot
	currentDecision *candidateDecision
	pendingDecision *candidateDecision
	distribution    distributionState
	storage         *statestore.Root
	serverDone      chan struct{}
	serverErr       error
	automaticErr    error
	resourceErr     error
	resourceProtect bool
	resourceGuard   *resource.Guard
	activeSource    uint16
	workContext     context.Context
	workCancel      context.CancelFunc
	work            sync.WaitGroup
	refreshing      bool
	closed          bool
}

type config struct {
	root            string
	networkID       [32]byte
	authorities     map[[32]byte]ed25519.PublicKey
	threshold       int
	acceptedProfile string
	now             time.Time
	clock           func() time.Time
	source          *source.Plan
	sourceInfo      source.Details
	observation     time.Time
	observe         func() time.Time
	automatic       time.Duration
	profile         string
	resources       func([]byte) error
	localRoles      string
	anchorWall      time.Time
	anchorMono      time.Time
}
