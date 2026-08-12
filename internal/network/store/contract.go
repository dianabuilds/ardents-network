package store

import (
	"errors"
	"path/filepath"
	"sync"
)

// Generation is one opaque immutable authenticated-state generation. Semantic
// verification belongs to network/epoch and network/state.
type Generation struct {
	Name     string
	Epoch    []byte
	Inputs   [][]byte
	Activate bool
}

// Root owns the exclusive lifetime lease for one persisted Network State root.
type Root struct {
	mu     sync.Mutex
	path   string
	lease  rootLease
	closed bool
}

// Open claims or verifies one exact owned root and acquires its exclusive lease.
func Open(path string) (*Root, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if err := inspectRoot(absolute); err != nil {
		return nil, err
	}
	lease, err := acquireRootLease(absolute)
	if err != nil {
		return nil, err
	}
	opened := false
	defer func() {
		if !opened {
			_ = lease.release()
		}
	}()
	if err := prepareRoot(absolute); err != nil {
		return nil, err
	}
	if err := verifyRootWritable(absolute); err != nil {
		return nil, err
	}
	root := &Root{path: absolute, lease: lease}
	if err := root.prepareControl(); err != nil {
		return nil, err
	}
	opened = true
	return root, nil
}

// Close releases the exclusive root lease after all callers have stopped.
func (root *Root) Close() error {
	root.mu.Lock()
	defer root.mu.Unlock()
	if root.closed {
		return nil
	}
	root.closed = true
	return root.lease.release()
}

func (root *Root) available() error {
	if root.closed {
		return errors.New("network state store is closed")
	}
	return nil
}
