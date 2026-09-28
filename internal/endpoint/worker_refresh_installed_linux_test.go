//go:build linux && text_worker_installed

package endpoint

import (
	"context"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/endpoint/introduction"
)

// Observe the installed Publisher's actual scheduler. No authority clock,
// registration creation time, refresh time or predecessor deadline is changed.
func observeInstalledRefresh(t *testing.T, ctx context.Context, owner *dutyContext, run *publisherRun) {
	t.Helper()
	owner.mu.Lock()
	first := owner.publication.pair.CurrentLocked()
	if first == nil || !first.PublishedLocked() {
		owner.mu.Unlock()
		t.Fatal("no initial published registration")
	}
	created := introduction.CreatedAt(first)
	scheduled, expiry := first.RefreshScheduleLocked()
	initialSlot, initialKey, initialRevision := introduction.Slot(first), first.RecipientPublicLocked(time.Now().UTC()), first.Revision()
	owner.mu.Unlock()
	if scheduled != created.Add(300*time.Second) || initialKey == [32]byte{} {
		t.Fatal("initial refresh schedule or recipient invalid")
	}
	var second *introduction.Registration
	var cutoff time.Time
	for {
		owner.mu.Lock()
		second = owner.publication.pair.CurrentLocked()
		changed := owner.publication.pair.ChangedLocked()
		switched := false
		if second != nil && second != first && second.PublishedLocked() {
			refreshAt, _ := second.RefreshScheduleLocked()
			switched = !refreshAt.IsZero()
		}
		if switched {
			// Read the timestamp recorded by the verified ACK transition itself.
			previous, until := owner.publication.pair.PreviousLocked()
			cutoff = until
			switchedAt := introduction.PublishedAt(second)
			secondCreated := introduction.CreatedAt(second)
			valid := previous == first && introduction.Slot(second) != initialSlot && second.Revision() > initialRevision && second.RecipientPublicLocked(time.Now().UTC()) != initialKey
			owner.mu.Unlock()
			if !valid || secondCreated.Before(scheduled) || time.Now().Before(scheduled) || switchedAt.IsZero() || switchedAt.Before(scheduled) || cutoff.After(switchedAt.Add(60*time.Second)) || cutoff.After(expiry) {
				t.Fatal("actual refresh violated slot, revision or predecessor bound")
			}
			break
		}
		owner.mu.Unlock()
		select {
		case <-changed:
		case <-run.done:
			t.Fatal("Publisher ended before actual refresh")
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	// Allow two seconds to observe joined cleanup after the exact acceptance
	// deadline. This does not extend the predecessor's authorization deadline.
	retirement, cancel := context.WithDeadline(ctx, cutoff.Add(2*time.Second))
	defer cancel()
	for {
		owner.mu.Lock()
		previous, previousUntil := owner.publication.pair.PreviousLocked()
		retired := previous == nil
		changed := owner.publication.pair.ChangedLocked()
		current := owner.publication.pair.CurrentLocked() == second && second.PublishedLocked()
		unchanged := retired || previous == first && previousUntil == cutoff
		owner.mu.Unlock()
		if !unchanged {
			t.Fatal("predecessor deadline or identity changed after publication")
		}
		if retired {
			if time.Now().Before(cutoff) || !current || first.RecipientPublicLocked(time.Now().UTC()) != [32]byte{} || second.RecipientPublicLocked(time.Now().UTC()) == [32]byte{} {
				t.Fatal("predecessor retirement lost current publication or retained its key")
			}
			select {
			case <-first.DoneSignal():
			default:
				t.Fatal("predecessor channel remained live")
			}
			break
		}
		select {
		case <-changed:
		case <-run.done:
			t.Fatal("Publisher ended during predecessor retirement")
		case <-retirement.Done():
			t.Fatal("predecessor cleanup was not observed within its bound")
		}
	}
	t.Logf("actual-refresh-age=%s; predecessor-retirement-observed=%s; clocks-unmodified=true", time.Since(created), time.Since(cutoff))
}
