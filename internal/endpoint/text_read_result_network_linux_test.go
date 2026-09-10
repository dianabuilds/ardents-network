//go:build linux

package endpoint

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

// Only launch qualification is supplied by the existing explicit worker
// fixture. This uses real Service authentication, worker RESULT, and local
// document projection. The installed profile exercises Open's actual launcher.
func readTextWorkerResultFixture(t *testing.T, ctx context.Context, worker *qualifiedTextWorker, destination targetlink.Link, bounds [3]int64) ([]byte, error) {
	stream, err := openTextWorkerResultFixture(t, ctx, worker, destination, bounds)
	if err != nil {
		return nil, err
	}
	return textdocument.Read(ctx, stream)
}

func openTextWorkerResultFixture(t *testing.T, ctx context.Context, worker *qualifiedTextWorker, destination targetlink.Link, bounds [3]int64) (_ *textReadResult, outcome error) {
	t.Helper()
	contextOwner := worker.job.owner
	owner, err := contextOwner.openTextConnection()
	if err != nil {
		return nil, err
	}
	t.Cleanup(func() {
		if err := owner.Close(); err != nil {
			t.Error(err)
		}
	})
	lifetime, cancel := context.WithCancel(contextOwner.lease.Context())
	callerDone := make(chan struct{})
	stopCaller := context.AfterFunc(ctx, func() { defer close(callerDone); cancel() })
	joinCaller := func() {
		if !stopCaller() {
			<-callerDone
		}
	}
	capability, err := contextOwner.endpoint.Admit(contextOwner.principal, broker.Connection)
	if err != nil {
		joinCaller()
		cancel()
		return nil, err
	}
	lease, _, err := contextOwner.endpoint.admission.Activate(lifetime, capability, contextOwner.principal, broker.Connection)
	if err != nil {
		joinCaller()
		cancel()
		return nil, err
	}
	transferred := false
	defer func() {
		if !transferred {
			joinCaller()
			lease.Release()
			cancel()
			outcome = errors.Join(outcome, worker.Close())
		}
	}()
	bounded, finish, err := worker.beginOperation(lease.Context(), broker.Connection)
	if err != nil {
		return nil, err
	}
	defer func() {
		if !transferred {
			finish()
		}
	}()
	attempt, err := contextOwner.prepareTextIntroduction(bounded, worker.job, destination, bounds)
	if err != nil {
		return nil, err
	}
	service, err := contextOwner.openTextJoinedService(bounded, worker.job, attempt)
	if err != nil {
		return nil, err
	}
	owner.mu.Lock()
	pending := make(chan struct{})
	owner.pending, owner.cancel = pending, cancel
	owner.mu.Unlock()
	stream := newTextReadResult(owner, pending, lease, cancel, worker, bounded, finish, service, joinCaller)
	transferred = true
	return stream, nil
}

func TestTextReadResultJoinsContextLossBeforeLocalRequest(t *testing.T) {
	readerOwner, publisherOwner := textUnpublishedNetworkFixture(t, route.ClosedCarrierTCP)
	reader := textServiceWorkerFixture(t, &textServiceBinding{owner: readerOwner, job: liveTextCapsuleJob(t, readerOwner)}, nil)
	publisher := textServiceWorkerFixture(t, &textServiceBinding{owner: publisherOwner, job: liveTextCapsuleJob(t, publisherOwner)}, []byte("document"))
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	run, err := publisher.startPublication(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// This test deliberately aborts an authenticated Service before its
		// request. The Publisher's abort may preserve that cancellation; it
		// must still join, and a different failure is not an accepted result.
		if err := run.Close(); err != nil {
			if !textReadCancellationOnly(err) {
				t.Error(err)
			} else {
				t.Logf("joined Publisher abort: %v", err)
			}
		}
	})
	until := time.Now().Add(time.Minute).Unix()
	stream, err := openTextWorkerResultFixture(t, ctx, reader, run.link, [3]int64{until, until, until})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := stream.Close(); !textReadCancellationOnly(err) {
			t.Errorf("unexpected read retirement: %v", err)
		}
	})
	if err := readerOwner.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-stream.joined:
	case <-ctx.Done():
		t.Fatal("context loss did not join blocked local request")
	}
	if err := stream.Close(); !textReadCancellationOnly(err) {
		t.Fatalf("cancelled local request returned unexpected outcome: %v", err)
	}
}

// Check every joined cause, not merely the presence of context.Canceled.
// These exact first-child labels are the current owners' diagnostic wrappers;
// they are accepted only when every underlying cause is cancellation.
func textReadCancellationOnly(err error) bool {
	if err == context.Canceled {
		return true
	}
	if err == nil {
		return false
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		causes := joined.Unwrap()
		if len(causes) == 0 {
			return false
		}
		if len(causes) > 1 {
			switch causes[0].Error() {
			case "text Service cleanup failed", "text Service transport retirement failed", "text request is invalid":
				causes = causes[1:]
			}
		}
		for _, cause := range causes {
			if !textReadCancellationOnly(cause) {
				return false
			}
		}
		return true
	}
	if wrapped := errors.Unwrap(err); wrapped != nil {
		return textReadCancellationOnly(wrapped)
	}
	return false
}

func TestTextReadCancellationRejectsAdditionalCleanupFailure(t *testing.T) {
	fault := errors.New("unexpected cleanup failure")
	for _, err := range []error{nil, fault, errors.Join(context.Canceled, fault), errors.Join(errors.New("text Service cleanup failed"), context.Canceled, fault), errors.New("text Service cleanup failed")} {
		if textReadCancellationOnly(err) {
			t.Fatalf("accepted non-cancellation cause: %v", err)
		}
	}
	if !textReadCancellationOnly(errors.Join(errors.New("text Service cleanup failed"), errors.Join(context.Canceled, context.Canceled))) {
		t.Fatal("exact cancellation wrapper refused")
	}
}
