package prefix

import (
	"context"
	"errors"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route/selection"
)

// This is an original-generation physical lifetime oracle. The actual prefix
// pipe readers and resource return are observed; no successful authority,
// Registration or JOIN is supplied by the controlled borrower.
func TestPrefixIdleRetirementJoinsStoppedBorrowBeforeParents(t *testing.T) {
	p, entry, idle, returns, cleanup := idlePhysicalPrefix(t, nil, nil)
	defer cleanup()
	interrupted, joined := make(chan struct{}), make(chan struct{})
	late := errors.New("original terminal borrower failed while joining")
	var interruption sync.Once
	// A stopped Registration no longer prevents idle retirement, but its
	// original physical work remains held until the terminal join returns.
	borrow := &Borrow{owner: p, interrupt: func() { interruption.Do(func() { close(interrupted) }) }, active: func() bool { return false }}
	borrow.join = func() error {
		<-joined
		borrow.ReturnJoined()
		return late
	}
	p.lifetimeMu.Lock()
	p.retainBorrowLocked(borrow)
	p.lifetimeMu.Unlock()
	idle <- time.Now()
	select {
	case <-interrupted:
	case <-time.After(time.Second):
		t.Fatal("Prefix did not interrupt its retained borrower")
	}
	select {
	case <-entry.readReturned:
		t.Fatal("parent retired before terminal borrower joined")
	default:
	}
	if returns.Load() != 0 {
		t.Fatal("original resources returned before borrower join")
	}
	select {
	case <-p.Done():
		t.Fatal("idle retirement completed before original borrower joined")
	default:
	}
	close(joined)
	select {
	case <-p.Done():
	case <-time.After(time.Second):
		t.Fatal("idle retirement failed to join")
	}
	err := p.Close()
	if !errors.Is(err, late) || p.Close() != err || returns.Load() != 1 {
		t.Fatal("joined borrower failure or exactly-once physical return lost", err, returns.Load())
	}
}

// Actual pipe parents qualify local generation/seal ordering only. No
// authenticated Source, Responder, JOIN readiness or Admission is supplied.
func TestPrefixPairCommitRefusesEitherSealedOriginal(t *testing.T) {
	for _, original := range []string{"source", "responder"} {
		t.Run(original, func(t *testing.T) {
			source, _, _, cleanupSource := terminalSetupPhysicalPrefix(t)
			defer cleanupSource()
			responder, _, _, cleanupResponder := terminalSetupPhysicalPrefix(t)
			defer cleanupResponder()
			if original == "source" {
				source.Seal()
			} else {
				responder.Seal()
			}
			transition := false
			err := source.CommitPair(responder, func(checkOriginal func() error) error {
				if err := checkOriginal(); err != nil {
					return err
				}
				transition = true
				return nil
			})
			if !errors.Is(err, net.ErrClosed) || transition {
				t.Fatal("sealed original generation allowed local handoff", err)
			}
		})
	}
}

func TestPrefixPairCommitHoldsBothOriginalSealsThroughLocalHandoff(t *testing.T) {
	source, _, _, cleanupSource := terminalSetupPhysicalPrefix(t)
	defer cleanupSource()
	responder, _, _, cleanupResponder := terminalSetupPhysicalPrefix(t)
	defer cleanupResponder()
	entered, finish := make(chan struct{}), make(chan struct{})
	var finishOnce sync.Once
	unblock := func() { finishOnce.Do(func() { close(finish) }) }
	defer unblock()
	result := make(chan error, 1)
	go func() {
		result <- source.CommitPair(responder, func(checkOriginal func() error) error {
			close(entered)
			<-finish
			return checkOriginal()
		})
	}()
	<-entered
	sourceSealed, responderSealed := make(chan struct{}), make(chan struct{})
	go func() { source.Seal(); close(sourceSealed) }()
	go func() { responder.Seal(); close(responderSealed) }()
	// Mutex waits are not durably blocking in synctest. The held original
	// locks provide an exact causal oracle, without a sleep or scheduling claim.
	for _, original := range []*Prefix{source, responder} {
		if original.lifetimeMu.TryLock() {
			original.lifetimeMu.Unlock()
			t.Error("pair handoff did not retain an original seal lock")
		}
	}
	for _, done := range []<-chan struct{}{sourceSealed, responderSealed} {
		select {
		case <-done:
			t.Error("original seal crossed the held pair handoff")
		default:
		}
	}
	unblock()
	if err := <-result; err != nil {
		t.Fatal("pair did not keep originals live through local handoff", err)
	}
	<-sourceSealed
	<-responderSealed
}

