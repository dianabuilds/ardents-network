package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
)

func TestSourceRetirementDoesNotHideJoinedPhysicalFailure(t *testing.T) {
	physical := errors.New("late native close failure")
	for _, test := range []struct {
		result error
		clean  bool
	}{
		{nil, true},
		{context.Canceled, true},
		{&sourceCommandFailure{fmt.Errorf("original interruption: %w", errors.Join(context.Canceled, context.Canceled))}, true},
		{errors.Join(context.Canceled, physical), false},
		{&sourceCommandFailure{errors.Join(context.Canceled, fmt.Errorf("joined physical: %w", physical))}, false},
		{errors.Join(context.Canceled, context.DeadlineExceeded), false},
		{errors.Join(context.Canceled, errors.New("source completion unavailable")), false},
	} {
		if sourceCancellationOnly(test.result) != test.clean {
			t.Fatalf("retirement classification erased original failure: %v", test.result)
		}
	}
}

type sourceRetirementControl struct {
	closeBegun chan struct{}
	closeJoin  chan struct{}
	physical   error
	waitNil    bool
}

func (*sourceRetirementControl) Current() (state.Snapshot, error) { return state.Snapshot{}, nil }
func (s *sourceRetirementControl) Wait(ctx context.Context) error {
	<-ctx.Done()
	if s.waitNil { // Match genuine State.Wait's ordinary cancellation contract.
		return nil
	}
	return ctx.Err()
}
func (s *sourceRetirementControl) Close() error {
	close(s.closeBegun)
	<-s.closeJoin
	return s.physical
}

func TestInstalledSourceSupervisorInterruptsBeforeHeldObservationJoins(t *testing.T) {
	// Failure-only lifecycle control: it exercises no installed Startup or
	// successful State/Source authority. Native integrated acceptance is separate.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered, release := make(chan struct{}), make(chan struct{})
	late := errors.New("held original observation failed")
	physical := errors.New("original State physical close failed")
	owner := &sourceRetirementControl{closeBegun: make(chan struct{}), closeJoin: make(chan struct{}), physical: physical}
	denied := make(chan struct{})
	var once sync.Once
	deny := func() { once.Do(func() { close(denied) }) }
	done := make(chan error, 1)
	go func() {
		done <- superviseInstalledSource(ctx, owner, func() error { close(entered); <-release; return late }, deny, io.Discard)
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("original check did not enter")
	}
	cancel()
	for _, interrupted := range []chan struct{}{denied, owner.closeBegun} {
		select {
		case <-interrupted:
		case <-time.After(time.Second):
			t.Fatal("cancellation waited behind original observation")
		}
	}
	select {
	case err := <-done:
		t.Fatalf("supervisor released original borrowers early: %v", err)
	default:
	}
	close(release)
	select {
	case err := <-done:
		t.Fatalf("supervisor treated observation completion as State join: %v", err)
	default:
	}
	close(owner.closeJoin)
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) || !errors.Is(err, late) || !errors.Is(err, physical) {
			t.Fatalf("original cancellation, late failure or physical cause lost: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("supervisor did not finish after original joins")
	}
}

func TestInstalledSourceSupervisorAcceptsStateNilWaitAfterCancellation(t *testing.T) {
	// Lifecycle-only control; no installed Startup, State authority or Source
	// factory is opened. The supervisor must also join State's nil-return path.
	for range 64 {
		ctx, cancel := context.WithCancel(t.Context())
		owner := &sourceRetirementControl{closeBegun: make(chan struct{}), closeJoin: make(chan struct{}), waitNil: true}
		close(owner.closeJoin)
		cancel()
		result := superviseInstalledSource(ctx, owner, func() error { return ctx.Err() }, func() {}, io.Discard)
		if !errors.Is(result, context.Canceled) || !sourceCancellationOnly(result) {
			t.Fatalf("ordinary nil Wait invented a background failure after joined cancellation: %v", result)
		}
	}
}

func TestSourceWaitCompletionDistinguishesLiveEndCancellationAndLateFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	live := sourceWaitOutcome(ctx, nil)
	if live == nil || errors.Is(live, context.Canceled) {
		t.Fatal("an unexpected live background end became ordinary cancellation", live)
	}
	cancel()
	completed := sourceWaitOutcome(ctx, nil) // Genuine State.Wait returns nil here.
	if !errors.Is(completed, context.Canceled) || !sourceCancellationOnly(completed) {
		t.Fatal("nil Wait completion lost the original cancellation", completed)
	}
	physical := errors.New("late original State failure")
	failed := sourceWaitOutcome(ctx, physical)
	if !errors.Is(failed, context.Canceled) || !errors.Is(failed, physical) || sourceCancellationOnly(failed) {
		t.Fatal("ordinary cancellation hid the retained late failure", failed)
	}
}
