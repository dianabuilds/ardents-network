package connection

import (
	"context"
	"errors"
	"sync"
	"testing"
)

// These controls exercise orchestration only. They supply no Route stream,
// token, accepting observation, authentication or Connection handoff.
func TestInitiatorSetupSubmissionAndJoinOverlap(t *testing.T) {
	submissionStarted := make(chan struct{})
	joinStarted := make(chan struct{})
	err := runInitiatorSetupFlights(t.Context(), func() error {
		t.Error("successful setup interrupted its physical acquisition")
		return nil
	}, func(context.Context) error {
		close(submissionStarted)
		<-joinStarted
		return nil
	}, func() error {
		close(joinStarted)
		<-submissionStarted
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestInitiatorSetupCancellationJoinsBothFlights(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	started := make(chan struct{})
	interrupted := make(chan struct{})
	submissionRetired := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- runInitiatorSetupFlights(ctx, func() error {
			close(interrupted)
			return nil
		}, func(ctx context.Context) error {
			close(started)
			<-ctx.Done()
			close(submissionRetired)
			return ctx.Err()
		}, func() error {
			<-interrupted
			return context.Canceled
		})
	}()
	<-started
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation lost", err)
	}
	select {
	case <-submissionRetired:
	default:
		t.Fatal("canceled setup returned before submission joined")
	}
}

func TestInitiatorSetupSubmissionFailureInterruptsAndRetainsJoin(t *testing.T) {
	submissionFailure := errors.New("matching submission refused")
	joinFailure := errors.New("original JOIN interrupted")
	cleanupFailure := errors.New("original acquisition cleanup failed")
	interrupted := make(chan struct{})
	err := runInitiatorSetupFlights(t.Context(), func() error {
		close(interrupted)
		return cleanupFailure
	}, func(context.Context) error {
		return submissionFailure
	}, func() error {
		<-interrupted
		return joinFailure
	})
	for _, cause := range []error{submissionFailure, joinFailure, cleanupFailure} {
		if !errors.Is(err, cause) {
			t.Fatal("lost original failure", cause, err)
		}
	}
}

func TestInitiatorSetupJoinFailureWaitsForSubmissionRetirement(t *testing.T) {
	joinFailure := errors.New("JOIN refused")
	submissionFailure := errors.New("late submission cleanup failed")
	retiring := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseSubmission := func() { releaseOnce.Do(func() { close(release) }) }
	defer releaseSubmission()
	done := make(chan error, 1)
	go func() {
		done <- runInitiatorSetupFlights(t.Context(), func() error { return nil }, func(ctx context.Context) error {
			<-ctx.Done()
			close(retiring)
			<-release
			return submissionFailure
		}, func() error { return joinFailure })
	}()
	<-retiring
	select {
	case err := <-done:
		t.Fatal("returned before submission joined", err)
	default:
	}
	releaseSubmission()
	err := <-done
	if !errors.Is(err, joinFailure) || !errors.Is(err, submissionFailure) {
		t.Fatal("lost joined original failures", err)
	}
}
