package prefix

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
)

// This failure-only physical setup supplies no authority, admission or JOIN
// RESULT. The real AfterFunc is stopped before cancellation; its callback must
// never run, and later physical cleanup must not wait for that absent callback.
func TestPrefixJoinStoppedSetupCallbackCannotBlockPhysicalRetirement(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		done := make(chan struct{})
		// Unblock a broken implementation only after the completion assertion,
		// so a failed test still joins its own goroutine before bubble exit.
		defer close(done)
		var callbacks, returns atomic.Int32
		cause := errors.New("original partial setup physical close failed")
		physical := newLifecycleConn(false)
		physical.closeFailure = cause
		terminal := &JoinChannel{ctx: ctx, conn: physical, interruption: prefixSetupInterruption{done: done},
			release: func() { returns.Add(1) }}
		terminal.interruption.stop = context.AfterFunc(ctx, func() { callbacks.Add(1) })
		terminal.JoinSetupInterruption()
		retired := make(chan error, 1)
		go func() { retired <- terminal.CloseSetup() }()
		synctest.Wait()
		select {
		case err := <-retired:
			if !errors.Is(err, cause) || returns.Load() != 1 || callbacks.Load() != 0 {
				t.Fatal("cleanup lost its physical result or changed callback ownership", err, returns.Load(), callbacks.Load())
			}
		default:
			t.Fatal("physical retirement waited for an already stopped callback")
		}
	})
}

func TestPrefixJoinRunningSetupCallbackJoinsBeforePhysicalReturn(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		started, gate, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
		var unblockOnce sync.Once
		unblock := func() { unblockOnce.Do(func() { close(gate) }) }
		defer unblock()
		var returns atomic.Int32
		cause := errors.New("physical setup retirement after callback failed")
		physical := newLifecycleConn(false)
		physical.closeFailure = cause
		terminal := &JoinChannel{ctx: ctx, conn: physical, interruption: prefixSetupInterruption{done: done},
			release: func() { returns.Add(1) }}
		terminal.interruption.stop = context.AfterFunc(ctx, func() { close(started); <-gate; close(done) })
		cancel()
		<-started
		retired := make(chan error, 1)
		go func() { retired <- terminal.CloseSetup() }()
		synctest.Wait()
		if returns.Load() != 0 {
			t.Fatal("setup returned capacity before its original callback joined")
		}
		select {
		case err := <-retired:
			t.Fatal("physical cleanup completed before callback", err)
		case <-physical.closed:
			t.Fatal("physical cleanup started before callback joined")
		default:
		}
		unblock()
		if err := <-retired; !errors.Is(err, cause) || returns.Load() != 1 {
			t.Fatal("joined callback lost physical failure or return", err, returns.Load())
		}
		if !terminal.JoinSetupInterruption() {
			t.Fatal("repeated join forgot that the original callback ran")
		}
	})
}
