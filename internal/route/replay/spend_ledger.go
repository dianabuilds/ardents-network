package replay

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
	"time"
)

const (
	closedSpendLedgerName       = "closed-token-spends"
	closedSpendLockName         = ".ardents-closed-spend-lock"
	closedSpendLedgerMagic      = "ARDSPN01"
	closedSpendLedgerHeaderSize = 8 + 32 + 32 + 32 + 8
	closedSpendRecordSize       = 32 + 8 + 1
	maximumClosedSpends         = 2 * 65536
)

// Binding is the complete public receiving duty fact set that
// owns a token spend journal. A journal cannot be rebound to a successor.
type Binding struct {
	NetworkID, ProfileDigest, ReceiverNodeID [32]byte
	ReceiverDutyGeneration                   uint64
}

// Ledger durably burns a token before Route can enable a lane.
// It stores only the token digest and its receiver-local expiry, never a
// holder key, permission, Target, or Application data.
type Ledger struct {
	closed         bool
	failure        error
	slots          *IntroductionSlots
	mu             sync.Mutex
	path           string
	binding        Binding
	spent          map[[32]byte]time.Time
	lease          closedSpendLease
	openAppendFile func(string) (closedSpendAppendFile, error)
}

type closedSpendAppendFile interface {
	io.Seeker
	io.Writer
	Sync() error
	WriteAt([]byte, int64) (int, error)
	Close() error
}

// Open opens one exclusive receiving-duty spend journal.
func Open(root string, binding Binding) (*Ledger, error) {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || !validBinding(binding) {
		return nil, errors.New("closed spend ledger binding is invalid")
	}
	lease, err := acquireClosedSpendLease(filepath.Join(root, closedSpendLockName))
	if err != nil {
		return nil, err
	}
	fail := func(cause error) (*Ledger, error) { return nil, errors.Join(cause, lease.release()) }
	path := filepath.Join(root, closedSpendLedgerName)
	raw, err := readClosedSpendFile(path, closedSpendLedgerHeaderSize+maximumClosedSpends*closedSpendRecordSize)
	if errors.Is(err, os.ErrNotExist) {
		ledger := &Ledger{path: path, binding: binding, spent: make(map[[32]byte]time.Time), lease: lease}
		if err := writeClosedSpendExclusive(path, encodeClosedSpendHeader(binding)); err != nil {
			return fail(err)
		}
		return ledger, nil
	}
	if err != nil {
		return fail(err)
	}
	ledger, err := decodeLedger(path, binding, raw)
	if err != nil {
		return fail(err)
	}
	ledger.lease = lease
	return ledger, nil
}

// Close releases the process-held journal lease. It does not erase spent
// records, which remain authoritative for the accepted duty/window.
func (ledger *Ledger) Close() error {
	if ledger == nil {
		return nil
	}
	ledger.mu.Lock()
	defer ledger.mu.Unlock()
	ledger.closed = true
	if ledger.slots != nil {
		ledger.slots.closed = true
	}
	err := ledger.lease.release()
	ledger.lease = closedSpendLease{}
	return errors.Join(ledger.failure, err)
}

// Spend durably records a token digest before granting the caller work. A
// duplicate or an ambiguous write is unavailable; expiry never revives a
// token in memory and only bounds retained journal history after reopen.
func (ledger *Ledger) Spend(token []byte, window, now time.Time) error {
	if ledger == nil || len(token) != 354 || !ValidWindow(window) || now.IsZero() {
		return errors.New("closed token spend is invalid")
	}
	now = now.UTC()
	if now.Before(window) || !now.Before(window.Add(time.Hour)) {
		return errors.New("closed token redemption window is unavailable")
	}
	digest := sha256.Sum256(token)
	ledger.mu.Lock()
	defer ledger.mu.Unlock()
	if ledger.closed {
		return errors.New("closed spend owner released")
	}
	if ledger.failure != nil {
		return ledger.failure
	}
	if err := ledger.prune(now); err != nil {
		ledger.failure = err
		return err
	}
	if _, found := ledger.spent[digest]; found {
		return errors.New("closed token is already spent")
	}
	if len(ledger.spent) >= maximumClosedSpends {
		return errors.New("closed token spend journal is exhausted")
	}
	if err := ledger.appendRecord(digest, window); err != nil {
		ledger.failure = err
		return err
	}
	ledger.spent[digest] = window
	return nil
}

func (ledger *Ledger) prune(now time.Time) error {
	retained := make(map[[32]byte]time.Time, len(ledger.spent))
	for digest, window := range ledger.spent {
		if now.Before(window.Add(time.Hour + time.Minute)) {
			retained[digest] = window
		}
	}
	if len(retained) == len(ledger.spent) {
		return nil
	}
	if err := replaceLedger(ledger.path, ledger.binding, retained); err != nil {
		return err
	}
	ledger.spent = retained
	return nil
}

func (ledger *Ledger) appendRecord(digest [32]byte, window time.Time) error {
	openFile := ledger.openAppendFile
	if openFile == nil {
		openFile = func(path string) (closedSpendAppendFile, error) { return os.OpenFile(path, os.O_RDWR, 0) }
	}
	return appendClosedSpendRecord(openFile, ledger.path, digest, window)
}

func validBinding(binding Binding) bool {
	return binding.NetworkID != [32]byte{} && binding.ProfileDigest != [32]byte{} && binding.ReceiverNodeID != [32]byte{} && binding.ReceiverDutyGeneration != 0
}

// ValidWindow accepts only a canonical UTC hourly token-spend window.
func ValidWindow(window time.Time) bool {
	return !window.IsZero() && window == window.UTC() && window == window.Truncate(time.Hour)
}

