//go:build linux

package transport

import (
	"context"
	"runtime"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network"
	"github.com/dianabuilds/ardents-network/internal/successor/route/selection"
)

// These fixtures supply only local lifetime state. They never authorize a
// Network view, admission, physical JOIN or successful stream publication.
func handoffLifetimePrefix(t *testing.T, domain uint8) *Prefix {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	return &Prefix{ctx: ctx, caller: ctx, cancel: cancel, closing: make(chan struct{}), activity: make(chan struct{}, 1), config: PrefixConfig{Leg: selection.Leg{EntryMember: network.Member{RoleDomain: domain}}}}
}

func TestResponderJoinRequiresImmutableOriginalSource(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		original, unrelated, responder := handoffLifetimePrefix(t, 1), handoffLifetimePrefix(t, 1), handoffLifetimePrefix(t, 3)
		if a, err := responder.AcquireResponderJoin(t.Context()); err == nil || a != nil {
			t.Fatal("unbound Responder acquired Source", a, err)
		}
		responder.source = original
		original.Seal()
		if a, err := responder.AcquireResponderJoin(t.Context()); err == nil || a != nil {
			t.Fatal("retired original Source acquired JOIN", a, err)
		}
		if len(original.joins) != 0 || len(unrelated.joins) != 0 || len(responder.joins) != 0 || unrelated.localCurrent() != nil {
			t.Fatal("refusal borrowed/replaced the original Source")
		}
		other, err := unrelated.AcquireSourceJoin(t.Context())
		if err != nil {
			t.Fatal("unrelated live Source unavailable", err)
		}
		if other.source != unrelated || responder.source != original {
			t.Fatal("unrelated acquisition replaced bound original")
		}
		if err := other.Close(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestJoinPublicationLinearizesWithOriginalRoleSeal(t *testing.T) {
	for _, role := range []string{"Source", "Responder"} {
		t.Run(role, func(t *testing.T) {
			source, responder := handoffLifetimePrefix(t, 1), handoffLifetimePrefix(t, 3)
			responder.source = source
			a, err := responder.AcquireResponderJoin(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer a.Close()
			a.mu.Lock()
			locked := true
			defer func() {
				if locked {
					a.mu.Unlock()
				}
			}()
			sealing := source
			if role == "Responder" {
				sealing = responder
			}
			sealed := make(chan struct{})
			go func() { sealing.Seal(); close(sealed) }()
			<-sealing.closing
			result := make(chan error, 1)
			go func() { result <- a.publish(t.Context(), &Joined{}) }()
			a.mu.Unlock()
			locked = false
			<-sealed
			if err := <-result; err == nil {
				t.Fatal("stream published after original role Seal")
			}
			a.mu.Lock()
			stream := a.stream
			a.mu.Unlock()
			if stream != nil || a.source != source || a.responder != responder {
				t.Fatal("late handoff changed stream or exact original handles")
			}
			if err := a.Close(); err != nil {
				t.Fatal(err)
			}
			if len(source.joins) != 0 || len(responder.joins) != 0 {
				t.Fatal("original role borrows not returned")
			}
		})
	}
}

// Publication must acquire both original registration locks before the stream
// mutex. The canceled caller makes this a refusal-only lifetime test.
func TestJoinPublicationHoldsOriginalRoleLocksBeforeStreamMutex(t *testing.T) {
	source, responder := handoffLifetimePrefix(t, 1), handoffLifetimePrefix(t, 3)
	responder.source = source
	a, err := responder.AcquireResponderJoin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	caller, cancel := context.WithCancel(t.Context())
	cancel()
	a.mu.Lock()
	locked := true
	defer func() {
		if locked {
			a.mu.Unlock()
		}
	}()
	result := make(chan error, 1)
	go func() { result <- a.publish(caller, &Joined{}) }()
	limit := time.Now().Add(time.Second)
	for _, original := range []*Prefix{source, responder} {
		for original.registrationMu.TryLock() {
			original.registrationMu.Unlock()
			if time.Now().After(limit) {
				t.Fatal("publication bypassed original role registration lock before stream mutex")
			}
			runtime.Gosched()
		}
	}
	a.mu.Unlock()
	locked = false
	if err := <-result; err == nil {
		t.Fatal("canceled publication accepted stream")
	}
	if a.stream != nil {
		t.Fatal("canceled publication installed stream")
	}
}
func TestJoinPublicationChecksExactOriginalCaller(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		source := handoffLifetimePrefix(t, 1)
		caller := &handoffDeferredCaller{Context: context.Background()}
		a, err := source.AcquireSourceJoin(caller)
		if err != nil {
			t.Fatal(err)
		}
		defer a.Close()
		caller.canceled.Store(true)
		if err := a.publish(t.Context(), &Joined{}); err == nil {
			// A mutant may wrongly install this nonphysical sentinel. Remove it
			// before fixture cleanup, which must not claim a real stream join.
			a.mu.Lock()
			a.stream = nil
			a.mu.Unlock()
			t.Fatal("original caller cancellation lost at publication")
		}
		a.mu.Lock()
		stream := a.stream
		a.mu.Unlock()
		if stream != nil {
			t.Fatal("canceled original caller published stream")
		}
	})
}

// Err changes before cancellation propagation, matching an original caller
// whose child has not yet observed cancellation at the final handoff.
type handoffDeferredCaller struct {
	context.Context
	canceled atomic.Bool
}

func (c *handoffDeferredCaller) Err() error {
	if c.canceled.Load() {
		return context.Canceled
	}
	return nil
}

func TestJoinAcquisitionRefusesCallerCanceledWhileWaitingRoleLock(t *testing.T) {
	for _, role := range []string{"Source", "Responder"} {
		t.Run(role, func(t *testing.T) {
			source, responder := handoffLifetimePrefix(t, 1), handoffLifetimePrefix(t, 3)
			responder.source = source
			caller, cancel := context.WithCancel(t.Context())
			observed := &handoffObservedCaller{Context: caller, observed: make(chan struct{})}
			held := source
			if role == "Responder" {
				held = responder
			}
			held.registrationMu.Lock()
			type outcome struct {
				a   *JoinAcquisition
				err error
			}
			result := make(chan outcome, 1)
			go func() { a, err := responder.AcquireResponderJoin(observed); result <- outcome{a, err} }()
			<-observed.observed // initial real caller Err returned nil before lock wait
			cancel()
			held.registrationMu.Unlock()
			got := <-result
			source.registrationMu.Lock()
			sourceBorrows := len(source.joins)
			source.registrationMu.Unlock()
			responder.registrationMu.Lock()
			responderBorrows := len(responder.joins)
			responder.registrationMu.Unlock()
			if got.a != nil {
				_ = got.a.Close()
			}
			if got.err == nil || got.a != nil || sourceBorrows != 0 || responderBorrows != 0 {
				t.Fatalf("canceled caller handed off after %s lock wait: acquisition=%p error=%v borrows=%d/%d", role, got.a, got.err, sourceBorrows, responderBorrows)
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
