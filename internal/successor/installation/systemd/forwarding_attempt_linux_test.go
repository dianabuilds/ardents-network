package systemd

import (
	"context"
	"errors"
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
