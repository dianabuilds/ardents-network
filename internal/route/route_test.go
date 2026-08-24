package route

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
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
	if err := attachment.Close(); err == nil {
		t.Fatal("Route Close left an active attachment reusable")
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
