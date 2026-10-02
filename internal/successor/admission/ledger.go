package admission

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"os"
	"sync"
)

// Kind separates bootstrap accounting from admitted accounting. Neither
// category is proof that a transport or authenticated lane exists.
type Kind uint8

const (
	Bootstrap Kind = 1
	Admitted  Kind = 2
)

const (
	Initialized    Outcome = "initialized-offline"
	Debited        Outcome = "debited-offline"
	AlreadyDebited Outcome = "already-debited"
	Exhausted      Outcome = "quota-exhausted"
	Conflict       Outcome = "request-conflict"
	Unavailable    Outcome = "storage-unavailable"
	Uncertain      Outcome = "storage-uncertain"
	Busy           Outcome = "busy"
	Unsupported    Outcome = "unsupported-platform"
)

var (
	ErrInvalid     = errors.New("admission input invalid")
	ErrUnavailable = errors.New("admission storage unavailable")
	ErrUncertain   = errors.New("admission storage uncertain")
	ErrBusy        = errors.New("admission root busy")
	ErrUnsupported = errors.New("admission unsupported platform")
)

const maximumDebits = 6 * 65536

type quotaKey struct {
	permission [32]byte
	window     uint64
	class      uint8
}
type permissionQuota struct {
	commitment [32]byte
	maxima     [3]uint32
	bootstrap  uint8
}
type debitRecord struct {
	batch verifiedBatch
	kind  Kind
	now   uint64
}
type ledgerState struct {
	mu          sync.Mutex
	root        *os.Root
	path        string
	identity    os.FileInfo
	files       map[string]os.FileInfo
	lock        *os.File
	binding     LedgerBinding
	header      [32]byte
	tail        [32]byte
	floor       uint64
	records     map[[32]byte]debitRecord
	duty        map[uint64]uint64
	used        map[quotaKey]uint64
	permissions map[[32]byte]permissionQuota
	failure     error
	closed      bool
	closeErr    error
	// Fault controls stay private and instance-local; tests still use real files.
	fault func(string) error
}

// Ledger copies share one state, lease, mutation lock and terminal identity.
type Ledger struct{ state *ledgerState }

// Supported reports availability without touching files.
func Supported() bool { return admissionPlatform() == nil }

func Initialize(path string, binding LedgerBinding) error {
	if err := admissionPlatform(); err != nil {
		return err
	}
	if !binding.valid() {
		return ErrInvalid
	}
	return initializeLedger(path, binding)
}

func Open(path string, binding LedgerBinding) (*Ledger, error) {
	if err := admissionPlatform(); err != nil {
		return nil, err
	}
	if !binding.valid() {
		return nil, ErrInvalid
	}
	return openLedger(path, binding, nil)
}

// Debit durably consumes issuance right, never a refundable traffic reservation.
func (l *Ledger) Debit(ctx context.Context, raw []byte, f Facts, kind Kind) Outcome {
	if l == nil || l.state == nil || ctx == nil || kind != Bootstrap && kind != Admitted {
		return InvalidInput
	}
	s := l.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failure != nil {
		return Uncertain
	}
	if s.closed {
		return Unavailable
	}
	if ctx.Err() != nil {
		return Canceled
	}
	b, result := verifyBatch(ctx, raw, f, s.binding)
	if result != Accepted {
		return result
	}
	if f.Now.Unix() < 0 || f.Now.Nanosecond() != 0 || uint64(f.Now.Unix()) < s.floor {
		return Validity
	}
	if err := s.checkFiles(); err != nil {
		s.failure = err
		return Uncertain
	}
	if ctx.Err() != nil {
		return Canceled
	}
	// After the first filesystem mutation, finish or terminalize; cancellation
	// cannot erase a potentially committed debit.
	now := uint64(f.Now.Unix())
	if now > s.floor {
		if err := s.saveFloor(now); err != nil {
			s.failure = err
			return Uncertain
		}
		s.floor = now
	}
	if prior, ok := s.records[b.request]; ok {
		if prior.batch.digest != b.digest || prior.kind != kind {
			return Conflict
		}
		return AlreadyDebited
	}
	if prior, ok := s.permissions[b.permission]; ok && prior.commitment != b.commitment {
		return Conflict
	}
	if !s.capacity(b, kind) {
		return Exhausted
	}
	record := debitRecord{b, kind, now}
	if err := s.appendRecord(record); err != nil {
		s.failure = err
		return Uncertain
	}
	s.accept(record)
	return Debited
}

func (s *ledgerState) capacity(b verifiedBatch, kind Kind) bool {
	q := s.permissions[b.permission]
	return len(s.records) < maximumDebits && s.duty[b.window] <= 65536-uint64(b.count) && uint64(b.count) <= uint64(b.maxima[b.class-1]) && s.used[quotaKey{b.permission, b.window, b.class}] <= uint64(b.maxima[b.class-1])-uint64(b.count) && (kind != Bootstrap || q.bootstrap < 2)
}
func (s *ledgerState) accept(r debitRecord) {
	b := r.batch
	s.records[b.request] = r
	s.duty[b.window] += uint64(b.count)
	s.used[quotaKey{b.permission, b.window, b.class}] += uint64(b.count)
	q := s.permissions[b.permission]
	q.commitment = b.commitment
	q.maxima = b.maxima
	if r.kind == Bootstrap {
		q.bootstrap++
	}
	s.permissions[b.permission] = q
}

// Close preserves debits and reports the same cleanup result on repeated calls.
func (l *Ledger) Close() error {
	if l == nil || l.state == nil {
		return nil
	}
	s := l.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return s.closeErr
	}
	s.closed = true
	s.closeErr = errors.Join(s.inject("close"), releaseAdmissionLock(s.lock), s.root.Close())
	return s.closeErr
}
func (s *ledgerState) inject(phase string) error {
	if s.fault != nil {
		return s.fault(phase)
	}
	return nil
}

// Hash-chained fixed records retain the complete permission commitment to
// prevent changed maxima/holder under one permission ID from enlarging quota.
const recordSize = 32*4 + 8 + 1 + 1 + 2 + 12 + 8 + 32 + 1

func recordBytes(r debitRecord, tail [32]byte) []byte {
	b := r.batch
	raw := make([]byte, 0, recordSize)
	for _, id := range [][32]byte{b.request, b.digest, b.permission, b.commitment} {
		raw = append(raw, id[:]...)
	}
	raw = binary.BigEndian.AppendUint64(raw, b.window)
	raw = append(raw, b.class, byte(r.kind))
	raw = binary.BigEndian.AppendUint16(raw, b.count)
	for _, max := range b.maxima {
		raw = binary.BigEndian.AppendUint32(raw, max)
	}
	raw = binary.BigEndian.AppendUint64(raw, r.now)
	hash := sha256.New()
	_, _ = hash.Write(tail[:])
	_, _ = hash.Write(raw)
	raw = append(raw, hash.Sum(nil)...)
	return append(raw, 0)
}
