package route

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/dianabuilds/ardents-network/internal/entry"
	"github.com/dianabuilds/ardents-network/internal/network/state"
)

func TestRouteAttachRefusesBeforeEntryAcquisition(t *testing.T) {
	called := false
	denied := errors.New("resource admission refused")
	route := openRoute(t, nativeView(time.Now().UTC(), 1), entryAcquirerFunc(func(context.Context, entry.Attempt, entry.CandidateOpener) (net.Conn, func() error, error) {
		called = true
		return nil, nil, errors.New("Entry acquisition must not run")
	}), func(context.Context) (func() error, error) { return nil, denied })
	defer route.Close()
	_, err := route.Attach(context.Background(), Intent{Deadline: time.Now().Add(time.Minute)})
	if !errors.Is(err, denied) {
		t.Fatalf("resource refusal = %v", err)
	}
	if called {
		t.Fatal("Entry acquisition ran after resource refusal")
	}
}

func TestRouteAttachmentCapacityReleasesOnlyAfterClose(t *testing.T) {
	now := time.Now().UTC()
	var mu sync.Mutex
	reserved, released := 0, 0
	route := openRoute(t, nativeView(now, 1), successfulEntry(), func(context.Context) (func() error, error) {
		mu.Lock()
		reserved++
		mu.Unlock()
		return func() error {
			mu.Lock()
			released++
			mu.Unlock()
			return nil
		}, nil
	})
	defer route.Close()
	first, err := route.Attach(context.Background(), Intent{Deadline: now.Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := route.Attach(context.Background(), Intent{Deadline: now.Add(time.Minute)}); err == nil {
		t.Fatal("overlapping attachment reused the only Route positions")
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := route.Attach(context.Background(), Intent{Deadline: now.Add(time.Minute)})
	if err != nil {
		t.Fatalf("attachment after prior close: %v", err)
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if reserved != 2 || released != 2 {
		t.Fatalf("reservations = %d, releases = %d", reserved, released)
	}
}

func TestRouteConcurrentAttachmentsUseDistinctCandidates(t *testing.T) {
	now := time.Now().UTC()
	route := openRoute(t, nativeView(now, 2), successfulEntry(), admitted())
	defer route.Close()
	first, err := route.Attach(context.Background(), Intent{Deadline: now.Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := route.Attach(context.Background(), Intent{Deadline: now.Add(time.Minute)})
	if err != nil {
		t.Fatalf("second attachment with alternate candidates: %v", err)
	}
	defer second.Close()
	if _, err := route.Attach(context.Background(), Intent{Deadline: now.Add(time.Minute)}); err == nil {
		t.Fatal("third attachment reused a candidate already reserved by a live attachment")
	}
}

func TestRouteAttachUsesCurrentNativeViewAndShortensSafetyDeadline(t *testing.T) {
	now := time.Now().UTC()
	view := nativeView(now, 2)
	view.ValidUntil = now.Add(20 * time.Second)
	for index := range view.Candidates[:view.CandidateCount] {
		view.Candidates[index].AssignmentNotAfter = now.Add(15 * time.Second)
	}
	var attempt entry.Attempt
	called := false
	route := openRoute(t, view, entryAcquirerFunc(func(_ context.Context, value entry.Attempt, _ entry.CandidateOpener) (net.Conn, func() error, error) {
		called, attempt = true, value
		connection, cleanup := relayReadyPipe()
		return connection, cleanup, nil
	}), admitted())
	defer route.Close()
	attachment, err := route.Attach(context.Background(), Intent{Deadline: now.Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	defer attachment.Close()
	if !called || !attempt.Deadline.Equal(now.Add(15*time.Second)) {
		t.Fatalf("Entry attempt = %+v, called = %t", attempt, called)
	}
}

func TestRouteRejectsRetiredProfileBeforeAdmission(t *testing.T) {
	now := time.Now().UTC()
	view := nativeView(now, 1)
	view.Profile = "h3-route-tracer-v1"
	called := false
	route := openRoute(t, view, successfulEntry(), func(context.Context) (func() error, error) {
		called = true
		return func() error { return nil }, nil
	})
	defer route.Close()
	if _, err := route.Attach(context.Background(), Intent{Deadline: now.Add(time.Minute)}); err == nil {
		t.Fatal("retired H3 profile opened a Route attachment")
	}
	if called {
		t.Fatal("retired profile reached resource admission")
	}
}

func TestRouteCloseCancelsPendingAttachmentAndReleasesReservation(t *testing.T) {
	now := time.Now().UTC()
	started := make(chan struct{})
	var once sync.Once
	released := make(chan struct{})
	route := openRoute(t, nativeView(now, 1), entryAcquirerFunc(func(ctx context.Context, _ entry.Attempt, _ entry.CandidateOpener) (net.Conn, func() error, error) {
		once.Do(func() { close(started) })
		<-ctx.Done()
		return nil, nil, ctx.Err()
	}), func(context.Context) (func() error, error) {
		return func() error { close(released); return nil }, nil
	})
	result := make(chan error, 1)
	go func() {
		_, err := route.Attach(context.Background(), Intent{Deadline: now.Add(time.Minute)})
		result <- err
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("Entry acquisition did not start")
	}
	if err := route.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancelled attachment error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Route Close did not join pending attachment")
	}
	select {
	case <-released:
	case <-time.After(time.Second):
		t.Fatal("Route Close left resource reservation live")
	}
}

func TestRouteCloseReleasesEveryActiveAttachment(t *testing.T) {
	now := time.Now().UTC()
	released := 0
	route := openRoute(t, nativeView(now, 1), successfulEntry(), func(context.Context) (func() error, error) {
		return func() error { released++; return nil }, nil
	})
	attachment, err := route.Attach(context.Background(), Intent{Deadline: now.Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	if err := route.Close(); err != nil {
		t.Fatal(err)
	}
	if released != 1 {
		t.Fatalf("active attachment release count = %d", released)
	}
	if err := attachment.Close(); err != nil {
		t.Fatalf("closed attachment terminal result = %v", err)
	}
	if attachment.LocalAddr() != nil {
		t.Fatal("Route Close left an active attachment carrier reusable")
	}
}

func TestRouteCloseJoinsConcurrentAttachmentCleanup(t *testing.T) {
	synctest.Test(t, routeCloseJoinsConcurrentAttachmentCleanup)
}

func routeCloseJoinsConcurrentAttachmentCleanup(t *testing.T) {
	now := time.Now().UTC()
	cleanupStarted := make(chan struct{})
	allowCleanup := make(chan struct{})
	cleanupFailure := errors.New("resource cleanup failed")
	var cleanupCalls atomic.Int32
	route := openRoute(t, nativeView(now, 1), successfulEntry(), func(context.Context) (func() error, error) {
		return func() error {
			cleanupCalls.Add(1)
			close(cleanupStarted)
			<-allowCleanup
			return cleanupFailure
		}, nil
	})
	attachment, err := route.Attach(context.Background(), Intent{Deadline: now.Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	attachmentResult := make(chan error, 1)
	go func() { attachmentResult <- attachment.Close() }()
	select {
	case <-cleanupStarted:
	case <-time.After(time.Second):
		t.Fatal("Attachment cleanup did not start")
	}
	routeResult := make(chan error, 1)
	go func() { routeResult <- route.Close() }()
	synctest.Wait()
	var routeErr error
	early := false
	select {
	case routeErr = <-routeResult:
		early = true
	default:
	}
	close(allowCleanup)
	attachmentErr := <-attachmentResult
	if early {
		t.Fatalf("Route Close returned before concurrent Attachment cleanup: %v", routeErr)
	}
	routeErr = <-routeResult
	if !errors.Is(attachmentErr, cleanupFailure) || !errors.Is(routeErr, cleanupFailure) {
		t.Fatalf("joined cleanup results = Attachment %v, Route %v", attachmentErr, routeErr)
	}
	if cleanupCalls.Load() != 1 {
		t.Fatalf("resource cleanup call count = %d", cleanupCalls.Load())
	}
}

func TestRouteCloseJoinsPostCleanupTerminalPublication(t *testing.T) {
	synctest.Test(t, routeCloseJoinsPostCleanupTerminalPublication)
}

func routeCloseJoinsPostCleanupTerminalPublication(t *testing.T) {
	now := time.Now().UTC()
	cleanupStarted := make(chan struct{})
	allowCleanup := make(chan struct{})
	cleanupFailure := errors.New("resource cleanup failed after carrier close")
	var cleanupCalls atomic.Int32
	route := openRoute(t, nativeView(now, 1), successfulEntry(), func(context.Context) (func() error, error) {
		return func() error {
			cleanupCalls.Add(1)
			close(cleanupStarted)
			<-allowCleanup
			return cleanupFailure
		}, nil
	})
	attachment, err := route.Attach(context.Background(), Intent{Deadline: now.Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	removalFinished := make(chan struct{})
	allowTerminalPublication := make(chan struct{})
	attachment.mu.Lock()
	rawCleanup := attachment.close
	attachment.close = func() error {
		result := rawCleanup()
		close(removalFinished)
		<-allowTerminalPublication
		return result
	}
	attachment.mu.Unlock()
	attachmentResult := make(chan error, 1)
	go func() { attachmentResult <- attachment.Close() }()
	<-cleanupStarted

	// Hold terminal publication after raw cleanup and the old release callback
	// return. The Route must keep this Attachment joinable across that boundary.
	close(allowCleanup)
	<-removalFinished
	routeResult := make(chan error, 1)
	go func() { routeResult <- route.Close() }()
	synctest.Wait()
	var routeErr error
	early := false
	select {
	case routeErr = <-routeResult:
		early = true
	default:
	}
	close(allowTerminalPublication)
	attachmentErr := <-attachmentResult
	if early {
		t.Fatalf("Route Close passed the post-cleanup terminal-publication gap: %v", routeErr)
	}
	routeErr = <-routeResult
	if !errors.Is(attachmentErr, cleanupFailure) || !errors.Is(routeErr, cleanupFailure) {
		t.Fatalf("post-cleanup terminal results = Attachment %v, Route %v", attachmentErr, routeErr)
	}
	if cleanupCalls.Load() != 1 {
		t.Fatalf("resource cleanup call count = %d", cleanupCalls.Load())
	}
}

func TestRouteCloseReportsCleanupFromAttachmentOpenedWhileClosing(t *testing.T) {
	synctest.Test(t, routeCloseReportsCleanupFromAttachmentOpenedWhileClosing)
}

func routeCloseReportsCleanupFromAttachmentOpenedWhileClosing(t *testing.T) {
	now := time.Now().UTC()
	entryStarted := make(chan struct{})
	allowEntry := make(chan struct{})
	cleanupFailure := errors.New("late attachment resource cleanup failed")
	var cleanupCalls atomic.Int32
	route := openRoute(t, nativeView(now, 1), entryAcquirerFunc(func(context.Context, entry.Attempt, entry.CandidateOpener) (net.Conn, func() error, error) {
		close(entryStarted)
		<-allowEntry
		connection, cleanup := relayReadyPipe()
		return connection, cleanup, nil
	}), func(context.Context) (func() error, error) {
		return func() error {
			cleanupCalls.Add(1)
			return cleanupFailure
		}, nil
	})
	attachResult := make(chan error, 1)
	go func() {
		_, err := route.Attach(context.Background(), Intent{Deadline: now.Add(time.Minute)})
		attachResult <- err
	}()
	<-entryStarted
	closeResult := make(chan error, 1)
	go func() { closeResult <- route.Close() }()
	synctest.Wait()
	route.mu.Lock()
	closed := route.closed
	route.mu.Unlock()
	if !closed {
		t.Fatal("Route Close did not reach the closed state while attachment opening was blocked")
	}
	close(allowEntry)
	attachErr := <-attachResult
	closeErr := <-closeResult
	if !errors.Is(attachErr, cleanupFailure) {
		t.Fatalf("late Attach cleanup result = %v", attachErr)
	}
	if !errors.Is(closeErr, cleanupFailure) {
		t.Fatalf("Route Close lost late attachment cleanup failure: %v", closeErr)
	}
	if cleanupCalls.Load() != 1 {
		t.Fatalf("late attachment resource cleanup call count = %d", cleanupCalls.Load())
	}
}

func TestRouteBoundsRememberedCleanupFailure(t *testing.T) {
	now := time.Now().UTC()
	firstFailure := errors.New("first resource cleanup failed")
	secondFailure := errors.New("second resource cleanup failed")
	failures := []error{firstFailure, secondFailure}
	cleanup := 0
	route := openRoute(t, nativeView(now, 1), successfulEntry(), func(context.Context) (func() error, error) {
		failure := failures[cleanup]
		cleanup++
		return func() error { return failure }, nil
	})
	for index, failure := range failures {
		attachment, err := route.Attach(context.Background(), Intent{Deadline: now.Add(time.Minute)})
		if err != nil {
			t.Fatalf("Attachment %d: %v", index+1, err)
		}
		if err := attachment.Close(); !errors.Is(err, failure) {
			t.Fatalf("Attachment %d cleanup = %v", index+1, err)
		}
	}
	if !errors.Is(route.terminalFailure, firstFailure) || errors.Is(route.terminalFailure, secondFailure) {
		t.Fatalf("remembered cleanup failure = %v, want first failure only", route.terminalFailure)
	}
	if err := route.Close(); !errors.Is(err, firstFailure) || errors.Is(err, secondFailure) {
		t.Fatalf("bounded Route cleanup evidence = %v", err)
	}
}

func openRoute(t *testing.T, snapshot state.Snapshot, source EntryAcquirer, admit ResourceAdmission) *Route {
	t.Helper()
	opened, err := Open(Config{View: viewFunc(func() (state.Snapshot, error) { return snapshot, nil }), Entry: source, Admit: admit})
	if err != nil {
		t.Fatal(err)
	}
	return opened
}

type viewFunc func() (state.Snapshot, error)

func (call viewFunc) Current() (state.Snapshot, error) { return call() }

func successfulEntry() EntryAcquirer {
	return entryAcquirerFunc(func(context.Context, entry.Attempt, entry.CandidateOpener) (net.Conn, func() error, error) {
		connection, cleanup := relayReadyPipe()
		return connection, cleanup, nil
	})
}

func relayReadyPipe() (net.Conn, func() error) {
	left, right := net.Pipe()
	go func() {
		setup, err := ReadRelaySetup(right)
		if err == nil {
			_ = WriteRelayReady(right, RelayReady{Setup: setup})
		}
	}()
	return left, right.Close
}

func admitted() ResourceAdmission {
	return func(context.Context) (func() error, error) { return func() error { return nil }, nil }
}

func nativeView(now time.Time, alternatives int) state.Snapshot {
	view := state.Snapshot{Generation: "generation", NetworkID: [32]byte{1}, Epoch: 1, Digest: [32]byte{2},
		ValidUntil: now.Add(time.Hour), Profile: Profile, ViewRoot: [32]byte{3}, Freshness: "fresh", TrustedTime: now}
	for roleIndex, role := range routeRoles {
		for alternative := range alternatives {
			index := roleIndex*alternatives + alternative
			candidate := &view.Candidates[index]
			candidate.NodeID = [32]byte{byte(index + 1)}
			candidate.PublicKey = [32]byte{byte(index + 21)}
			candidate.Family = role + string(rune('a'+alternative))
			candidate.Endpoint = "127.0.0.1:" + string(rune('1'+index))
			candidate.Capacity, candidate.Domain = 1, role
			candidate.ValidFrom, candidate.ValidUntil = now.Add(-time.Minute), now.Add(time.Hour)
			candidate.AssignmentNotAfter = now.Add(30 * time.Minute)
		}
	}
	view.CandidateCount = uint8(len(routeRoles) * alternatives)
	return view
}
