package alphacontrol

import (
	"crypto/ed25519"
	"errors"
	"path/filepath"
	"sync"
	"time"
)

const (
	readerMarkerName = ".ardents-alpha-control-reader-v1"
	readerMarker     = "ardents-alpha-control-reader-v1\n"
	readerFloorName  = "catalog-floor.bin"
	readerLockName   = ".ardents-alpha-control-reader.lock"
)

// ReaderConfig opens one reader-owned persistent catalog-floor root. It has no
// Endpoint, Release, Network State, Namespace, or Update root input.
type ReaderConfig struct {
	Root          string
	DisclosureKey ed25519.PublicKey
	ComponentKeys [3]ed25519.PublicKey
	Clock         func() time.Time
}

// Reader owns only a durable disclosure-catalog floor and its process lock.
// Component verification remains supplied by the caller as a closed operation.
type Reader struct {
	root          string
	disclosure    ed25519.PublicKey
	componentKeys [3]ed25519.PublicKey
	clock         func() time.Time
	lease         readerLease
	floor         Floor
	mu            sync.Mutex
	closed        bool
}

// OpenReader claims an empty owned root or reopens an existing reader root.
func OpenReader(config ReaderConfig) (*Reader, error) {
	if config.Root == "" || len(config.DisclosureKey) != ed25519.PublicKeySize || config.Clock == nil {
		return nil, errors.New("alpha control reader configuration is incomplete")
	}
	for _, root := range config.ComponentKeys {
		if len(root) != ed25519.PublicKeySize {
			return nil, errors.New("alpha control component root is invalid")
		}
	}
	root, err := filepath.Abs(config.Root)
	if err != nil {
		return nil, err
	}
	if err := prepareReaderRoot(root); err != nil {
		return nil, err
	}
	lease, err := acquireReaderLease(root)
	if err != nil {
		return nil, errors.New("alpha control reader root is already active")
	}
	floor, err := readFloor(filepath.Join(root, readerFloorName))
	if err != nil {
		_ = lease.release()
		return nil, err
	}
	reader := &Reader{root: root, disclosure: append(ed25519.PublicKey(nil), config.DisclosureKey...), clock: config.Clock, lease: lease, floor: floor}
	for index, root := range config.ComponentKeys {
		reader.componentKeys[index] = append(ed25519.PublicKey(nil), root...)
	}
	return reader, nil
}

// Inspect verifies the catalog at one captured reader time and durably records
// its floor before it returns an accepted catalog result. Component outcomes do
// not authorize any action and do not prevent later explicit reinspection.
func (reader *Reader) Inspect(raw []byte, components [3][]byte, verify ComponentVerifier) (Inspection, error) {
	if reader == nil || verify == nil {
		return Inspection{}, errors.New("alpha control reader inspection input is incomplete")
	}
	reader.mu.Lock()
	defer reader.mu.Unlock()
	if reader.closed {
		return Inspection{}, errors.New("alpha control reader is closed")
	}
	at := reader.clock().UTC()
	result, next, err := Inspect(raw, reader.disclosure, reader.componentKeys, components, reader.floor, at, verify)
	if err != nil {
		return result, err
	}
	if err := writeFloor(filepath.Join(reader.root, readerFloorName), next); err != nil {
		return Inspection{}, err
	}
	reader.floor = next
	return result, nil
}

// Close releases only the reader-owned lock. It never removes a floor or
// modifies component-owner storage.
func (reader *Reader) Close() error {
	if reader == nil {
		return nil
	}
	reader.mu.Lock()
	defer reader.mu.Unlock()
	if reader.closed {
		return nil
	}
	reader.closed = true
	return reader.lease.release()
}
