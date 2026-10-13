package issuance

import (
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

// Store copies share one lifecycle and exclusive lease.
type Store struct{ state *storeState }
type storeState struct {
	mu        sync.Mutex
	root      *os.Root
	path      string
	identity  os.FileInfo
	files     map[string]os.FileInfo
	lock      *os.File
	inventory Inventory
	material  [32]byte
	closed    bool
	failure   error
	closeErr  error
}

// Supported reports whether native storage semantics are available.
func Supported() bool { return platform() == nil }

// Initialize creates a fresh immutable root. It never reopens or replaces one.
func Initialize(ctx context.Context, root string, b Binding) error {
	return initialize(ctx, root, b, nil)
}
func initialize(ctx context.Context, root string, b Binding, fault func(string) error) error {
	if err := platform(); err != nil {
		return err
	}
	if ctx == nil || !b.valid() || !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return ErrInvalid
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	// Refuse existing roots before spending work, without claiming or changing them.
	if _, err := os.Lstat(root); !errors.Is(err, os.ErrNotExist) {
		return ErrUnavailable
	}
	raw, err := generate(ctx, b)
	if err != nil {
		return err
	}
	defer clear(raw)
	inventory, err := decodeMaterial(raw, b)
	if err != nil {
		return err
	}
	return initializeStorage(ctx, root, b, raw, inventory.Digest, fault)
}

// Open validates retained identity even for an expired preparation interval.
func Open(ctx context.Context, root string, b Binding) (Store, error) {
	if err := platform(); err != nil {
		return Store{}, err
	}
	if ctx == nil || !b.valid() {
		return Store{}, ErrInvalid
	}
	if ctx.Err() != nil {
		return Store{}, ctx.Err()
	}
	return openStorage(ctx, root, b, nil)
}

// Inventory returns a defensive public copy while this owner remains open.
func (s Store) Inventory() (Inventory, error) {
	if s.state == nil {
		return Inventory{}, ErrClosed
	}
	v := s.state
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.closed {
		return Inventory{}, ErrClosed
	}
	if err := v.checkRetained(); err != nil {
		return Inventory{}, err
	}
	return v.inventory.clone(), nil
}

// Close releases the shared lease once and retains the result on every copy.
func (s Store) Close() error {
	if s.state == nil {
		return ErrClosed
	}
	v := s.state
	v.mu.Lock()
	defer v.mu.Unlock()
	if !v.closed {
		v.closed = true
		v.closeErr = errors.Join(v.failure, v.checkIdentity(), releaseLock(v.lock), v.root.Close())
	}
	return v.closeErr
}

// Check validates original material and lease before dependent effects. A
// detected loss is terminal for every copy, even if the path is restored.
func (s Store) Check() error {
	if s.state == nil {
		return ErrClosed
	}
	v := s.state
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.closed {
		return ErrClosed
	}
	return v.checkRetained()
}

func (s *storeState) checkRetained() error {
	if s.failure != nil {
		return s.failure
	}
	err := s.checkIdentity()
	if err == nil {
		var raw []byte
		raw, _, err = readOwned(s.root, "issuer.keys", maximumMaterial)
		if err == nil {
			if sha256.Sum256(raw) != s.material {
				err = ErrUnavailable
			}
		}
		// Open fully validated these exact material bytes and their inventory.
		// Equality to that retained digest preserves the proof without reparsing
		// every immutable private key during each native custody observation.
		clear(raw)
	}
	if err != nil {
		s.failure = errors.Join(ErrUnavailable, err)
	}
	return s.failure
}
