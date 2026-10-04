package state

import (
	"context"
	"crypto/ed25519"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/duty"
	"github.com/dianabuilds/ardents-network/internal/successor/network/epoch"
	source2 "github.com/dianabuilds/ardents-network/internal/successor/network/source"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state/durable"
)

// networkState owns verified-decision admission, finite Source work, and publication order.
// The durable package holds its exclusive physical root and persisted bytes.
type networkState struct {
	mu                  sync.RWMutex
	config              config
	current             *epoch.Decision
	pendingDecision     *epoch.Decision
	distribution        distributionState
	storage             *durable.Root
	serverDone          chan struct{}
	automaticDone       chan struct{}
	serverErr           error
	terminalErr         error
	automaticErr        error
	activeSource        uint16
	servingPredecessors []duty.Duty
	workContext         context.Context
	workCancel          context.CancelFunc
	work                sync.WaitGroup
	refreshing          bool
	closed              bool
	closeOnce           sync.Once
	closeErr            error
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
	source                 *source2.Plan
	fetchSourceOperation   func(context.Context, int, source2.Message) (source2.Message, error)
	sourceInfo             source2.Details
	observation            time.Time
	observe                func() time.Time
	automatic              time.Duration
	permitWork             func() error
	localRoles             string
	anchorWall             time.Time
	anchorMono             time.Time
}
