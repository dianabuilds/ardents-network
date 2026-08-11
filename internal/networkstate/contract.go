package networkstate

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"fmt"
	"path/filepath"
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
}

// Materialization is one precommitted Candidate View member and its proof.
type Materialization struct {
	EpochDigest [32]byte
	Index       uint32
	Record      []byte
	Siblings    [][32]byte
}

// Snapshot is an immutable description of the current verified generation.
type Snapshot struct {
	Generation     string
	Epoch          uint64
	Digest         [32]byte
	ValidUntil     time.Time
	ViewRoot       [32]byte
	ViewLength     uint32
	RejectedRoot   [32]byte
	RejectedLength uint32
}

// Store owns verification, at most 64 S1-0 generations, and the atomic current pointer.
type Store struct {
	mu      sync.RWMutex
	config  config
	current *Snapshot
	closed  bool
}

type config struct {
	root        string
	networkID   [32]byte
	authorities map[[32]byte]ed25519.PublicKey
	threshold   int
	now         time.Time
}

// Open recovers one state root and verifies any current generation before use.
func Open(input Config) (*Store, error) {
	resolved, err := validateConfig(input)
	if err != nil {
		return nil, err
	}
	if err := prepareRoot(resolved.root); err != nil {
		return nil, err
	}
	store := &Store{config: resolved}
	current, err := loadCurrent(resolved)
	if err != nil {
		return nil, err
	}
	store.current = current
	return store, nil
}

// Accept verifies a complete offline decision before committing a new generation.
func (s *Store) Accept(ctx context.Context, epoch []byte, inputs [][]byte, materials []Materialization) (Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return Snapshot{}, errors.New("network state is closed")
	}
	decision, err := verifyDecision(s.config, s.current, epoch, inputs, materials, true)
	if err != nil {
		return Snapshot{}, err
	}
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	if err := commitGeneration(s.config.root, decision); err != nil {
		return Snapshot{}, err
	}
	snapshot := decision.snapshot
	s.current = &snapshot
	return snapshot, nil
}

// Current returns a copy of the current immutable Snapshot.
func (s *Store) Current() (Snapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return Snapshot{}, errors.New("network state is closed")
	}
	if s.current == nil {
		return Snapshot{}, errors.New("network state has no current generation")
	}
	return *s.current, nil
}

// Close prevents further work through this Store.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	return nil
}

func validateConfig(input Config) (config, error) {
	if input.Root == "" {
		return config{}, errors.New("state root is required")
	}
	root, err := filepath.Abs(input.Root)
	if err != nil {
		return config{}, fmt.Errorf("resolve state root: %w", err)
	}
	if input.Threshold < 1 || input.Threshold > len(input.Authorities) {
		return config{}, errors.New("authority threshold is outside the authority set")
	}
	if len(input.Authorities) > 16 {
		return config{}, errors.New("authority set exceeds 16 keys")
	}
	if input.Now.IsZero() {
		return config{}, errors.New("verification time is required")
	}
	authorities := make(map[[32]byte]ed25519.PublicKey, len(input.Authorities))
	for id, public := range input.Authorities {
		if len(public) != ed25519.PublicKeySize {
			return config{}, errors.New("authority public key has invalid length")
		}
		if sha256.Sum256(public) != id {
			return config{}, errors.New("authority identifier does not match its public key")
		}
		authorities[id] = append(ed25519.PublicKey(nil), public...)
	}
	return config{
		root:        root,
		networkID:   input.NetworkID,
		authorities: authorities,
		threshold:   input.Threshold,
		now:         input.Now.UTC(),
	}, nil
}
