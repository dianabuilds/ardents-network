package alpha

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

const (
	persistentFloorMarkerName = ".ardents-alpha-corpus-floor-v1"
	persistentFloorMarker     = "ardents-alpha-corpus-floor-v1\n"
	persistentFloorFile       = "corpus-floor.bin"
	persistentFloorLock       = ".ardents-alpha-corpus-floor.lock"
	persistentFloorNext       = "corpus-floor.next"
)

// PersistentFloorConfig identifies one Endpoint-owned corpus-state root and
// pins the only corpus authority/cohort/Network it may retain. Its root is
// protected for the current platform user before it can be claimed.
type PersistentFloorConfig struct {
	Root      string
	Authority ed25519.PublicKey
	Cohort    string
	Network   [32]byte
}

// PersistentFloor stores the latest accepted signed corpus and its floor
// atomically across restart. It never obtains corpus bytes or creates an
// Endpoint readiness decision.
type PersistentFloor struct {
	root      string
	authority ed25519.PublicKey
	cohort    string
	network   [32]byte
	lease     persistentFloorLease

	mu     sync.Mutex
	serial uint64
	digest [32]byte
	corpus *Corpus
	closed bool
}

// OpenPersistentFloor opens an empty owned root or restores one accepted
// corpus/floor. It rejects a root with unexpected entries rather than treating
// it as a cache to repair.
func OpenPersistentFloor(config PersistentFloorConfig) (*PersistentFloor, error) {
	if config.Root == "" || len(config.Authority) != ed25519.PublicKeySize || !validCohort(config.Cohort) || config.Network == [32]byte{} {
		return nil, errors.New("alpha persistent floor configuration is invalid")
	}
	root, err := filepath.Abs(config.Root)
	if err != nil {
		return nil, err
	}
	if err := inspectPersistentFloorRoot(root); err != nil {
		return nil, err
	}
	lease, err := acquirePersistentFloorLease(root)
	if err != nil {
		return nil, errors.New("alpha persistent floor root is already active")
	}
	if err := recoverPersistentFloorRoot(root); err != nil {
		_ = lease.release()
		return nil, err
	}
	serial, digest, corpus, err := readPersistentFloor(filepath.Join(root, persistentFloorFile), config)
	if err != nil {
		_ = lease.release()
		return nil, err
	}
	return &PersistentFloor{root: root, authority: append(ed25519.PublicKey(nil), config.Authority...), cohort: config.Cohort,
		network: config.Network, lease: lease, serial: serial, digest: digest, corpus: corpus}, nil
}

// Observe atomically retains a later verified corpus, refuses rollback and
// same-serial conflict, and permits an exact repeat of the retained bytes.
func (floor *PersistentFloor) Observe(corpus *Corpus) error {
	if floor == nil || corpus == nil || corpus.Cohort() != floor.cohort || corpus.Network() != floor.network {
		return errors.New("alpha persistent floor input is invalid")
	}
	verified, err := OpenCorpus(floor.authority, corpus.Bytes())
	if err != nil || verified.Serial() != corpus.Serial() {
		return errors.New("alpha persistent floor corpus authority is invalid")
	}
	digest := corpus.Digest()
	floor.mu.Lock()
	defer floor.mu.Unlock()
	if floor.closed {
		return errors.New("alpha persistent floor is closed")
	}
	if corpus.Serial() < floor.serial {
		return &ResolutionError{Failure: FailureStale}
	}
	if corpus.Serial() == floor.serial && floor.serial != 0 && digest != floor.digest {
		return &ResolutionError{Failure: FailureConflict}
	}
	if corpus.Serial() == floor.serial {
		return nil
	}
	if err := writePersistentFloor(filepath.Join(floor.root, persistentFloorFile), floor.cohort, floor.network, floor.authority, corpus); err != nil {
		return err
	}
	floor.serial, floor.digest, floor.corpus = corpus.Serial(), digest, verified
	return nil
}

// Current returns the accepted signed corpus retained across restart. Its
// caller must still use ValidAt or Resolve with an explicit decision time.
func (floor *PersistentFloor) Current() (*Corpus, error) {
	if floor == nil {
		return nil, errors.New("alpha persistent floor is nil")
	}
	floor.mu.Lock()
	defer floor.mu.Unlock()
	if floor.closed || floor.corpus == nil {
		return nil, errors.New("alpha persistent floor has no accepted corpus")
	}
	return OpenCorpus(floor.authority, floor.corpus.Bytes())
}

// Close releases only this floor's exclusive root lease and leaves retained
// corpus evidence in place.
func (floor *PersistentFloor) Close() error {
	if floor == nil {
		return nil
	}
	floor.mu.Lock()
	defer floor.mu.Unlock()
	if floor.closed {
		return nil
	}
	floor.closed = true
	return floor.lease.release()
}

// inspectPersistentFloorRoot establishes an owner-only root without changing
// its contents. Recovery runs only after the caller holds its exclusive lease,
// so another opener can never delete an active writer's temporary successor.
func inspectPersistentFloorRoot(root string) error {
	info, err := os.Lstat(root)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(root, 0o700); err != nil {
			return err
		}
		info, err = os.Lstat(root)
	}
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("alpha persistent floor root is invalid")
	}
	return validatePersistentFloorRootPermissions(root, info)
}

