package prefix

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	framing "github.com/dianabuilds/ardents-network/internal/successor/route/channel"
)

// These actual pipe readers isolate supervision scheduling; the callbacks
// supply no authenticated Network view or successful Route admission.
func TestPrefixSupervisionFollowsFreshEffectsWithoutCachingThem(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		a, aPeer := net.Pipe()
		b, bPeer := net.Pipe()
		end := time.Now().Add(time.Minute)
		queues := framing.NewBudget(4 << 20)
		observed := &prefixObservation{}
		p := &Prefix{ctx: ctx, cancel: cancel, done: make(chan struct{}), closing: make(chan struct{}), observation: observed}
		p.entry = framing.New(ctx, a, end, 1<<20, nil, false, queues, nil)
		p.interior = framing.New(ctx, b, end, 1<<20, nil, false, queues, nil)
		t.Cleanup(func() {
			cancel()
			_ = aPeer.Close()
			_ = bPeer.Close()
			_ = p.entry.Close()
			_ = p.interior.Close()
			<-p.done
		})
		var calls atomic.Int32
		var refused atomic.Bool
		failure := errors.New("original observer refused")
		check := func() error {
			return observed.check(func() error {
				calls.Add(1)
				if refused.Load() {
					return failure
				}
				return nil
			})
		}
		go p.watch(check, make(chan time.Time))
		synctest.Wait()
		time.Sleep(80 * time.Millisecond)
		// Both effect callers must perform their observations, even back to back.
		first, second := check(), check()
		if first != nil || second != nil || calls.Load() != 2 {
			t.Fatal("effect checks reused a prior result")
		}
		time.Sleep(21 * time.Millisecond)
		synctest.Wait()
		if calls.Load() != 2 {
			t.Fatal("watch repeated a just-completed effect observation")
		}
		refused.Store(true)
		// A failed effect must not postpone the background refusal observation.
		time.Sleep(49 * time.Millisecond)
		if !errors.Is(check(), failure) {
			t.Fatal("fresh refusal was hidden")
		}
		time.Sleep(30 * time.Millisecond)
		synctest.Wait()
		if calls.Load() != 4 {
			t.Fatal("supervision interval was extended after failed observation", calls.Load())
		}
		select {
		case <-p.done:
		default:
			t.Fatal("original observation failure did not retire supervision")
		}
		if !errors.Is(p.entry.Close(), failure) {
			t.Fatal("original observer failure was lost")
		}
	})
}
