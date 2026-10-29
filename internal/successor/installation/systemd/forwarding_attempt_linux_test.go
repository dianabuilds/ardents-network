package systemd

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

// These test only the local admission/Close ordering. No bus connection,
// installed Forwarding, successful manager request or accepting authority is faked.
func TestForwardingControlLocalAttemptCannotFollowOriginalDenialOrClose(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	r := &managerReference{original: ctx}
	s := &forwardingControl{reference: r}
	cancel()
	if err := s.admitForwardingStart(); err != context.Canceled || s.startAttempted {
		t.Fatal("cancelled original admitted native Start attempt", err)
	}
	r = &managerReference{original: t.Context()}
	s = &forwardingControl{reference: r}
	r.closed.Store(true)
	if err := s.admitForwardingStart(); !errors.Is(err, ErrUnavailable) || s.startAttempted {
		t.Fatal("closed original admitted native Start attempt", err)
	}
}

func TestForwardingReplacementCopiesShareOneReloadAttemptAndDenial(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	r := &managerReference{original: ctx}
	s := ForwardingReplacement{original: &forwardingReplacement{reference: r}}
	copy := s
	var admitted atomic.Int32
	var uses sync.WaitGroup
	for n := 0; n < 32; n++ {
		uses.Add(1)
		go func() {
			defer uses.Done()
			if err := copy.original.reference.admitForwardingReload(&copy.original.reloaded); err == nil {
				admitted.Add(1)
			}
		}()
	}
	uses.Wait()
	if admitted.Load() != 1 || !s.original.reloaded {
		t.Fatal("copies split the one native attempt", admitted.Load())
	}
	cancel()
	if err := r.admitForwardingReload(&s.original.reloaded); err != context.Canceled {
		t.Fatal("lost original denial", err)
	}
	for _, state := range []string{"closed", "failed", "cancelled"} {
		t.Run(state, func(t *testing.T) {
			original, deny := context.WithCancel(t.Context())
			defer deny()
			r := &managerReference{original: original}
			failure := errors.New("original native failure")
			switch state {
			case "closed":
				r.closed.Store(true)
			case "failed":
				r.terminal = failure
			case "cancelled":
				deny()
			}
			attempted := false
			err := r.admitForwardingReload(&attempted)
			if err == nil || attempted {
				t.Fatal("denied owner admitted native attempt", err)
			}
			if state == "failed" && !errors.Is(err, failure) {
				t.Fatal("native failure replaced", err)
			}
		})
	}
}

func TestForwardingControlCleanupAttemptNeverRestoresAdmission(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	r := &managerReference{original: ctx}
	s := &forwardingControl{reference: r}
	if err := s.admitForwardingStart(); err != nil {
		t.Fatal(err)
	}
	if err := s.admitForwardingStart(); err == nil {
		t.Fatal("second Start attempt admitted")
	}
	cancel()
	if err := s.admitForwardingStop(context.WithoutCancel(ctx)); err != nil {
		t.Fatal("original cleanup denied", err)
	}
	if err := s.admitForwardingStop(t.Context()); err == nil {
		t.Fatal("second Stop attempt admitted")
	}
	if err := s.admitForwardingStart(); err != context.Canceled {
		t.Fatal("cleanup restored original admission", err)
	}
	if err := r.local(); err != context.Canceled {
		t.Fatal("cleanup replaced original caller", err)
	}
}