// recoverPersistentFloorRoot claims an empty leased root or validates one
// already claimed root. It removes only an interrupted successor left by the
// holder of that lease.
func recoverPersistentFloorRoot(root string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	markerPath := filepath.Join(root, persistentFloorMarkerName)
	markerInfo, markerErr := os.Lstat(markerPath)
	if os.IsNotExist(markerErr) {
		if len(entries) != 1 || entries[0].Name() != persistentFloorLock {
			return errors.New("alpha persistent floor refuses an unowned root")
		}
		if err := writePersistentFloorMarker(markerPath); err != nil {
			return err
		}
		return syncPersistentFloorDirectory(root)
	}
	if markerErr != nil || !markerInfo.Mode().IsRegular() || markerInfo.Mode()&os.ModeSymlink != 0 {
		return errors.New("alpha persistent floor root marker is invalid")
	}
	marker, err := os.ReadFile(markerPath)
	if err != nil || string(marker) != persistentFloorMarker {
		return errors.New("alpha persistent floor root marker is invalid")
	}
	for _, entry := range entries {
		if entry.Name() != persistentFloorMarkerName && entry.Name() != persistentFloorFile && entry.Name() != persistentFloorLock && entry.Name() != persistentFloorNext {
			return errors.New("alpha persistent floor root has an unknown entry")
		}
		info, inspectErr := os.Lstat(filepath.Join(root, entry.Name()))
		if inspectErr != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("alpha persistent floor root entry is invalid")
		}
	}
	next := filepath.Join(root, persistentFloorNext)
	if info, err := os.Lstat(next); err == nil {
		if !info.Mode().IsRegular() {
			return errors.New("alpha persistent floor temporary file is invalid")
		}
		return os.Remove(next)
	} else if !os.IsNotExist(err) {
		return err
	}
	return nil
}

func writePersistentFloorMarker(path string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err = file.WriteString(persistentFloorMarker); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func readPersistentFloor(path string, config PersistentFloorConfig) (uint64, [32]byte, *Corpus, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return 0, [32]byte{}, nil, nil
	}
	if err != nil || len(raw) < 4+1+1+32+32+8+32+2 || string(raw[:4]) != "ANF1" || raw[4] != 1 {
		return 0, [32]byte{}, nil, errors.New("alpha persistent floor is invalid")
	}
	offset, cohortLength := 5, int(raw[5])
	offset++
	if cohortLength == 0 || offset+cohortLength+32+32+8+32+2 > len(raw) || string(raw[offset:offset+cohortLength]) != config.Cohort {
		return 0, [32]byte{}, nil, errors.New("alpha persistent floor binding is invalid")
	}
	offset += cohortLength
	if string(raw[offset:offset+32]) != string(config.Network[:]) {
		return 0, [32]byte{}, nil, errors.New("alpha persistent floor network is invalid")
	}
	offset += 32
	if sha256.Sum256(config.Authority) != [32]byte(raw[offset:offset+32]) {
		return 0, [32]byte{}, nil, errors.New("alpha persistent floor authority is invalid")
	}
	offset += 32
	serial := binary.BigEndian.Uint64(raw[offset : offset+8])
	offset += 8
	var digest [32]byte
	copy(digest[:], raw[offset:offset+32])
	offset += 32
	corpusLength := int(binary.BigEndian.Uint16(raw[offset : offset+2]))
	offset += 2
	if serial == 0 || corpusLength == 0 || offset+corpusLength != len(raw) {
		return 0, [32]byte{}, nil, errors.New("alpha persistent floor content is invalid")
	}
	corpus, err := OpenCorpus(config.Authority, raw[offset:])
	if err != nil || corpus.Cohort() != config.Cohort || corpus.Network() != config.Network || corpus.Serial() != serial || corpus.Digest() != digest {
		return 0, [32]byte{}, nil, errors.New("alpha persistent floor corpus is invalid")
	}
	return serial, digest, corpus, nil
}

func writePersistentFloor(path, cohort string, network [32]byte, authority ed25519.PublicKey, corpus *Corpus) error {
	bytes := corpus.Bytes()
	if len(bytes) == 0 || len(bytes) > 0xffff {
		return errors.New("alpha persistent floor corpus is outside its bound")
	}
	raw := make([]byte, 0, 4+1+1+len(cohort)+32+32+8+32+2+len(bytes))
	raw = append(raw, 'A', 'N', 'F', '1', 1, byte(len(cohort)))
	raw = append(raw, cohort...)
	raw = append(raw, network[:]...)
	authorityDigest := sha256.Sum256(authority)
	raw = append(raw, authorityDigest[:]...)
	raw = binary.BigEndian.AppendUint64(raw, corpus.Serial())
	digest := corpus.Digest()
	raw = append(raw, digest[:]...)
	raw = binary.BigEndian.AppendUint16(raw, uint16(len(bytes)))
	raw = append(raw, bytes...)
	temporary := filepath.Join(filepath.Dir(path), persistentFloorNext)
	file, err := os.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err = file.Write(raw); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(temporary)
		return err
	}
	if err := durablePersistentFloorRename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("publish alpha persistent floor: %w", err)
	}
	return syncPersistentFloorDirectory(filepath.Dir(path))
}
