package issuance

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"os"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
)

var (
	ErrConflict = errors.New("request-conflict")
	ErrValidity = errors.New("outside-validity")
	ErrCapacity = errors.New("result-capacity")
)

const maximumResults = 6 * 65536
const maximumJournal = 256 << 20

var resultFiles = []string{"results.pin", "results.lock", "results.journal", "results.floor"}

type savedResult struct {
	raw, response []byte
	kind          admission.Kind
	checked       time.Time
}
type resultState struct {
	mu                sync.Mutex
	root              *os.Root
	path              string
	identity          os.FileInfo
	files             map[string]os.FileInfo
	lock              *os.File
	store             Store
	binding           admission.LedgerBinding
	header, tail      [32]byte
	floor             uint64
	size              int64
	signatures        int
	records           map[[32]byte]savedResult
	closed            bool
	failure, closeErr error
	fault             func(string) error
}

// ResultStore copies share one transaction owner, lease and terminal lifecycle.
type ResultStore struct{ state *resultState }

func InitializeResults(ctx context.Context, path string, store Store, b admission.LedgerBinding) error {
	if err := platform(); err != nil {
		return err
	}
	if ctx == nil {
		return ErrInvalid
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	v, err := matchingInventory(store, b)
	if err != nil {
		return err
	}
	return initializeResults(ctx, path, b, v.Digest, nil)
}
func OpenResults(ctx context.Context, path string, store Store, b admission.LedgerBinding) (ResultStore, error) {
	if err := platform(); err != nil {
		return ResultStore{}, err
	}
	if ctx == nil {
		return ResultStore{}, ErrInvalid
	}
	if ctx.Err() != nil {
		return ResultStore{}, ctx.Err()
	}
	v, err := matchingInventory(store, b)
	if err != nil {
		return ResultStore{}, err
	}
	// Use a protected binding snapshot rather than caller-owned key slices.
	canonical, err := resultPin(b, v.Digest)
	if err != nil {
		return ResultStore{}, err
	}
	return openResults(ctx, path, store, b, canonical, nil)
}

// Issue signs only verified durable debits; its response is always defensive.
func (r ResultStore) Issue(ctx context.Context, c admission.DebitConfirmation, now time.Time) ([]byte, bool, error) {
	if r.state == nil || ctx == nil {
		return nil, false, ErrInvalid
	}
	s := r.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, false, ErrClosed
	}
	if s.failure != nil {
		return nil, false, ErrUncertain
	}
	if ctx.Err() != nil {
		return nil, false, ctx.Err()
	}
	raw, b, kind, checked, valid := c.Snapshot()
	if !valid {
		return nil, false, ErrInvalid
	}
	digest, err := admission.BindingDigest(b)
	if err != nil || digest != s.bindingDigest() {
		return nil, false, ErrInvalid
	}
	if now.Nanosecond() != 0 || now.Unix() < 0 || now.Before(checked) || uint64(now.Unix()) < s.floor {
		return nil, false, ErrValidity
	}
	if outcome := admission.ValidateBatch(ctx, raw, b, now); outcome != admission.Accepted {
		if outcome == admission.Canceled {
			return nil, false, ctx.Err()
		}
		return nil, false, ErrValidity
	}
	if err = s.check(); err != nil {
		s.failure = err
		return nil, false, ErrUncertain
	}
	var id [32]byte
	copy(id[:], raw[236:268])
	prior, exists := s.records[id]
	if exists && (sha256.Sum256(prior.raw) != sha256.Sum256(raw) || prior.kind != kind) {
		return nil, false, ErrConflict
	}
	var response []byte
	if exists {
		response = prior.response
	} else {
		if len(s.records) >= maximumResults || s.signatures+int(raw[624]) > maximumResults {
			return nil, false, ErrCapacity
		}
		response, err = s.store.sign(ctx, raw)
		if err != nil {
			return nil, false, err
		}
	}
	if ctx.Err() != nil {
		return nil, false, ctx.Err()
	}
	request := append([]byte{byte(kind)}, binary.BigEndian.AppendUint64(nil, uint64(checked.Unix()))...)
	request = append(request, raw...)
	first, tail := resultFrame(s.tail, request)
	second, last := resultFrame(tail, response)
	if !exists && s.size+int64(len(first)+len(second)) > maximumJournal {
		return nil, false, ErrCapacity
	}
	// Once a floor/journal mutation begins, cancellation cannot refund or abort
	// the durability sequence. Any ambiguity terminalizes this shared owner.
	if uint64(now.Unix()) > s.floor {
		if err = s.saveFloor(uint64(now.Unix())); err != nil {
			s.failure = err
			return nil, false, errors.Join(ErrUncertain, err)
		}
	}
	if !exists {
		if err = s.appendPair(first, second); err != nil {
			s.failure = err
			return nil, false, errors.Join(ErrUncertain, err)
		}
		s.tail = last
		s.size += int64(len(first) + len(second))
		s.signatures += int(raw[624])
		s.records[id] = savedResult{raw: raw, response: append([]byte(nil), response...), kind: kind, checked: checked}
	}
	return append([]byte(nil), response...), exists, nil
}
func (s *resultState) bindingDigest() [32]byte { d, _ := admission.BindingDigest(s.binding); return d }
func (r ResultStore) Close() error {
	if r.state == nil {
		return ErrClosed
	}
	s := r.state
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.closed = true
		s.closeErr = errors.Join(s.failure, s.check(), releaseLock(s.lock), s.root.Close())
	}
	return s.closeErr
}
