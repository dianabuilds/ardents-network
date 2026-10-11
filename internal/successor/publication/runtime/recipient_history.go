package runtime

import (
	"context"
	"errors"
	"sync"
	"time"

	executionruntime "github.com/dianabuilds/ardents-network/internal/successor/execution/runtime"
)

// This registry identifies independent private histories; it shares neither
// their nonce maps nor their admission locks. Only the sealed original Session
// identity can locate a history. A new Publisher under it cannot reset it.
var recipientHistories = struct {
	sync.Mutex
	owners map[executionruntime.AdministrationContext]*recipientHistory
}{owners: make(map[executionruntime.AdministrationContext]*recipientHistory)}

type recipientHistory struct {
	mu       sync.Mutex
	ctx      context.Context
	closed   bool
	entries  map[[32]byte]*openingReservation
	attempts []time.Time
	last     time.Time
	users    sync.WaitGroup
	done     chan struct{}
}

type openingReservation struct {
	history     *recipientHistory
	nonce       [32]byte
	expires     time.Time
	retainUntil time.Time
	accepted    bool
	once        sync.Once
}

func retainRecipientHistory(original executionruntime.AdministrationContext, operation *executionruntime.Operation) (*recipientHistory, error) {
	if err := original.Check(operation); err != nil {
		return nil, err
	}
	recipientHistories.Lock()
	defer recipientHistories.Unlock()
	if history := recipientHistories.owners[original]; history != nil {
		return history, nil
	}
	history := newRecipientHistory(original)
	recipientHistories.owners[original] = history
	go func() {
		<-history.done
		if err, completed := original.Completion(); !completed || err != nil {
			return
		}
		recipientHistories.Lock()
		if recipientHistories.owners[original] == history {
			delete(recipientHistories.owners, original)
		}
		recipientHistories.Unlock()
	}()
	return history, nil
}

func newRecipientHistory(original executionruntime.AdministrationContext) *recipientHistory {
	history := &recipientHistory{ctx: original.Context(), entries: make(map[[32]byte]*openingReservation), done: make(chan struct{})}
	go func() {
		<-original.Done()
		history.mu.Lock()
		history.closed = true
		history.mu.Unlock()
		// Join every original opening even after terminal cleanup failure.
		// Such failure retains the sealed history and its registry identity,
		// just as Execution retains the original cleanup capacity.
		history.users.Wait()
		if err, completed := original.Completion(); completed && err == nil {
			history.mu.Lock()
			clear(history.entries)
			history.attempts = nil
			history.mu.Unlock()
		}
		close(history.done)
	}()
	return history
}

func (history *recipientHistory) reserve(nonce [32]byte, expiry, registrationEnd, now time.Time) (*openingReservation, error) {
	history.mu.Lock()
	defer history.mu.Unlock()
	if history.closed || history.ctx.Err() != nil || nonce == [32]byte{} ||
		!now.Before(expiry) || expiry.After(registrationEnd) || now.Before(history.last) {
		return nil, errors.New("Publisher opening history unavailable")
	}
	history.last = now
	for key, entry := range history.entries {
		if entry.accepted && !now.Before(entry.retainUntil) {
			delete(history.entries, key)
		}
	}
	if history.entries[nonce] != nil {
		return nil, errors.New("Publisher delivery nonce already reserved")
	}
	if len(history.entries) >= 2640 {
		return nil, errors.New("Publisher replay capacity unavailable")
	}
	retained := history.attempts[:0]
	for _, attempt := range history.attempts {
		if attempt.After(now.Add(-time.Second)) {
			retained = append(retained, attempt)
		}
	}
	history.attempts = retained
	if len(retained) >= 4 {
		return nil, errors.New("Publisher opening rate unavailable")
	}
	history.attempts = append(history.attempts, now)
	reservation := &openingReservation{history: history, nonce: nonce, expires: expiry, retainUntil: registrationEnd.Add(60 * time.Second)}
	history.entries[nonce] = reservation
	history.users.Add(1)
	return reservation, nil
}

// Only genuine downstream acceptance may call this private transition. No
// public flag or callback can supply acceptance. Subsequent release retains
// the original registration's replay horizon, independent of capsule expiry.
func (reservation *openingReservation) commit(now time.Time) error {
	history := reservation.history
	history.mu.Lock()
	defer history.mu.Unlock()
	if history.closed || history.ctx.Err() != nil || history.entries[reservation.nonce] != reservation ||
		!now.Before(reservation.expires) || now.Before(history.last) {
		return errors.New("Publisher replay commit unavailable")
	}
	history.last = now
	reservation.accepted = true
	return nil
}

func (reservation *openingReservation) release() {
	reservation.once.Do(func() {
		history := reservation.history
		history.mu.Lock()
		if !reservation.accepted && history.entries[reservation.nonce] == reservation {
			delete(history.entries, reservation.nonce)
		}
		history.mu.Unlock()
		history.users.Done()
	})
}
