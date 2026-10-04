//go:build linux

package main

import (
	"context"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"testing"
	"time"
)

func TestLocalTransfer(t *testing.T) {
	if err := transferLoopback(t.Context(), 64<<10, time.Now().Add(time.Second), func() error { return nil }); err != nil {
		t.Fatal(err)
	}
}

func TestLocalTransferWithdrawnAfterSocketsOpened(t *testing.T) {
	withdrawn := errors.New("authority withdrawn")
	first := true
	err := transferLoopback(t.Context(), 64<<10, time.Now().Add(time.Second), func() error {
		if first {
			first = false
			return nil
		}
		return withdrawn
	})
	if !errors.Is(err, withdrawn) {
		t.Fatal("lost withdrawal", err)
	}
}

func TestLocalTransferCanceledDuringWork(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	first := true
	err := transferLoopback(ctx, 64<<10, time.Now().Add(time.Second), func() error {
		if first {
			first = false
			return nil
		}
		cancel()
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatal("lost cancellation", err)
	}
}

func TestWorkCannotExtendAcceptedAuthority(t *testing.T) {
	start := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	facts := admission.AuthorityFacts{NotBefore: start, NotAfter: start.Add(time.Second)}
	now := start
	current := workAuthority(facts, start, start.Add(5*time.Second), func() (admission.AuthorityFacts, time.Time, error) { return facts, now, nil })
	if err := current(); err != nil {
		t.Fatal(err)
	}
	now = facts.NotAfter
	if err := current(); err == nil {
		t.Fatal("same expired facts extended authority")
	}
}

func TestWorkRefusesObservationRollback(t *testing.T) {
	start := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	facts := admission.AuthorityFacts{NotBefore: start, NotAfter: start.Add(time.Hour)}
	now := start.Add(time.Second)
	current := workAuthority(facts, start, start.Add(time.Minute), func() (admission.AuthorityFacts, time.Time, error) { return facts, now, nil })
	if err := current(); err != nil {
		t.Fatal(err)
	}
	now = start
	if err := current(); err == nil {
		t.Fatal("rollback renewed work")
	}
}

func TestObservationFloorSpansAdmissionAndWork(t *testing.T) {
	now := time.Now()
	source := monotonicObserver(func() (admission.AuthorityFacts, time.Time, error) { return admission.AuthorityFacts{}, now, nil })
	_, initial, err := source()
	if err != nil {
		t.Fatal(err)
	}
	now = initial.Add(time.Second)
	if _, _, err := source(); err != nil {
		t.Fatal(err)
	}
	now = initial.Add(time.Millisecond)
	if _, _, err := source(); err == nil {
		t.Fatal("rollback across admission/work boundary accepted")
	}
}