func encodeClosedSpendHeader(binding Binding) []byte {
	raw := make([]byte, 0, closedSpendLedgerHeaderSize)
	raw = append(raw, closedSpendLedgerMagic...)
	for _, value := range [][32]byte{binding.NetworkID, binding.ProfileDigest, binding.ReceiverNodeID} {
		raw = append(raw, value[:]...)
	}
	return binary.BigEndian.AppendUint64(raw, binding.ReceiverDutyGeneration)
}

func decodeLedger(path string, binding Binding, raw []byte) (*Ledger, error) {
	return decodeLedgerWithRepair(path, binding, raw, truncateLedger)
}

// decodeLedgerWithRepair accepts only one final crash tail. A
// complete record after an incomplete one is ambiguous and must remain intact
// for operator investigation rather than silently discarding a spent token.
func decodeLedgerWithRepair(path string, binding Binding, raw []byte, repair func(string, int64) error) (*Ledger, error) {
	if len(raw) < closedSpendLedgerHeaderSize || !bytes.Equal(raw[:8], []byte(closedSpendLedgerMagic)) || !bytes.Equal(raw[:closedSpendLedgerHeaderSize], encodeClosedSpendHeader(binding)) {
		return nil, errors.New("closed spend ledger cannot be rebound")
	}
	ledger := &Ledger{path: path, binding: binding, spent: make(map[[32]byte]time.Time)}
	offset := closedSpendLedgerHeaderSize
	complete := len(raw) - (len(raw)-offset)%closedSpendRecordSize
	repairAt := -1
	for offset < complete {
		digest, window, committed := decodeClosedSpendRecord(raw[offset : offset+closedSpendRecordSize])
		if !committed {
			// A zero marker is written before the commit byte. It is recoverable
			// only when this is the exact final record, with no later bytes.
			if raw[offset+closedSpendRecordSize-1] != 0 || offset+closedSpendRecordSize != len(raw) {
				return nil, errors.New("closed spend ledger recovery is ambiguous")
			}
			repairAt = offset
			break
		}
		if !ValidWindow(window) || digest == [32]byte{} || ledger.spent[digest] != (time.Time{}) || len(ledger.spent) >= maximumClosedSpends {
			return nil, errors.New("closed spend ledger record is invalid")
		}
		ledger.spent[digest] = window
		offset += closedSpendRecordSize
	}
	if repairAt < 0 && complete != len(raw) {
		repairAt = complete
	}
	if repairAt >= 0 {
		if err := repair(path, int64(repairAt)); err != nil {
			return nil, err
		}
	}
	return ledger, nil
}

func decodeClosedSpendRecord(raw []byte) ([32]byte, time.Time, bool) {
	var digest [32]byte
	if len(raw) != closedSpendRecordSize {
		return digest, time.Time{}, false
	}
	copy(digest[:], raw[:32])
	return digest, time.Unix(int64(binary.BigEndian.Uint64(raw[32:40])), 0).UTC(), raw[40] == 1
}

func appendClosedSpendRecord(openFile func(string) (closedSpendAppendFile, error), path string, digest [32]byte, window time.Time) error {
	raw := make([]byte, closedSpendRecordSize)
	copy(raw[:32], digest[:])
	binary.BigEndian.PutUint64(raw[32:40], uint64(window.Unix()))
	file, err := openFile(path)
	if err != nil {
		return err
	}
	start, err := file.Seek(0, io.SeekEnd)
	if err == nil {
		_, err = file.Write(raw)
	}
	if err == nil {
		err = file.Sync()
	}
	if err == nil {
		_, err = file.WriteAt([]byte{1}, start+closedSpendRecordSize-1)
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func writeClosedSpendExclusive(path string, raw []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err = file.Write(raw); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func readClosedSpendFile(path string, maximum int) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	raw, err := io.ReadAll(io.LimitReader(file, int64(maximum)+1))
	closeErr := file.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if len(raw) > maximum {
		return nil, errors.New("closed spend ledger exceeds bound")
	}
	return raw, nil
}

func truncateLedger(path string, size int64) error {
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	if err = file.Truncate(size); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func replaceLedger(path string, binding Binding, spends map[[32]byte]time.Time) error {
	if len(spends) > maximumClosedSpends {
		return errors.New("closed spend ledger exceeds bound")
	}
	digests := make([][32]byte, 0, len(spends))
	for digest := range spends {
		digests = append(digests, digest)
	}
	sort.Slice(digests, func(left, right int) bool { return bytes.Compare(digests[left][:], digests[right][:]) < 0 })
	raw := encodeClosedSpendHeader(binding)
	for _, digest := range digests {
		window := spends[digest]
		if !ValidWindow(window) {
			return errors.New("closed spend ledger record is invalid")
		}
		record := make([]byte, closedSpendRecordSize)
		copy(record[:32], digest[:])
		binary.BigEndian.PutUint64(record[32:40], uint64(window.Unix()))
		record[40] = 1
		raw = append(raw, record...)
	}
	return replaceClosedReplayFile(path, raw)
}

func replaceClosedReplayFile(path string, raw []byte) error {
	root := filepath.Dir(path)
	temporary, err := os.CreateTemp(root, ".closed-spend-")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if err = temporary.Chmod(0o600); err == nil {
		_, err = temporary.Write(raw)
	}
	if err == nil {
		err = temporary.Sync()
	}
	closeErr := temporary.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Rename(temporaryPath, path); err != nil {
		return err
	}
	return syncClosedSpendDirectory(root)
}
