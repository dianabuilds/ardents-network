package route

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/entry"
)

func TestAttachmentReturnsImmutableEvidenceAndCleansOnce(t *testing.T) {
	left, right := net.Pipe()
	defer right.Close()
	var closed atomic.Int32
	attachment := &Attachment{connection: left, evidence: Evidence{AuthenticatedTarget: [32]byte{1}, AuthorityPublic: [32]byte{2},
		Publication: []byte("publication"), Generation: 3, AttachmentID: [32]byte{4}}, close: func() error {
		closed.Add(1)
		return left.Close()
	}}
	evidence, err := attachment.Evidence()
	if err != nil {
		t.Fatal(err)
	}
	evidence.Publication[0] = 'x'
	again, err := attachment.Evidence()
	if err != nil || string(again.Publication) != "publication" {
		t.Fatalf("attachment evidence = %+v, %v", again, err)
	}
	if err := attachment.Close(); err != nil {
		t.Fatal(err)
	}
	if err := attachment.Close(); err != nil {
		t.Fatal(err)
	}
	if closed.Load() != 1 {
		t.Fatalf("attachment cleanup calls = %d", closed.Load())
	}
}

func TestRouteAdmitsBeforeReadingState(t *testing.T) {
	stateRead := false
	denied := errors.New("route capacity refused")
	owner := openRouteForTest(t, func() (StateView, error) {
		stateRead = true
		return nil, nil
	}, func(context.Context) (func() error, error) { return nil, denied })
	defer owner.Close()
	if _, err := owner.Attach(context.Background(), Intent{Target: [32]byte{1}}); !errors.Is(err, denied) {
		t.Fatalf("admission refusal = %v", err)
	}
	if stateRead {
		t.Fatal("Route read State after capacity refusal")
	}
}

func TestRouteReleasesAdmissionWhenStateIsUnavailable(t *testing.T) {
	var released atomic.Int32
	owner := openRouteForTest(t, func() (StateView, error) {
		return nil, errors.New("State is unavailable")
	}, func(context.Context) (func() error, error) {
		return func() error {
			released.Add(1)
			return nil
		}, nil
	})
	defer owner.Close()
	if _, err := owner.Attach(context.Background(), Intent{Target: [32]byte{1}}); err == nil {
		t.Fatal("Route accepted an unavailable State view")
	}
	if released.Load() != 1 {
		t.Fatalf("Route admission releases = %d, want 1", released.Load())
	}
}

func TestRouteCloseCancelsAndJoinsPendingAdmission(t *testing.T) {
	entered := make(chan struct{})
	owner := openRouteForTest(t, func() (StateView, error) {
		return nil, errors.New("State must not run while admission is pending")
	}, func(ctx context.Context) (func() error, error) {
		close(entered)
		<-ctx.Done()
		return nil, ctx.Err()
	})
	result := make(chan error, 1)
	go func() {
		_, err := owner.Attach(context.Background(), Intent{Target: [32]byte{1}})
		result <- err
	}()
	<-entered
	if err := owner.Close(); err != nil {
		t.Fatalf("close Route: %v", err)
	}
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("pending Route attachment error = %v, want context cancellation", err)
	}
}

func TestRouteReportsAdmissionCleanupFailure(t *testing.T) {
	cleanupFailure := errors.New("route admission cleanup failed")
	owner := openRouteForTest(t, func() (StateView, error) {
		return nil, errors.New("state is unavailable")
	}, func(context.Context) (func() error, error) {
		return func() error { return cleanupFailure }, nil
	})
	defer owner.Close()
	if _, err := owner.Attach(context.Background(), Intent{Target: [32]byte{1}}); !errors.Is(err, cleanupFailure) {
		t.Fatalf("route failure = %v, want admission cleanup failure", err)
	}
}

func TestAttachmentCloseJoinsConcurrentCleanupFailure(t *testing.T) {
	left, right := net.Pipe()
	defer right.Close()
	cleanupFailure := errors.New("attachment cleanup failed")
	entered, unblock := make(chan struct{}), make(chan struct{})
	var closed atomic.Int32
	attachment := &Attachment{connection: left, evidence: Evidence{AuthenticatedTarget: [32]byte{1}, AttachmentID: [32]byte{2}}, close: func() error {
		closed.Add(1)
		close(entered)
		<-unblock
		_ = left.Close()
		return cleanupFailure
	}}
	result := make(chan error, 2)
	go func() { result <- attachment.Close() }()
	<-entered
	go func() { result <- attachment.Close() }()
	close(unblock)
	for range 2 {
		if err := <-result; !errors.Is(err, cleanupFailure) {
			t.Fatalf("concurrent attachment cleanup error = %v, want cleanup failure", err)
		}
	}
	if closed.Load() != 1 {
		t.Fatalf("concurrent attachment cleanup calls = %d, want 1", closed.Load())
	}
}

func TestRouteCloseRejectsNewAttachment(t *testing.T) {
	owner := openRouteForTest(t, func() (StateView, error) { return nil, nil }, func(context.Context) (func() error, error) {
		return func() error { return nil }, nil
	})
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Attach(context.Background(), Intent{Target: [32]byte{1}}); err == nil {
		t.Fatal("closed Route accepted an attachment")
	}
}

func openRouteForTest(t *testing.T, current func() (StateView, error), admit ResourceAdmission) *Route {
	t.Helper()
	owner, err := Open(Config{NetworkID: [32]byte{1}, Current: current, Entry: routeTestEntry{},
		Credentials: func(context.Context, CredentialRequest) (Credential, error) {
			return Credential{}, errors.New("credential must not run")
		},
		Admit: admit, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	return owner
}

type routeTestEntry struct{}

func (routeTestEntry) Contact() (entry.Candidate, error) {
	return entry.Candidate{}, errors.New("Entry must not run")
}

func (routeTestEntry) Acquire(context.Context, entry.Attempt, entry.CandidateOpener) (net.Conn, func() error, error) {
	return nil, nil, errors.New("Entry must not run")
}
