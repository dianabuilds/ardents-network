package transfer

import (
	"errors"
	"testing"
	"time"
)

func recoveryFixture(t *testing.T) (*RecoveryEpisode, time.Time) {
	t.Helper()
	started := time.Unix(123456, 0)
	episode, err := NewRecoveryEpisode(1, started, started.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	return episode, started
}

func TestRecoveryLossStartsDeadlineBeforeProposal(t *testing.T) {
	episode, started := recoveryFixture(t)
	if err := episode.Start(1, started.Add(4*time.Second)); err != nil {
		t.Fatal(err)
	}
	want := started.Add(15*time.Second - 10*time.Millisecond)
	if episode.active != nil || episode.proposals != 0 || !episode.deadline.Equal(want) {
		t.Fatal("waiting for a peer reserved a proposal or changed the original deadline")
	}
	episode.Progress(started.Add(8 * time.Second))
	if err := episode.Start(1, started.Add(10*time.Second)); err != nil {
		t.Fatal(err)
	}
	ticket, err := episode.Begin(1, started.Add(12*time.Second))
	if err != nil || !ticket.deadline.Equal(want) || episode.proposals != 1 {
		t.Fatalf("delayed proposal renewed the loss deadline: %+v %v", ticket, err)
	}
	if err := episode.Commit(ticket, want); !errors.Is(err, ErrRecoveryExhausted) || ticket.consumed {
		t.Fatalf("waiting deadline admitted a late commit: %v", err)
	}
	if !episode.CanRetry(want.Add(-time.Nanosecond)) || episode.CanRetry(want) {
		t.Fatal("retry budget did not preserve the exact deadline boundary")
	}
}

func TestRecoveryEpisodeRetainsOriginalDeadlineAndThreeProposals(t *testing.T) {
	episode, started := recoveryFixture(t)
	wantDeadline := started.Add(15*time.Second - 10*time.Millisecond)
	physical := errors.New("original failed JOIN")
	for i := range 3 {
		ticket, err := episode.Begin(1, started.Add(time.Duration(i+4)*time.Second))
		if err != nil || ticket.Generation() != 2 || !ticket.Deadline().Equal(wantDeadline) || ticket.IsRetry() != (i > 0) {
			t.Fatalf("proposal %d: ticket=%+v err=%v", i, ticket, err)
		}
		// Another observer of the same loss cannot open a competing attempt
		// or spend another proposal while the original one remains retained.
		if _, err := episode.Begin(1, started.Add(7*time.Second)); err == nil || episode.proposals != i+1 {
			t.Fatal("concurrent loss admitted another proposal")
		}
		if err := episode.Reject(ticket, physical); err != physical {
			t.Fatalf("original physical failure lost: %v", err)
		}
	}
	if _, err := episode.Begin(1, started.Add(8*time.Second)); !errors.Is(err, ErrRecoveryExhausted) {
		t.Fatalf("fourth proposal admitted: %v", err)
	}
	if episode.generation != 1 || episode.proposals != 3 || !episode.deadline.Equal(wantDeadline) {
		t.Fatal("failed proposals renewed generation, count or deadline")
	}
	if episode.CanRetry(started.Add(8 * time.Second)) {
		t.Fatal("exhausted episode advertised another proposal")
	}
}

func TestRecoveryEpisodeDoesNotRenewDeadlineWithProgress(t *testing.T) {
	episode, started := recoveryFixture(t)
	episode.Progress(started.Add(time.Second))
	ticket, err := episode.Begin(1, started.Add(4*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	want := started.Add(16*time.Second - 10*time.Millisecond)
	if !ticket.deadline.Equal(want) {
		t.Fatalf("episode did not start at last progress: %v", ticket.deadline)
	}
	episode.Progress(started.Add(9 * time.Second))
	if err := episode.Reject(ticket, errors.New("loss")); err == nil {
		t.Fatal("lost physical failure")
	}
	next, err := episode.Begin(1, started.Add(10*time.Second))
	if err != nil || !next.deadline.Equal(want) {
		t.Fatalf("progress extended active episode: %+v %v", next, err)
	}
	if err := episode.Commit(next, want); !errors.Is(err, ErrRecoveryExhausted) {
		t.Fatalf("exact deadline admitted late Continuity: %v", err)
	}
	if episode.active != next || next.consumed || episode.generation != 1 {
		t.Fatal("late commit released unjoined proposal or advanced generation")
	}
}

func TestRecoveryEpisodeSuccessAdvancesOnceWithoutRenewingCutoff(t *testing.T) {
	episode, started := recoveryFixture(t)
	cutoff := episode.cutoff
	ticket, err := episode.Begin(1, started.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err := episode.Commit(ticket, started.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	if episode.generation != 2 || episode.active != nil || episode.proposals != 0 || !episode.deadline.IsZero() || !episode.cutoff.Equal(cutoff) {
		t.Fatal("commit did not transfer exactly one generation and retain original cutoff")
	}
	if err := episode.Commit(ticket, started.Add(3*time.Second)); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("consumed proposal committed again: %v", err)
	}
	if _, err := episode.Begin(1, started.Add(3*time.Second)); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("stale loss opened another proposal: %v", err)
	}
	// Successful Continuity alone is not new Application progress.
	next, err := episode.Begin(2, started.Add(4*time.Second))
	if err != nil || next.generation != 3 || !next.deadline.Equal(started.Add(15*time.Second-10*time.Millisecond)) {
		t.Fatalf("commit renewed progress or failed to advance: %+v %v", next, err)
	}
}

func TestRecoveryEpisodeStopRetainsOriginalJoinObligation(t *testing.T) {
	episode, started := recoveryFixture(t)
	ticket, err := episode.Begin(1, started.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	episode.Stop()
	if episode.CanRetry(started.Add(2 * time.Second)) {
		t.Fatal("stopped episode advertised another proposal")
	}
	episode.Progress(started.Add(10 * time.Second))
	if err := episode.Commit(ticket, started.Add(2*time.Second)); !errors.Is(err, ErrRecoveryExhausted) {
		t.Fatalf("late accepted proposal resurrected stopped owner: %v", err)
	}
	if episode.active != ticket || ticket.consumed || !episode.lastProgress.Equal(started) {
		t.Fatal("stop discarded original physical obligation or accepted late progress")
	}
	physical := errors.New("rejected proposal physical close failed")
	if err := episode.Reject(ticket, physical); err != physical {
		t.Fatalf("stopped proposal lost join failure: %v", err)
	}
	if _, err := episode.Begin(1, started.Add(3*time.Second)); !errors.Is(err, ErrRecoveryExhausted) {
		t.Fatalf("stopped owner reopened: %v", err)
	}
}

func TestRecoveryEpisodeRejectsForeignAndCopiedTickets(t *testing.T) {
	episode, started := recoveryFixture(t)
	ticket, err := episode.Begin(1, started.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	copyTicket := *ticket
	physical := errors.New("physical failure")
	for _, foreign := range []*RecoveryTicket{nil, &copyTicket, {generation: 2, deadline: ticket.deadline}} {
		if err := episode.Commit(foreign, started.Add(2*time.Second)); !errors.Is(err, ErrIntegrity) {
			t.Fatalf("foreign proposal committed: %v", err)
		}
		if err := episode.Reject(foreign, physical); !errors.Is(err, ErrIntegrity) || !errors.Is(err, physical) {
			t.Fatalf("foreign rejection accepted or actual failure lost: %v", err)
		}
	}
	if err := episode.Reject(ticket, nil); !errors.Is(err, ErrIntegrity) || episode.active != ticket || ticket.consumed {
		t.Fatal("nil failure retired proposal")
	}
	if err := episode.Reject(ticket, physical); err != physical {
		t.Fatal(err)
	}
	if err := episode.Reject(ticket, physical); !errors.Is(err, ErrIntegrity) || !errors.Is(err, physical) {
		t.Fatalf("consumed proposal reused or error erased: %v", err)
	}
}

func TestRecoveryEpisodeCutoffIdleExpiryAndGenerationOverflow(t *testing.T) {
	started := time.Unix(123456, 0)
	episode, err := NewRecoveryEpisode(1, started, started.Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	ticket, err := episode.Begin(1, started.Add(time.Second))
	if err != nil || !ticket.deadline.Equal(started.Add(3*time.Second)) {
		t.Fatalf("proposal extended original safety cutoff: %+v %v", ticket, err)
	}
	if err := episode.Commit(ticket, started.Add(3*time.Second)); !errors.Is(err, ErrRecoveryExhausted) {
		t.Fatalf("commit at expired authority admitted: %v", err)
	}
	idle, _ := recoveryFixture(t)
	if _, err := idle.Begin(1, started.Add(16*time.Second)); !errors.Is(err, ErrRecoveryExhausted) || idle.proposals != 0 {
		t.Fatalf("idle detection renewed the expired episode: %v", err)
	}
	overflow, err := NewRecoveryEpisode(^uint64(0), started, started.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := overflow.Begin(^uint64(0), started.Add(time.Second)); !errors.Is(err, ErrIntegrity) || overflow.proposals != 0 {
		t.Fatalf("overflow generation admitted: %v", err)
	}
	for _, input := range []struct {
		generation    uint64
		start, cutoff time.Time
	}{
		{0, started, started.Add(time.Minute)}, {1, time.Time{}, started}, {1, started, started},
	} {
		if _, err := NewRecoveryEpisode(input.generation, input.start, input.cutoff); !errors.Is(err, ErrIntegrity) {
			t.Fatalf("invalid initial bound admitted: %+v %v", input, err)
		}
	}
}
