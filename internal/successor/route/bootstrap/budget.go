package bootstrap

import (
	"errors"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/successor/route/channel"
)

const (
	LaneBytes       = uint64(128 << 10)
	Lifetime        = 10 * time.Second
	outputPerMinute = uint64(1 << 20)
	creditScale     = uint64(time.Minute)
)

// Budget belongs to one actual duty lifetime. Adjacencies borrow it; opening
// another connection never creates another burst or queued-work allowance.
type Budget struct {
	mu     sync.Mutex
	live   uint8
	last   time.Time
	credit uint64
	held   uint64
	queues *channel.Budget
}

func NewBudget() *Budget { return newBudget(time.Now()) }

func newBudget(now time.Time) *Budget {
	return &Budget{last: now, credit: LaneBytes * creditScale, queues: channel.NewBudget(256 << 10)}
}

// Adjacency is one physical adjacent connection, never a peer-supplied ID.
type Adjacency struct {
	state *adjacencyState
}

type adjacencyState struct {
	budget *Budget
	live   uint8
	sealed bool
}

func (b *Budget) Adjacency() *Adjacency { return &Adjacency{state: &adjacencyState{budget: b}} }

// Queues lends the same finite queued-work owner to every bootstrap channel.
func (b *Budget) Queues() *channel.Budget { return b.queues }

// Claim is an original finite lane reservation. Expiry denies effects but does
// not return its live capacity before the physical work has joined.
type Claim struct {
	state *claimState
}

type claimState struct {
	adjacency *Adjacency
	deadline  time.Time
	released  bool
}

func (a *Adjacency) Reserve(end time.Time) (*Claim, error) { return a.reserve(time.Now(), end) }

func (a *Adjacency) reserve(now, end time.Time) (*Claim, error) {
	if a == nil || a.state == nil || a.state.budget == nil || !now.Before(end) {
		return nil, errors.New("bootstrap original bound unavailable")
	}
	b := a.state.budget
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.queues == nil || a.state.sealed || a.state.live >= 4 || b.live >= 16 {
		return nil, errors.New("bootstrap live capacity unavailable")
	}
	if bound := now.Add(Lifetime); end.After(bound) {
		end = bound
	}
	a.state.live++
	b.live++
	return &Claim{state: &claimState{adjacency: a, deadline: end}}, nil
}

// Seal stops new work without reclaiming reservations from live borrowers.
func (a *Adjacency) Seal() {
	if a == nil || a.state == nil || a.state.budget == nil {
		return
	}
	a.state.budget.mu.Lock()
	a.state.sealed = true
	a.state.budget.mu.Unlock()
}

func (c *Claim) Deadline() time.Time {
	if c == nil || c.state == nil {
		return time.Time{}
	}
	return c.state.deadline
}

// Restriction derives the immutable Node OPEN byte from a real live claim.
// The receiving role must still authenticate its duty and bootstrap operation.
func (c *Claim) Restriction() (ardp.ChildRestriction, error) {
	if c == nil || c.state == nil || c.state.adjacency == nil || c.state.adjacency.state == nil || c.state.adjacency.state.budget == nil {
		return ardp.OrdinaryChild, errors.New("bootstrap claim absent")
	}
	b := c.state.adjacency.state.budget
	b.mu.Lock()
	defer b.mu.Unlock()
	if c.state.released || c.state.adjacency.state.sealed || !time.Now().Before(c.state.deadline) {
		return ardp.OrdinaryChild, errors.New("bootstrap claim retired")
	}
	return ardp.IssuerBootstrapChild, nil
}

// ChargeOutput reserves complete output before physical emission. A later
// failed write cannot restore it. Fractional credit survives refused attempts.
func (c *Claim) ChargeOutput(n uint64) error {
	if c == nil || c.state == nil || c.state.adjacency == nil || c.state.adjacency.state == nil || c.state.adjacency.state.budget == nil {
		return errors.New("bootstrap claim absent")
	}
	b := c.state.adjacency.state.budget
	b.mu.Lock()
	defer b.mu.Unlock()
	// Sample while serialized: an earlier caller can acquire the lock after
	// a later one, which must not invent a clock rollback on an honest host.
	return c.chargeLocked(time.Now(), n)
}

