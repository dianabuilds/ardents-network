package systemd

import (
	"context"
	"errors"
	"testing"
)

// These test only the local admission/Close ordering. No bus connection,
// installed Source, successful manager request or accepting authority is faked.
func TestSourceControlLocalAttemptCannotFollowOriginalDenialOrClose(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	r := &managerReference{original: ctx}
	s := &sourceControl{fixedUnitControl: &fixedUnitControl{reference: r, purpose: sourceControlPurpose}}
	cancel()
	if err := s.admitStart(); err != context.Canceled || s.startAttempted {
		t.Fatal("cancelled original admitted native Start attempt", err)
	}
	r = &managerReference{original: t.Context()}
	s = &sourceControl{fixedUnitControl: &fixedUnitControl{reference: r, purpose: sourceControlPurpose}}
	r.closed.Store(true)
	if err := s.admitStart(); !errors.Is(err, ErrUnavailable) || s.startAttempted {
		t.Fatal("closed original admitted native Start attempt", err)
	}
}

func TestSourceControlCleanupAttemptNeverRestoresAdmission(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	r := &managerReference{original: ctx}
	s := &sourceControl{fixedUnitControl: &fixedUnitControl{reference: r, purpose: sourceControlPurpose}}
	if err := s.admitStart(); err != nil {
		t.Fatal(err)
	}
	if err := s.admitStart(); err == nil {
		t.Fatal("second Start attempt admitted")
	}
	cancel()
	if err := s.admitStop(context.WithoutCancel(ctx)); err != nil {
		t.Fatal("original cleanup denied", err)
	}
	if err := s.admitStop(t.Context()); err == nil {
		t.Fatal("second Stop attempt admitted")
	}
	if err := s.admitStart(); err != context.Canceled {
		t.Fatal("cleanup restored original admission", err)
	}
	if err := r.local(); err != context.Canceled {
		t.Fatal("cleanup replaced original caller", err)
	}
}
