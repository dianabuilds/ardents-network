//go:build linux && text_worker_installed

package endpoint

import (
	"context"
	"testing"
	"time"
)

// Observe the installed Publisher's actual scheduler. No authority clock,
// registration creation time, refresh time or predecessor deadline is changed.
func observeInstalledTextRefresh(t *testing.T, ctx context.Context, owner *textContext, run *textPublisherRun) {
	t.Helper()
	owner.mu.Lock()
	first := owner.registration
	if first == nil || !first.published {
		owner.mu.Unlock()
		t.Fatal("no initial published registration")
	}
	created, scheduled, expiry := first.createdAt, first.refreshAt, first.request.Expiry
	initialSlot, initialKey, initialRevision := first.request.Slot, first.recipient.Public(time.Now().UTC()), first.request.Revision
	owner.mu.Unlock()
	if scheduled != created.Add(300*time.Second) || initialKey == [32]byte{} {
		t.Fatal("initial refresh schedule or recipient invalid")
	}
	var second *textIntroductionRegistration
	var cutoff time.Time
	for {
		owner.mu.Lock()
		second = owner.registration
		changed := owner.registrationChanged
		switched := second != nil && second != first && second.published && !second.refreshAt.IsZero()
		if switched {
			// Read the timestamp recorded by the verified ACK transition itself.
			cutoff = owner.previousUntil
			switchedAt := second.publishedAt
			valid := owner.previousRegistration == first && second.request.Slot != initialSlot && second.request.Revision > initialRevision && second.recipient.Public(time.Now().UTC()) != initialKey
			owner.mu.Unlock()
			if !valid || second.createdAt.Before(scheduled) || time.Now().Before(scheduled) || switchedAt.IsZero() || switchedAt.Before(scheduled) || cutoff.After(switchedAt.Add(60*time.Second)) || cutoff.After(expiry) {
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
		retired := owner.previousRegistration == nil
		changed := owner.registrationChanged
		current := owner.registration == second && second.published
		unchanged := retired || owner.previousRegistration == first && owner.previousUntil == cutoff
		owner.mu.Unlock()
		if !unchanged {
			t.Fatal("predecessor deadline or identity changed after publication")
		}
		if retired {
			if time.Now().Before(cutoff) || !current || first.recipient.Public(time.Now().UTC()) != [32]byte{} || second.recipient.Public(time.Now().UTC()) == [32]byte{} {
				t.Fatal("predecessor retirement lost current publication or retained its key")
			}
			select {
			case <-first.channel.Done():
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
