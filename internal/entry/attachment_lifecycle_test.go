package entry

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

func TestCloseCleansActiveAttachmentBeforeReleasingRoot(t *testing.T) {
	fixture := newLiveEntryFixture(t)
	root := entryRoot(t)
	owner, err := Open(fixture.config(root))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Import(fixture.invite(t, fixture.candidates[0], 0, 1, nil)); err != nil {
		t.Fatal(err)
	}
	var cleanupCalls atomic.Int32
	var releasedEarly atomic.Bool
	connection, cleanup, err := owner.Acquire(context.Background(), Attempt{ID: [32]byte{71}, Deadline: fixture.now.Add(5 * time.Second)},
		func(context.Context, Candidate, Presentation, time.Time) (net.Conn, func() error, bool, error) {
			client, server := net.Pipe()
			return client, func() error {
				cleanupCalls.Add(1)
				unexpected, openErr := Open(fixture.config(root))
				if openErr == nil {
					releasedEarly.Store(true)
					_ = unexpected.Close()
				}
				return errors.Join(client.Close(), server.Close())
			}, true, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if connection == nil {
		t.Fatal("Acquire returned no active connection")
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if releasedEarly.Load() {
		t.Fatal("Entry released its root before active attachment cleanup")
	}
	if cleanupCalls.Load() != 1 {
		t.Fatalf("cleanup calls = %d, want 1", cleanupCalls.Load())
	}
	if owner.state.Attempt == nil || owner.state.Attempt.Terminal != "opened" || len(owner.state.Contacts) != 1 || !owner.state.Contacts[0].Cleanup {
		t.Fatalf("terminal attachment = %+v contacts = %+v", owner.state.Attempt, owner.state.Contacts)
	}
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if cleanupCalls.Load() != 1 {
		t.Fatalf("idempotent cleanup calls = %d, want 1", cleanupCalls.Load())
	}
	reopened, err := Open(fixture.config(root))
	if err != nil {
		t.Fatalf("root lease leaked after Close: %v", err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCloseSerializesConcurrentAcquireAndAttachmentCleanup(t *testing.T) {
	fixture := newLiveEntryFixture(t)
	root := entryRoot(t)
	owner, err := Open(fixture.config(root))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Import(fixture.invite(t, fixture.candidates[0], 0, 1, nil)); err != nil {
		t.Fatal(err)
	}
	cleanupEntered := make(chan struct{})
	releaseCleanup := make(chan struct{})
	var cleanupCalls atomic.Int32
	_, cleanup, err := owner.Acquire(context.Background(), Attempt{ID: [32]byte{72}, Deadline: fixture.now.Add(5 * time.Second)},
		func(context.Context, Candidate, Presentation, time.Time) (net.Conn, func() error, bool, error) {
			client, server := net.Pipe()
			return client, func() error {
				cleanupCalls.Add(1)
				close(cleanupEntered)
				<-releaseCleanup
				return errors.Join(client.Close(), server.Close())
			}, true, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	closeResult := make(chan error, 1)
	go func() { closeResult <- owner.Close() }()
	<-cleanupEntered

	if _, _, err := owner.Acquire(context.Background(), Attempt{ID: [32]byte{73}, Deadline: fixture.now.Add(5 * time.Second)},
		func(context.Context, Candidate, Presentation, time.Time) (net.Conn, func() error, bool, error) {
			t.Fatal("opener ran after Entry Close began")
			return nil, nil, false, nil
		}); err == nil {
		t.Fatal("Acquire admitted work while Entry was closing")
	}
	cleanupResult := make(chan error, 1)
	go func() { cleanupResult <- cleanup() }()
	close(releaseCleanup)
	if err := <-closeResult; err != nil {
		t.Fatal(err)
	}
	if err := <-cleanupResult; err != nil {
		t.Fatal(err)
	}
	if cleanupCalls.Load() != 1 {
		t.Fatalf("concurrent cleanup calls = %d, want 1", cleanupCalls.Load())
	}
	reopened, err := Open(fixture.config(root))
	if err != nil {
		t.Fatalf("root lease leaked after concurrent Close: %v", err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCloseCancelsInflightAcquisitionAndPersistsTerminalOutcome(t *testing.T) {
	fixture := newLiveEntryFixture(t)
	root := entryRoot(t)
	owner, err := Open(fixture.config(root))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Import(fixture.invite(t, fixture.candidates[0], 0, 1, nil)); err != nil {
		t.Fatal(err)
	}
	openerEntered := make(chan struct{})
	acquireResult := make(chan error, 1)
	var opens atomic.Int32
	go func() {
		_, _, acquireErr := owner.Acquire(context.Background(), Attempt{ID: [32]byte{74}, Deadline: fixture.now.Add(5 * time.Second)},
			func(ctx context.Context, _ Candidate, _ Presentation, _ time.Time) (net.Conn, func() error, bool, error) {
				if opens.Add(1) == 1 {
					close(openerEntered)
				}
				<-ctx.Done()
				return nil, nil, true, ctx.Err()
			})
		acquireResult <- acquireErr
	}()
	<-openerEntered
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-acquireResult; err == nil {
		t.Fatal("in-flight acquisition did not observe Entry cancellation")
	}
	if owner.state.Attempt == nil || owner.state.Attempt.Terminal != "entry-local-denial" {
		t.Fatalf("canceled attempt = %+v", owner.state.Attempt)
	}
	reopened, err := Open(fixture.config(root))
	if err != nil {
		t.Fatalf("root lease leaked after canceled acquisition: %v", err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCloseReturnsAttachmentCleanupFailureAfterPersistingIt(t *testing.T) {
	fixture := newLiveEntryFixture(t)
	root := entryRoot(t)
	owner, err := Open(fixture.config(root))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Import(fixture.invite(t, fixture.candidates[0], 0, 1, nil)); err != nil {
		t.Fatal(err)
	}
	injected := errors.New("injected attachment cleanup failure")
	var cleanupCalls atomic.Int32
	_, cleanup, err := owner.Acquire(context.Background(), Attempt{ID: [32]byte{75}, Deadline: fixture.now.Add(5 * time.Second)},
		func(context.Context, Candidate, Presentation, time.Time) (net.Conn, func() error, bool, error) {
			client, server := net.Pipe()
			return client, func() error {
				cleanupCalls.Add(1)
				return errors.Join(net.ErrClosed, injected, client.Close(), server.Close())
			}, true, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); !errors.Is(err, injected) {
		t.Fatalf("Close error = %v, want injected cleanup failure", err)
	}
	if owner.state.Attempt == nil || owner.state.Attempt.Terminal != "entry-local-denial" || len(owner.state.Contacts) != 1 || owner.state.Contacts[0].Cleanup {
		t.Fatalf("failed cleanup outcome = %+v contacts = %+v", owner.state.Attempt, owner.state.Contacts)
	}
	if err := cleanup(); !errors.Is(err, injected) {
		t.Fatalf("repeated cleanup error = %v", err)
	}
	if err := owner.Close(); !errors.Is(err, injected) {
		t.Fatalf("repeated Close error = %v", err)
	}
	if cleanupCalls.Load() != 1 || len(owner.attachments) != 0 {
		t.Fatalf("cleanup calls = %d active attachments = %d", cleanupCalls.Load(), len(owner.attachments))
	}
	reopened, err := Open(fixture.config(root))
	if err != nil {
		t.Fatalf("root lease leaked after failed cleanup: %v", err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
}