// An actual canceled original caller remains canceled after a failed read.
// Preserve both causes; derived cancellation propagation is deliberately held.
func TestPrefixOriginalObservationRechecksCallerAfterRead(t *testing.T) {
	original, revoke := context.WithCancel(t.Context())
	defer revoke()
	p, _, returns, cleanup := terminalSetupPhysicalPrefix(t, &prefixDeferredPropagation{Context: original})
	defer cleanup()
	failure := errors.New("original Network read failed after revocation")
	observations := 0
	p.config.Current = func() (network.RuntimeView, error) {
		observations++
		revoke()
		return network.RuntimeView{}, failure
	}
	err := p.originalCurrent()
	if !errors.Is(err, failure) || !errors.Is(err, context.Canceled) || observations != 1 || p.ctx.Err() != nil || returns.Load() != 0 {
		t.Fatalf("original observation lost revocation: error=%v observations=%d derived=%v returns=%d", err, observations, p.ctx.Err(), returns.Load())
	}
	if err := p.Close(); err != nil || returns.Load() != 1 {
		t.Fatal("physical failure-only fixture failed joined return", err, returns.Load())
	}
}

// These fixtures provide only local generation state, never authority or a
// successfully admitted channel. Genuine role opens remain command scenarios.
func handoffLifetimePrefix(t *testing.T, domain uint8) *Prefix {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	return &Prefix{ctx: ctx, caller: ctx, cancel: cancel, closing: make(chan struct{}), activity: make(chan struct{}, 1), config: Config{Leg: selection.Leg{EntryMember: network.Member{RoleDomain: domain}}}}
}

func TestResponderJoinRequiresImmutableOriginalSource(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		original, unrelated, responder := handoffLifetimePrefix(t, 1), handoffLifetimePrefix(t, 1), handoffLifetimePrefix(t, 3)
		if b, err := responder.BorrowResponderJoin(t.Context(), func() {}, func() error { return nil }); err == nil || b != nil {
			t.Fatal("unbound Responder acquired Source", b, err)
		}
		responder.source = original
		if b, err := unrelated.borrowJoin(t.Context(), responder, func() {}, func() error { return nil }); err == nil || b != nil {
			t.Fatal("foreign Source borrowed the original Responder", b, err)
		}
		if len(original.borrows) != 0 || len(unrelated.borrows) != 0 || len(responder.borrows) != 0 {
			t.Fatal("foreign Source refusal acquired pair capacity")
		}
		original.Seal()
		if b, err := responder.BorrowResponderJoin(t.Context(), func() {}, func() error { return nil }); err == nil || b != nil {
			t.Fatal("retired original Source acquired JOIN", b, err)
		}
		if len(original.borrows) != 0 || len(unrelated.borrows) != 0 || len(responder.borrows) != 0 || unrelated.localCurrent() != nil {
			t.Fatal("refusal borrowed/replaced original Source")
		}
		other, err := unrelated.BorrowSourceJoin(t.Context(), func() {}, func() error { return nil })
		if err != nil {
			t.Fatal("unrelated live Source unavailable", err)
		}
		if other.source != unrelated || responder.source != original {
			t.Fatal("unrelated acquisition replaced bound original")
		}
		other.ReturnJoined()
		if len(unrelated.borrows) != 0 {
			t.Fatal("original borrow retained after joined return")
		}
		if b, err := responder.BorrowSourceJoin(t.Context(), func() {}, func() error { return nil }); b != nil || err == nil {
			t.Fatal("Responder substituted Source")
		}
	})
}

