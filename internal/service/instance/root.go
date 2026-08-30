package instance

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	markerName = "instance-root.marker"
	marker     = "ardents-service-instance-root-v1\n"
	stateName  = "instance-root.json"
	lockName   = ".instance-root.lock"
)

// Initialize creates a new generation or reopens an exactly matching one.
func Initialize(config InitializeConfig) (*Root, error) {
	if config.Root == "" || config.NetworkID == ([32]byte{}) || !canonicalTime(config.NotBefore) ||
		!canonicalTime(config.NotAfter) || !config.NotAfter.After(config.NotBefore) {
		return nil, ErrInvalid
	}
	path, err := admittedRoot(config.Root)
	if err != nil {
		return nil, err
	}
	if err := prepareRoot(path); err != nil {
		return nil, err
	}
	root, err := openPrepared(path)
	if err != nil {
		return nil, err
	}
	if root.state.present() {
		if root.state.NetworkID != config.NetworkID || root.state.NotBefore != config.NotBefore.Unix() ||
			root.state.NotAfter != config.NotAfter.Unix() {
			_ = root.Close()
			return nil, ErrInvalid
		}
		return root, nil
	}
	state, err := generateState(config)
	if err != nil {
		_ = root.Close()
		return nil, fmt.Errorf("generate Service Instance root: %w", err)
	}
	if err := writeState(path, state); err != nil {
		state.erase()
		_ = root.Close()
		return nil, err
	}
	root.state = state
	return root, nil
}

// Open reopens one initialized host root and holds its exclusive process lock.
func Open(path string) (*Root, error) {
	admitted, err := admittedRoot(path)
	if err != nil {
		return nil, err
	}
	root, err := openPrepared(admitted)
	if err != nil {
		return nil, err
	}
	if !root.state.present() {
		_ = root.Close()
		return nil, ErrInvalid
	}
	return root, nil
}

func openPrepared(path string) (*Root, error) {
	if err := validateMarker(path); err != nil {
		return nil, err
	}
	lock, err := acquireRootLock(filepath.Join(path, lockName))
	if err != nil {
		return nil, err
	}
	state, err := readState(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		_ = lock.release()
		return nil, err
	}
	if err := validateRootEntries(path, state.present()); err != nil {
		state.erase()
		_ = lock.release()
		return nil, err
	}
	return &Root{path: path, lock: lock, state: state}, nil
}

// Request returns a fresh copy of the stable public credential request.
func (root *Root) Request() ([]byte, error) {
	root.mu.Lock()
	defer root.mu.Unlock()
	if root.closed || !root.state.present() {
		return nil, ErrClosed
	}
	view, err := root.state.requestView()
	if err != nil {
		return nil, err
	}
	return encodeRequest(view), nil
}

// Close erases in-memory private material and releases the exclusive lock.
func (root *Root) Close() error {
	root.mu.Lock()
	defer root.mu.Unlock()
	if root.closed {
		return ErrClosed
	}
	root.closed = true
	root.bindingOpen = false
	root.state.erase()
	return root.lock.release()
}

func canonicalTime(value time.Time) bool {
	return !value.IsZero() && value.Equal(value.UTC().Truncate(time.Second))
}
