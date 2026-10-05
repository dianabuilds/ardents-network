package introduction

import (
	"crypto/sha256"
	"errors"
	"sync"
	"time"
)

// historyStorage is a private physical seam, not a caller-supplied authority.
// Native construction authenticates and leases the actual retained root before
// publishing History. A failed commit may have changed durable bytes; History
// retains that failure and never publishes a new accepting snapshot afterward.
type historyStorage interface {
	verify([]byte) error
	commit([]byte) error
	close() error
}

// History owns its own exclusive root and non-reclaim/time-floor snapshot.
// Withdrawal and channel failure cannot erase a claim. Close is synchronous
// with its own transactions; transport composition must join before calling it.
type History struct {
	mu sync.Mutex
	slotSnapshot
	raw               []byte
	closed            bool
	registryClaimed   bool // one live capacity owner for this independently leased root
	failure, closeErr error
	store             historyStorage
}

func (h *History) claimRegistry() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed || h.failure != nil || h.store == nil {
		return errors.New("introduction history unavailable")
	}
	if h.registryClaimed {
		return errors.New("introduction history already has a registry")
	}
	h.registryClaimed = true
	return nil
}

// checkCapacity checks the original durable occupancy and floor without
// pruning or writing. Its sole Registry serializes the pending claim transition.
func (h *History) checkCapacity(now time.Time, pending uint64) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed || h.failure != nil || h.store == nil || now.Before(h.floor) {
		return errors.New("introduction history unavailable")
	}
	if err := h.checkRetained(); err != nil {
		h.failure = err
		return err
	}
	retained := uint64(0)
	for _, expiry := range h.entries {
		if now.Before(expiry) {
			retained++
		}
	}
	if retained+pending >= slotMaximum {
		return errors.New("introduction capacity exhausted")
	}
	return nil
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
	if h.closed || h.failure != nil || h.store == nil {
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
	if err := h.store.commit(raw); err != nil {
		h.failure = err
		return err
	}
	h.entries, h.floor, h.raw = next, floor, raw
	return nil
}

func (h *History) checkRetained() error {
	if h.store == nil {
		return errors.New("introduction history storage unavailable")
	}
	return h.store.verify(h.raw)
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
		if h.store == nil {
			h.closeErr = errors.Join(h.failure, errors.New("introduction history storage unavailable"))
		} else {
			h.closeErr = errors.Join(h.failure, h.store.close())
		}
	}
	return h.closeErr
}
