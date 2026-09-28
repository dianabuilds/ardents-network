package state

import (
	"context"
	"crypto/ed25519"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/network/source"
	"github.com/dianabuilds/ardents-network/internal/network/state/durable"
	"github.com/dianabuilds/ardents-network/internal/resource"
)

// networkState owns verified-decision admission, finite Source work, and publication order.
// The durable package holds its exclusive physical root and persisted bytes.
type networkState struct {
	mu              sync.RWMutex
	config          config
	current         *epoch.Decision
	pendingDecision *epoch.Decision
	distribution    distributionState
	storage         *durable.Root
	serverDone      chan struct{}
	resourceDone    chan struct{}
	automaticDone   chan struct{}
	serverErr       error
	terminalErr     error
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
	closeOnce       sync.Once
	closeErr        error
}

type config struct {
	root                   string
	networkID              [32]byte
	authorities            map[[32]byte]ed25519.PublicKey
	threshold              int
	closedProfileAuthority ed25519.PublicKey
	acceptedProfile        string
	now                    time.Time
	clock                  func() time.Time
	source                 *source.Plan
	sourceInfo             source.Details
	observation            time.Time
	observe                func() time.Time
	automatic              time.Duration
	profile                string
	resources              func([]byte) error
	localRoles             string
	anchorWall             time.Time
	anchorMono             time.Time
}
