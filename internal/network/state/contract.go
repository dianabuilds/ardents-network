package state

import (
	"context"
	"crypto/ed25519"
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/source"
	statestore "github.com/dianabuilds/ardents-network/internal/network/store"
	"github.com/dianabuilds/ardents-network/internal/resource"
)

// Config identifies one owned state root and its offline trust policy.
type Config struct {
	Root        string
	NetworkID   [32]byte
	Authorities map[[32]byte]ed25519.PublicKey
	Threshold   int
	Now         time.Time
	Clock       func() time.Time

	Source                   source.Config
	ClockObservation         time.Time
	ClockObservationFile     string
	ObserveClock             func() time.Time
	AutomaticRefreshInterval time.Duration
	RuntimeProfile           string
}

// Snapshot is an immutable description of the current verified generation.
type Snapshot struct {
	Generation         string
	NetworkID          [32]byte
	Epoch              uint64
	Digest             [32]byte
	EpochValidFrom     time.Time
	ValidUntil         time.Time
	Profile            string
	ViewRoot           [32]byte
	ViewLength         uint32
	RejectedRoot       [32]byte
	RejectedLength     uint32
	Freshness          string
	Conflicting        bool
	SourceAttempts     uint16
	SourceOutcomes     [4]string
	LatestCompleteness string
	ObservedEpochs     [4]uint64
	ObservedDigests    [4][32]byte
	TrustedTime        time.Time
	NextAutomatic      time.Time
	PendingEpoch       uint64
	PendingDigest      [32]byte
	PendingAt          time.Time
	RecordPresent      bool
	NodeID             [32]byte
	NodePublicKey      [32]byte
	RecordGeneration   uint64
	RecordValidFrom    time.Time
	RecordValidUntil   time.Time
	DeclaredFamily     string
	ProbeEndpoint      string
	ProbeCapacity      uint16
	Assignment         string
	AssignmentDigest   [32]byte
}

// store owns verification, finite source work, immutable generations, and pointers.
type store struct {
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
	root        string
	networkID   [32]byte
	authorities map[[32]byte]ed25519.PublicKey
	threshold   int
	now         time.Time
	clock       func() time.Time
	source      *source.Plan
	sourceInfo  source.Details
	observation time.Time
	observe     func() time.Time
	automatic   time.Duration
	profile     string
	anchorWall  time.Time
	anchorMono  time.Time
}

// Open recovers one state root and verifies any current generation before use.
func Open(input Config) (*store, error) {
	resolved, err := validateConfig(input)
	if err != nil {
		return nil, err
	}
	var guard *resource.Guard
	if resolved.profile != "" {
		guard, err = resource.New(resource.Config{Profile: resolved.profile, Interval: time.Second})
		if err != nil {
			return nil, err
		}
		if err := guard.Check(); err != nil {
			return nil, err
		}
	}
	storage, err := statestore.Open(resolved.root)
	if err != nil {
		return nil, err
	}
	opened := false
	defer func() {
		if !opened {
			_ = storage.Close()
		}
	}()
	workContext, workCancel := context.WithCancel(context.Background())
	defer func() {
		if !opened {
			workCancel()
		}
	}()
	store := &store{config: resolved, storage: storage, workContext: workContext, workCancel: workCancel, resourceGuard: guard}
	current, currentDecision, err := loadCurrent(resolved, storage)
	if err != nil {
		return nil, err
	}
	store.current, store.currentDecision = current, currentDecision
	if err := store.loadDistributionState(); err != nil {
		return nil, err
	}
	if resolved.sourceInfo.Serving {
		if store.current == nil {
			return nil, errors.New("source mode requires a current generation")
		}
		store.serverDone = make(chan struct{})
		ready := make(chan error, 1)
		go func() {
			err := store.serveSource(workContext, ready)
			store.mu.Lock()
			store.serverErr = err
			close(store.serverDone)
			store.mu.Unlock()
		}()
		if err := <-ready; err != nil {
			workCancel()
			<-store.serverDone
			return nil, err
		}
	}
	if resolved.automatic > 0 {
		store.work.Add(1)
		go store.runAutomaticRefresh(workContext)
	}
	if resolved.profile == "h3-s-v1" {
		store.work.Add(1)
		go store.runResourceGovernor(workContext)
	}
	opened = true
	return store, nil
}
