package networkstate

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"errors"
	"sync"
	"time"
)

// Config identifies one owned state root and its offline trust policy.
type Config struct {
	Root        string
	NetworkID   [32]byte
	Authorities map[[32]byte]ed25519.PublicKey
	Threshold   int
	Now         time.Time
	Clock       func() time.Time

	SourceAddresses            [2]string
	SourceServerNames          [2]string
	SourceIdentities           [2][32]byte
	SourceFamilies             [2]string
	SourceEndpointHandles      [2]string
	SourceRootPEM              [2][]byte
	SourceLeafKeyDigests       [2][32]byte
	SourceClientCertificate    tls.Certificate
	SourceMaterializationIndex uint32
	ClockObservation           time.Time
	ClockObservationFile       string
	ObserveClock               func() time.Time
	SourceOrderSeed            [32]byte
	AutomaticRefreshInterval   time.Duration

	ServeAddress           string
	ServeCertificate       tls.Certificate
	ServeClientRootPEM     []byte
	ServeClientKeyDigests  [][32]byte
	ServeReadHeaderTimeout time.Duration
}

type materialization struct {
	EpochDigest [32]byte
	Index       uint32
	Record      []byte
	Siblings    [][32]byte
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
	lease           rootLease
	serverDone      chan struct{}
	serverErr       error
	automaticErr    error
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
	sources     [2]sourceConfig
	material    uint32
	observation time.Time
	observe     func() time.Time
	orderSeed   [32]byte
	automatic   time.Duration
	server      sourceServerConfig
	anchorWall  time.Time
	anchorMono  time.Time
}

// Open recovers one state root and verifies any current generation before use.
func Open(input Config) (*store, error) {
	resolved, err := validateConfig(input)
	if err != nil {
		return nil, err
	}
	if err := inspectRoot(resolved.root); err != nil {
		return nil, err
	}
	lease, err := acquireRootLease(resolved.root)
	if err != nil {
		return nil, err
	}
	opened := false
	defer func() {
		if !opened {
			_ = lease.release()
		}
	}()
	if err := prepareRoot(resolved.root); err != nil {
		return nil, err
	}
	workContext, workCancel := context.WithCancel(context.Background())
	defer func() {
		if !opened {
			workCancel()
		}
	}()
	store := &store{config: resolved, lease: lease, workContext: workContext, workCancel: workCancel}
	current, err := loadCurrent(resolved)
	if err != nil {
		return nil, err
	}
	store.current = current
	if current != nil {
		decision, _, loadErr := loadGenerationChain(resolved, current.Generation, make(map[string]bool), true)
		if loadErr != nil {
			return nil, loadErr
		}
		store.currentDecision = &decision
	}
	if err := store.loadDistributionState(); err != nil {
		return nil, err
	}
	if resolved.server.address != "" {
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
	opened = true
	return store, nil
}

// Accept verifies a complete offline decision before committing a new generation.
func (s *store) Accept(ctx context.Context, epoch []byte, inputs [][]byte, encodedMaterials [][]byte) (Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return Snapshot{}, errors.New("network state is closed")
	}
	if s.refreshing {
		return Snapshot{}, errors.New("network state refresh owns the active transition")
	}
	materials, err := decodeMaterializations(encodedMaterials)
	if err != nil {
		return Snapshot{}, err
	}
	decision, err := verifyDecision(s.config, s.current, epoch, inputs, materials, true)
	if err != nil {
		return Snapshot{}, err
	}
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	state := s.distribution
	state.sequence++
	state.trustedTimeFloor = max(state.trustedTimeFloor, s.config.clock().UTC().Unix())
	if err := s.commitActiveDecision(decision, state); err != nil {
		return Snapshot{}, err
	}
	return s.snapshotWithDistribution(s.config.clock().UTC()), nil
}
