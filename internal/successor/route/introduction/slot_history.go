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
	mu           sync.Mutex
	transactions sync.Mutex // physical serialization, never the short denial lock
	slotSnapshot
	raw               []byte
	closed            bool
	physicalClosed    bool
	registryClaimed   bool // one live capacity owner for this independently leased root
	failure, closeErr error
	denial            error
	store             historyStorage
}

// Stop denies original effects synchronously without waiting for native I/O.
// It does not release custody; Close follows the original borrowers' join.
func (h *History) Stop(cause error) {
	if h == nil {
		return
	}
	h.mu.Lock()
	h.closed = true
	if h.denial == nil {
		h.denial = cause
	}
	h.mu.Unlock()
}

func (h *History) localCheck() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.failure != nil {
		return h.failure
	}
	if h.closed || h.store == nil {
		return errors.Join(errors.New("introduction history unavailable"), h.denial)
	}
	return nil
}

func (h *History) fail(err error) error {
	h.mu.Lock()
	if h.failure == nil {
		h.failure = err
	}
	retained := h.failure
	h.mu.Unlock()
	return retained
}

// Check observes only original native custody and this owner's short state.
// It never invokes Network, Node or a receiving authority observer.
func (h *History) Check() error {
	if h == nil {
		return errors.New("introduction history absent")
	}
	if err := h.localCheck(); err != nil {
		return err
	}
	h.transactions.Lock()
	defer h.transactions.Unlock()
	if err := h.localCheck(); err != nil {
		return err
	}
	if err := h.checkRetained(); err != nil {
		return h.fail(err)
	}
	return h.localCheck()
}

// Failure is the first operational/native cause, separately from local denial
// and physical release. Request refusals never latch it.
func (h *History) Failure() error {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.failure
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
	h.transactions.Lock()
	defer h.transactions.Unlock()
	if err := h.localCheck(); err != nil {
		return err
	}
	if now.Before(h.floor) {
		return errors.New("introduction clock below retained floor")
	}
	if err := h.checkRetained(); err != nil {
		return h.fail(err)
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
	return h.localCheck()
}

// Claim persists the original expiry and monotonic time floor before an ACK.
// Capacity, accepted allowance, authority and channel ownership are checked by
// the registration owner before this effect and again before acceptance.
func (h *History) Claim(slot [32]byte, expiry, now time.Time) error {
	if h == nil || slot == [32]byte{} || now.Unix() <= 0 || !now.Before(expiry) || expiry.After(now.Add(600*time.Second)) || expiry != expiry.UTC().Truncate(time.Second) {
		return errors.New("introduction slot claim invalid")
	}
	h.transactions.Lock()
	defer h.transactions.Unlock()
	if err := h.localCheck(); err != nil {
		return err
	}
	if now.Before(h.floor) {
		return errors.New("introduction clock below retained floor")
	}
	if err := h.checkRetained(); err != nil {
		return h.fail(err)
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
	if err := h.localCheck(); err != nil {
		return err
	}
	if err := h.store.commit(raw); err != nil {
		return h.fail(err)
	}
	h.entries, h.floor, h.raw = next, floor, raw
	return h.localCheck()
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
	h.Stop(nil)
	h.transactions.Lock()
	defer h.transactions.Unlock()
	h.mu.Lock()
	alreadyClosed := h.physicalClosed
	failure := h.failure
	h.mu.Unlock()
	if !alreadyClosed {
		// Denial does not discharge native custody. Reobserve after all original
		// transactions joined, including loss introduced during retirement.
		if failure == nil && len(h.raw) != 0 && h.store != nil {
			if err := h.checkRetained(); err != nil {
				failure = h.fail(err)
			}
		}
		var closeErr error
		if h.store == nil {
			closeErr = errors.Join(failure, errors.New("introduction history storage unavailable"))
		} else {
			closeErr = errors.Join(failure, h.store.close())
		}
		h.mu.Lock()
		h.closeErr = closeErr
		h.physicalClosed = true
		h.mu.Unlock()
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.closeErr
}