func TestJoinPublicationLinearizesWithOriginalRoleSeal(t *testing.T) {
	for _, role := range []string{"Source", "Responder"} {
		t.Run(role, func(t *testing.T) {
			source, responder := handoffLifetimePrefix(t, 1), handoffLifetimePrefix(t, 3)
			responder.source = source
			borrow, err := responder.BorrowResponderJoin(t.Context(), func() {}, func() error { return nil })
			if err != nil {
				t.Fatal(err)
			}
			defer borrow.ReturnJoined()
			var streamMu sync.Mutex
			streamMu.Lock()
			locked := true
			defer func() {
				if locked {
					streamMu.Unlock()
				}
			}()
			sealing := source
			if role == "Responder" {
				sealing = responder
			}
			sealed := make(chan struct{})
			go func() { sealing.Seal(); close(sealed) }()
			<-sealing.closing
			published := false
			result := make(chan error, 1)
			go func() {
				result <- borrow.Publish(t.Context(), t.Context(), t.Context(), func(check func() error) error {
					streamMu.Lock()
					defer streamMu.Unlock()
					if err := check(); err != nil {
						return err
					}
					published = true
					return nil
				})
			}()
			streamMu.Unlock()
			locked = false
			<-sealed
			if err := <-result; err == nil {
				t.Fatal("stream published after original role Seal")
			}
			if published || borrow.source != source || borrow.responder != responder {
				t.Fatal("late handoff changed stream or exact original handles")
			}
			borrow.ReturnJoined()
			if len(source.borrows) != 0 || len(responder.borrows) != 0 {
				t.Fatal("original role borrows not returned")
			}
		})
	}
}

