package introduction

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

// This isolates the real watcher's scheduling and refusal propagation. The
// observation callback grants no Registration, Receipt or Network authority.
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
