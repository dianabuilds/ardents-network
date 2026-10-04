//go:build linux

package introduction

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission/spending"
)

const (
	slotHistoryName = "closed-introduction-slots"
	slotLeaseName   = ".ardents-introduction-lock"
	slotHeaderSize  = 120
	slotMaximum     = 1024
)

// Binding is the exact public duty binding of one independent Route history.
// It is not a Network authority observation.
type Binding struct {
	NetworkID, ProfileDigest, ReceiverNodeID [32]byte
	ReceiverDutyGeneration                   uint64
}

// History owns its own exclusive root and non-reclaim/time-floor snapshot.
// Withdrawal and channel failure cannot erase a claim. Close is synchronous
// with its own transactions; transport composition must join before calling it.
type History struct {
	mu                sync.Mutex
	binding           Binding
	path              string
	lease             *os.File
	entries           map[[32]byte]time.Time
	floor             time.Time
	raw               []byte
	closed            bool
	failure, closeErr error
	replace           func(string, []byte) error
}

// OpenHistory strictly reopens a complete retained root. Missing roots or
// members refuse; this operation never creates a marker or resets history.
func OpenHistory(root string, binding Binding) (*History, error) {
	return openHistory(root, binding, nil, syncHistory)
}

// InitializeHistory creates an empty independent root only from an opaque
// one-use fact of genuinely durable fresh Admission creation for this binding.
// Interrupted creation retains damage; reopen never reconstructs the fact.
func InitializeHistory(root string, binding Binding, fact *spending.FreshRoot) (*History, error) {
	if fact == nil {
		return nil, errors.New("introduction history initialization fact absent")
	}
	return openHistory(root, binding, fact, syncHistory)
}