func TestJoinPublicationHoldsOriginalRoleLocksBeforeStreamMutex(t *testing.T) {
	source, responder := handoffLifetimePrefix(t, 1), handoffLifetimePrefix(t, 3)
	responder.source = source
	borrow, err := responder.BorrowResponderJoin(t.Context(), func() {}, func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	defer borrow.ReturnJoined()
	caller, cancel := context.WithCancel(t.Context())
	cancel()
	var streamMu sync.Mutex
	streamMu.Lock()
	locked := true
	defer func() {
		if locked {
			streamMu.Unlock()
		}
	}()
	published := false
	result := make(chan error, 1)
	go func() {
		result <- borrow.Publish(caller, t.Context(), t.Context(), func(check func() error) error {
			streamMu.Lock()
			defer streamMu.Unlock()
			if err := check(); err != nil {
				return err
			}
			published = true
			return nil
		})
	}()
	limit := time.Now().Add(time.Second)
	for _, original := range []*Prefix{source, responder} {
		for original.lifetimeMu.TryLock() {
			original.lifetimeMu.Unlock()
			if time.Now().After(limit) {
				t.Fatal("publication bypassed original role registration lock before stream mutex")
			}
			runtime.Gosched()
		}
	}
	streamMu.Unlock()
	locked = false
	if err := <-result; err == nil {
		t.Fatal("canceled publication accepted stream")
	}
	if published {
		t.Fatal("canceled publication installed stream")
	}
}

func TestJoinAcquisitionRefusesCallerCanceledWhileWaitingRoleLock(t *testing.T) {
	for _, role := range []string{"Source", "Responder"} {
		t.Run(role, func(t *testing.T) {
			source, responder := handoffLifetimePrefix(t, 1), handoffLifetimePrefix(t, 3)
			responder.source = source
			caller, cancel := context.WithCancel(t.Context())
			defer cancel()
			observed := &handoffObservedCaller{Context: caller, observed: make(chan struct{})}
			held := source
			if role == "Responder" {
				held = responder
			}
			held.lifetimeMu.Lock()
			type outcome struct {
				borrow *JoinBorrow
				err    error
			}
			result := make(chan outcome, 1)
			go func() {
				b, err := responder.BorrowResponderJoin(observed, func() {}, func() error { return nil })
				result <- outcome{b, err}
			}()
			<-observed.observed
			cancel()
			held.lifetimeMu.Unlock()
			got := <-result
			source.lifetimeMu.Lock()
			sourceBorrows := len(source.borrows)
			source.lifetimeMu.Unlock()
			responder.lifetimeMu.Lock()
			responderBorrows := len(responder.borrows)
			responder.lifetimeMu.Unlock()
			if got.borrow != nil {
				got.borrow.ReturnJoined()
			}
			if got.err == nil || got.borrow != nil || sourceBorrows != 0 || responderBorrows != 0 {
				t.Fatalf("canceled caller handed off after %s lock wait: borrow=%p error=%v borrows=%d/%d", role, got.borrow, got.err, sourceBorrows, responderBorrows)
			}
		})
	}
}

type handoffObservedCaller struct {
	context.Context
	observed chan struct{}
	signaled atomic.Bool
}

func (c *handoffObservedCaller) Err() error {
	err := c.Context.Err()
	if c.signaled.CompareAndSwap(false, true) {
		close(c.observed)
	}
	return err
}

// Pair claims remain at both original owners while an interrupted operation
// still joins. Real framing readers and returns expose premature retirement.
func TestJoinPairBorrowRetainsBothParentsUntilOperationJoins(t *testing.T) {
	source, sourceDone, sourceReturns, sourceCleanup := terminalSetupPhysicalPrefix(t)
	defer sourceCleanup()
	responder, responderDone, responderReturns, responderCleanup := terminalSetupPhysicalPrefix(t)
	defer responderCleanup()
	source.config.Leg.EntryMember.RoleDomain = 1
	responder.config.Leg.EntryMember.RoleDomain = 3
	responder.source = source
	interrupted, joined := make(chan struct{}), make(chan struct{})
	var interruptedOnce, joinedOnce sync.Once
	unblock := func() { joinedOnce.Do(func() { close(joined) }) }
	defer unblock()
	var borrow *JoinBorrow
	var err error
	borrow, err = responder.BorrowResponderJoin(t.Context(), func() { interruptedOnce.Do(func() { close(interrupted) }) }, func() error { <-joined; borrow.ReturnJoined(); return nil })
	if err != nil {
		t.Fatal(err)
	}
	closed := make(chan error, 1)
	go func() { closed <- source.Close() }()
	<-interrupted
	for i, original := range []*Prefix{source, responder} {
		original.lifetimeMu.Lock()
		_, held := original.borrows[borrow.claims[i]]
		original.lifetimeMu.Unlock()
		if !held {
			t.Error("original pair claim returned before join")
		}
	}
	for _, done := range []<-chan struct{}{sourceDone, responderDone} {
		select {
		case <-done:
			t.Error("parent reader retired before pair operation joined")
		default:
		}
	}
	if sourceReturns.Load() != 0 || responderReturns.Load() != 0 {
		t.Error("parent resource returned before borrower joined")
	}
	select {
	case err := <-closed:
		t.Fatal("Prefix reported join before original borrower", err)
	default:
	}
	unblock()
	if err := <-closed; err != nil {
		t.Fatal(err)
	}
	if len(source.borrows) != 0 || len(responder.borrows) != 0 {
		t.Fatal("original pair claims retained after join")
	}
	if err := responder.Close(); err != nil {
		t.Fatal(err)
	}
	if sourceReturns.Load() != 1 || responderReturns.Load() != 1 {
		t.Fatal("physical parents did not return exactly once")
	}
}
