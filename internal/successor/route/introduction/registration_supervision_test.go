package introduction

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

// A held foreground observation is scheduling evidence only. This fixture
// supplies no Registration, Receipt or authenticated Network authority.
func TestRegistrationSupervisionRechecksIntervalAfterWaitingForForeground(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		r := &HolderRegistration{ctx: ctx, caller: ctx, cancel: cancel,
			watcherDone: make(chan struct{}), retiring: make(chan struct{})}
		var calls atomic.Int32
		r.check = func() error {
			if calls.Add(1) == 1 {
				time.Sleep(250 * time.Millisecond)
			}
			return nil
		}
		go r.watch()
		t.Cleanup(func() { cancel(); <-r.watcherDone })
		synctest.Wait()
		time.Sleep(50 * time.Millisecond)
		foreground := make(chan error, 1)
		go func() { foreground <- r.checkOriginal() }()
		synctest.Wait()
		time.Sleep(250 * time.Millisecond)
		synctest.Wait()
		if err := <-foreground; err != nil {
			t.Fatal(err)
		}
		if calls.Load() != 1 {
			t.Error("queued supervision repeated the held foreground observation", calls.Load())
		}
		time.Sleep(99 * time.Millisecond)
		synctest.Wait()
		if calls.Load() != 1 {
			t.Error("queued supervision ignored the completed-observation interval", calls.Load())
		}
		time.Sleep(time.Millisecond)
		synctest.Wait()
		if calls.Load() != 2 {
			t.Error("supervision did not perform a fresh observation at the original interval", calls.Load())
		}
		if err := r.checkOriginal(); err != nil || calls.Load() != 3 {
			t.Error("foreground check reused the background result", err, calls.Load())
		}
	})
}

// This isolates the real watcher's scheduling and refusal propagation. The
// observation callback grants no Registration, Receipt or Network authority.
func TestRegistrationQueuedSupervisionDoesNotDeferAfterForegroundFailure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		r := &HolderRegistration{ctx: ctx, caller: ctx, cancel: cancel,
			watcherDone: make(chan struct{}), retiring: make(chan struct{})}
		var calls atomic.Int32
		failure := errors.New("held foreground observation refused")
		r.check = func() error {
			if calls.Add(1) == 1 {
				time.Sleep(250 * time.Millisecond)
			}
			return failure
		}
		go r.watch()
		t.Cleanup(func() { cancel(); <-r.watcherDone })
		synctest.Wait()
		time.Sleep(50 * time.Millisecond)
		foreground := make(chan error, 1)
		go func() { foreground <- r.checkOriginal() }()
		synctest.Wait()
		time.Sleep(250 * time.Millisecond)
		synctest.Wait()
		if err := <-foreground; !errors.Is(err, failure) {
			t.Fatal("foreground refusal was lost", err)
		}
		select {
		case <-r.watcherDone:
		default:
			t.Fatal("failed foreground observation deferred queued supervision")
		}
		if calls.Load() != 2 || !r.stopped || !errors.Is(r.failure, failure) {
			t.Fatal("queued supervision did not freshly observe and retain refusal")
		}
	})
}

func TestRegistrationSupervisionWaitsAfterSlowObservation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		r := &HolderRegistration{ctx: ctx, caller: ctx, cancel: cancel,
			watcherDone: make(chan struct{}), retiring: make(chan struct{})}
		var calls atomic.Int32
		failure := errors.New("original registration observer refused")
		r.check = func() error {
			if calls.Add(1) == 1 {
				time.Sleep(250 * time.Millisecond)
				return nil
			}
			return failure
		}
		go r.watch()
		t.Cleanup(func() { cancel(); <-r.watcherDone })
		synctest.Wait()
		time.Sleep(350 * time.Millisecond)
		synctest.Wait()
		if calls.Load() != 1 {
			t.Fatal("overdue supervision repeated immediately after slow observation")
		}
		time.Sleep(99 * time.Millisecond)
		synctest.Wait()
		if calls.Load() != 1 {
			t.Fatal("supervision did not preserve the completed-observation interval")
		}
		time.Sleep(time.Millisecond)
		synctest.Wait()
		select {
		case <-r.watcherDone:
		default:
			t.Fatal("fresh observation refusal did not retire the watcher")
		}
		if calls.Load() != 2 || !r.stopped || !errors.Is(r.failure, failure) || ctx.Err() == nil {
			t.Fatal("original refusal was not retained with physical interruption")
		}
	})
}

func TestRegistrationSupervisionUsesCompletedForegroundObservationOnly(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		r := &HolderRegistration{ctx: ctx, caller: ctx, cancel: cancel,
			watcherDone: make(chan struct{}), retiring: make(chan struct{})}
		var calls atomic.Int32
		failure := errors.New("original registration observer refused")
		r.check = func() error {
			if calls.Add(1) <= 2 {
				return nil
			}
			return failure
		}
		go r.watch()
		t.Cleanup(func() { cancel(); <-r.watcherDone })
		synctest.Wait()
		time.Sleep(50 * time.Millisecond)
		if err := r.checkOriginal(); err != nil {
			t.Fatal(err)
		}
		time.Sleep(25 * time.Millisecond)
		if err := r.checkOriginal(); err != nil || calls.Load() != 2 {
			t.Fatal("foreground check reused an earlier result", err)
		}
		time.Sleep(50 * time.Millisecond)
		if err := r.checkOriginal(); !errors.Is(err, failure) {
			t.Fatal("foreground refusal was lost", err)
		}
		time.Sleep(49 * time.Millisecond)
		synctest.Wait()
		if calls.Load() != 3 {
			t.Fatal("background observation repeated before completed-check interval")
		}
		time.Sleep(time.Millisecond)
		synctest.Wait()
		select {
		case <-r.watcherDone:
		default:
			t.Fatal("failed foreground observation postponed supervision")
		}
		if calls.Load() != 4 || !r.stopped || !errors.Is(r.failure, failure) {
			t.Fatal("background refusal did not retain original failure")
		}
	})
}