func openHistory(root string, binding Binding, fact *spending.FreshRoot, syncRoot func(string) error) (_ *History, result error) {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || binding.NetworkID == [32]byte{} || binding.ProfileDigest == [32]byte{} || binding.ReceiverNodeID == [32]byte{} || binding.ReceiverDutyGeneration == 0 {
		return nil, errors.New("introduction history binding invalid")
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0o077 != 0 {
		return nil, errors.Join(errors.New("introduction history private root unavailable"), err)
	}
	names, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var initialization *spending.RootInitialization
	if fact != nil {
		if len(names) != 0 {
			return nil, errors.New("introduction history initialization requires vacant root")
		}
		initialization, err = fact.Begin(spending.Binding{NetworkID: binding.NetworkID, ProfileDigest: binding.ProfileDigest, ReceiverNodeID: binding.ReceiverNodeID, ReceiverDutyGeneration: binding.ReceiverDutyGeneration})
		if err != nil {
			return nil, err
		}
		// Completion consumes the handoff even on failed I/O. The failed result
		// below cannot publish an owner; actual filesystem bytes remain retained.
		defer func() {
			if result != nil {
				result = errors.Join(result, initialization.Complete())
			}
		}()
	} else {
		if len(names) != 2 {
			return nil, errors.New("introduction history retained members missing or foreign")
		}
		for _, name := range names {
			if name.Name() != slotLeaseName && name.Name() != slotHistoryName {
				return nil, errors.New("introduction history retained member foreign")
			}
		}
	}
	flags := os.O_RDWR | syscall.O_NOFOLLOW | syscall.O_NONBLOCK
	if initialization != nil {
		if err := initialization.Check(); err != nil {
			return nil, err
		}
		flags |= os.O_CREATE | os.O_EXCL
	}
	lease, err := os.OpenFile(filepath.Join(root, slotLeaseName), flags, 0o600)
	if err != nil {
		return nil, err
	}
	h := &History{binding: binding, path: filepath.Join(root, slotHistoryName), lease: lease, entries: make(map[[32]byte]time.Time)}
	defer func() {
		if result != nil {
			result = errors.Join(result, h.Close())
		}
	}()
	info, err = lease.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 || info.Size() != 0 {
		return nil, errors.Join(errors.New("introduction history lease invalid"), err)
	}
	if err := syscall.Flock(int(lease.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return nil, errors.New("introduction history already owned")
	}
	if initialization != nil {
		if err := initialization.Check(); err != nil {
			return nil, err
		}
		if err := lease.Sync(); err != nil {
			return nil, err
		}
		file, err := os.OpenFile(h.path, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0o600)
		if err != nil {
			return nil, err
		}
		raw := h.encode(h.entries, time.Time{})
		_, writeErr := file.Write(raw)
		if writeErr == nil {
			writeErr = file.Sync()
		}
		if err := errors.Join(writeErr, file.Close()); err != nil {
			return nil, err
		}
	}
	raw, err := readHistory(h.path)
	if err != nil {
		return nil, err
	}
	if err := h.decode(raw); err != nil {
		return nil, err
	}
	if initialization != nil {
		if err := initialization.Check(); err != nil {
			return nil, err
		}
	}
	if err := syncRoot(h.path); err != nil {
		return nil, err
	}
	if initialization != nil {
		if err := initialization.Complete(); err != nil {
			return nil, err
		}
	}
	h.raw = bytes.Clone(raw)
	return h, nil
}

// Claim persists the original expiry and monotonic time floor before an ACK.
// Capacity, accepted allowance, authority and channel ownership are checked by
// the registration owner before this effect and again before acceptance.
func (h *History) Claim(slot [32]byte, expiry, now time.Time) error {
	if h == nil || slot == [32]byte{} || now.Unix() <= 0 || !now.Before(expiry) || expiry.After(now.Add(600*time.Second)) || expiry != expiry.UTC().Truncate(time.Second) {
		return errors.New("introduction slot claim invalid")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed || h.failure != nil {
		return errors.Join(errors.New("introduction history unavailable"), h.failure)
	}
	if now.Before(h.floor) {
		return errors.New("introduction clock below retained floor")
	}
	if err := h.checkRetained(); err != nil {
		h.failure = err
		return err
	}
	digest := sha256.Sum256(slot[:])
	if prior, found := h.entries[digest]; found && now.Before(prior) {
		return errors.New("introduction slot already claimed")
	}
	next := make(map[[32]byte]time.Time, len(h.entries)+1)
	for key, end := range h.entries {
		if now.Before(end) {
			next[key] = end
		}
	}
	if len(next) >= slotMaximum {
		return errors.New("introduction history capacity exhausted")
	}
	next[digest] = expiry
	floor := now.UTC().Truncate(time.Second)
	raw := h.encode(next, floor)
	replace := h.replace
	if replace == nil {
		replace = replaceHistory
	}
	if err := replace(h.path, raw); err != nil {
		h.failure = err
		return err
	}
	h.entries, h.floor, h.raw = next, floor, raw
	return nil
}

func (h *History) checkRetained() error {
	names, err := os.ReadDir(filepath.Dir(h.path))
	if err != nil || len(names) != 2 {
		return errors.Join(errors.New("introduction history members changed or lost"), err)
	}
	for _, name := range names {
		if name.Name() != slotLeaseName && name.Name() != slotHistoryName {
			return errors.New("introduction history member foreign")
		}
	}
	retained, err := os.Lstat(filepath.Join(filepath.Dir(h.path), slotLeaseName))
	if err != nil {
		return err
	}
	held, err := h.lease.Stat()
	if err != nil || !os.SameFile(held, retained) {
		return errors.Join(errors.New("introduction history lease lost"), err)
	}
	raw, err := readHistory(h.path)
	if err != nil || !bytes.Equal(raw, h.raw) {
		return errors.Join(errors.New("introduction history changed or lost"), err)
	}
	return nil
}

func (h *History) encode(entries map[[32]byte]time.Time, floor time.Time) []byte {
	raw := make([]byte, 0, slotHeaderSize+len(entries)*40)
	raw = append(raw, "ARDISL01"...)
	for _, id := range [][32]byte{h.binding.NetworkID, h.binding.ProfileDigest, h.binding.ReceiverNodeID} {
		raw = append(raw, id[:]...)
	}
	raw = binary.BigEndian.AppendUint64(raw, h.binding.ReceiverDutyGeneration)
	var seconds uint64
	if !floor.IsZero() {
		seconds = uint64(floor.Unix())
	}
	raw = binary.BigEndian.AppendUint64(raw, seconds)
	ids := make([][32]byte, 0, len(entries))
	for id := range entries {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(a, b int) bool { return bytes.Compare(ids[a][:], ids[b][:]) < 0 })
	for _, id := range ids {
		raw = append(raw, id[:]...)
		raw = binary.BigEndian.AppendUint64(raw, uint64(entries[id].Unix()))
	}
	return raw
}

func (h *History) decode(raw []byte) error {
	header := h.encode(nil, time.Time{})
	if len(raw) < slotHeaderSize || (len(raw)-slotHeaderSize)%40 != 0 || !bytes.Equal(raw[:112], header[:112]) {
		return errors.New("introduction history damaged or rebound")
	}
	seconds := binary.BigEndian.Uint64(raw[112:120])
	if seconds > 1<<63-1 {
		return errors.New("introduction time floor invalid")
	}
	if seconds != 0 {
		h.floor = time.Unix(int64(seconds), 0).UTC()
	}
	if len(raw) > slotHeaderSize && seconds == 0 {
		return errors.New("introduction claimed history lacks time floor")
	}
	var previous [32]byte
	for offset := slotHeaderSize; offset < len(raw); offset += 40 {
		var digest [32]byte
		copy(digest[:], raw[offset:offset+32])
		expiry := binary.BigEndian.Uint64(raw[offset+32 : offset+40])
		if digest == [32]byte{} || expiry <= seconds || expiry > 1<<63-1 || (offset != slotHeaderSize && bytes.Compare(previous[:], digest[:]) >= 0) {
			return errors.New("introduction history entry invalid")
		}
		h.entries[digest] = time.Unix(int64(expiry), 0).UTC()
		previous = digest
	}
	return nil
}

// Close retains the failure and one physical lease release result.
func (h *History) Close() error {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.closed {
		h.closed = true
		h.closeErr = errors.Join(h.failure, syscall.Flock(int(h.lease.Fd()), syscall.LOCK_UN), h.lease.Close())
	}
	return h.closeErr
}

func readHistory(path string) ([]byte, error) {
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	info, statErr := file.Stat()
	if statErr != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
		return nil, errors.Join(errors.New("introduction snapshot invalid"), statErr, file.Close())
	}
	raw, readErr := io.ReadAll(io.LimitReader(file, slotHeaderSize+slotMaximum*40+1))
	if err := errors.Join(readErr, file.Close()); err != nil {
		return nil, err
	}
	if len(raw) > slotHeaderSize+slotMaximum*40 {
		return nil, errors.New("introduction history exceeds bound")
	}
	return raw, nil
}

func syncHistory(path string) error {
	file, err := os.OpenFile(path, os.O_RDWR|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	if err := errors.Join(file.Sync(), file.Close()); err != nil {
		return err
	}
	return syncHistoryDirectory(filepath.Dir(path))
}

func syncHistoryDirectory(root string) error {
	directory, err := os.Open(root)
	if err != nil {
		return err
	}
	return errors.Join(directory.Sync(), directory.Close())
}

func replaceHistory(path string, raw []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".introduction-snapshot-")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer func() { _ = os.Remove(temporary) }()
	_, writeErr := file.Write(raw)
	if writeErr == nil {
		writeErr = file.Sync()
	}
	if err := errors.Join(writeErr, file.Close()); err != nil {
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		return err
	}
	return syncHistoryDirectory(filepath.Dir(path))
}