func (c *Claim) chargeLocked(now time.Time, n uint64) error {
	b := c.state.adjacency.state.budget
	if c.state.released || c.state.adjacency.state.sealed || !now.Before(c.state.deadline) {
		return errors.New("bootstrap output lifetime unavailable")
	}
	return b.chargeLocked(now, n)
}

// HoldTermination debits one complete CLOSE before accepting bootstrap work,
// including a child refused by the live-claim cap. Its once-only output permit
// grants no lane or authority. Unused or failed output never refunds the debit.
func (b *Budget) HoldTermination(end time.Time) (*Termination, error) {
	if b == nil {
		return nil, errors.New("bootstrap output owner absent")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.holdTerminationLocked(time.Now(), end)
}

// Termination is an opaque prepaid output reservation. Copies retain the same
// once-only state. Physical finish returns its held burst space, never its debit.
type Termination struct {
	state *terminationState
}

type terminationState struct {
	budget *Budget
	end    time.Time
	done   bool
}

func (b *Budget) holdTerminationLocked(now, end time.Time) (*Termination, error) {
	if !now.Before(end) {
		return nil, errors.New("bootstrap terminal bound unavailable")
	}
	if bound := now.Add(Lifetime); end.After(bound) {
		end = bound
	}
	if err := b.chargeLocked(now, ardp.HeaderSize+1); err != nil {
		return nil, err
	}
	b.held += ardp.HeaderSize + 1
	return &Termination{state: &terminationState{budget: b, end: end}}, nil
}

func (t *Termination) Emit() error {
	if t == nil || t.state == nil || t.state.budget == nil {
		return errors.New("bootstrap terminal permit absent")
	}
	t.state.budget.mu.Lock()
	defer t.state.budget.mu.Unlock()
	return t.emitLocked(time.Now())
}

func (t *Termination) emitLocked(now time.Time) error {
	if t.state.done || !now.Before(t.state.end) {
		return errors.New("bootstrap terminal permit unavailable")
	}
	// Settle under the old burst ceiling before returning held space. Delayed
	// terminal output must not coexist with an independently restored full burst.
	if err := t.state.budget.refillLocked(now); err != nil {
		return err
	}
	t.state.budget.held -= ardp.HeaderSize + 1
	t.state.done = true
	return nil
}

func (t *Termination) ReleaseAfterJoin() {
	if t == nil || t.state == nil || t.state.budget == nil {
		return
	}
	t.state.budget.mu.Lock()
	defer t.state.budget.mu.Unlock()
	t.releaseLocked(time.Now())
}

func (t *Termination) releaseLocked(now time.Time) {
	if !t.state.done {
		// A clock refusal cannot restore credit. The finite held space can
		// still return after join; later charges retain the original clock floor.
		_ = t.state.budget.refillLocked(now)
		t.state.budget.held -= ardp.HeaderSize + 1
		t.state.done = true
	}
}

func (b *Budget) chargeLocked(now time.Time, n uint64) error {
	if err := b.refillLocked(now); err != nil {
		return err
	}
	if n == 0 || n > LaneBytes || n*creditScale > b.credit {
		return errors.New("bootstrap output capacity unavailable")
	}
	b.credit -= n * creditScale
	return nil
}

func (b *Budget) refillLocked(now time.Time) error {
	if b.queues == nil || now.Before(b.last) {
		return errors.New("bootstrap output owner unavailable")
	}
	elapsed := now.Sub(b.last)
	maximum := (LaneBytes - b.held) * creditScale
	if elapsed >= time.Minute {
		b.credit = maximum
	} else if addition := uint64(elapsed) * outputPerMinute; addition >= maximum-b.credit {
		b.credit = maximum
	} else {
		b.credit += addition
	}
	b.last = now
	return nil
}

// ReleaseAfterJoin returns only this claim, once. Physical owners call it after
// their TLS/channel readers, writers and descendants have actually joined.
func (c *Claim) ReleaseAfterJoin() {
	if c == nil || c.state == nil || c.state.adjacency == nil || c.state.adjacency.state == nil || c.state.adjacency.state.budget == nil {
		return
	}
	b := c.state.adjacency.state.budget
	b.mu.Lock()
	defer b.mu.Unlock()
	if !c.state.released {
		c.state.released = true
		c.state.adjacency.state.live--
		b.live--
	}
}
