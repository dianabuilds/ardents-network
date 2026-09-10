//go:build linux

package endpoint

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

// Only timer scheduling, accepted State and worker qualification are fixtures.
// Both key generations, issuance, registration, publication and capsule delivery
// use their real owners over each Carrier. No elapsed-300s or JOIN claim.
func TestTextPublicationAutomaticallyRefreshesAndRetiresPredecessor(t *testing.T) {
	for _, carrier := range []route.CarrierProfile{route.ClosedCarrierTCP, route.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			gate := newTextDescriptorACKGate()
			defer gate.open()
			endpoint, owner, source, first := startTextRegisteredPublisherNetwork(t, carrier, gate)
			now := time.Now().UTC().Truncate(time.Second)
			// Age the actual slot before publication. A late first ACK must not
			// restart the original 300-second age from the acknowledgement.
			timer := time.NewTimer(time.Until(first.createdAt.Add(2 * time.Second)))
			select {
			case <-timer.C:
			case <-t.Context().Done():
				timer.Stop()
				t.Fatal(t.Context().Err())
			}
			published, err := owner.publishTextDescriptor(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			owner.mu.Lock()
			refresh, originalAt := owner.refresh, first.refreshAt
			owner.mu.Unlock()
			if refresh == nil || originalAt != first.createdAt.Add(300*time.Second) || time.Until(originalAt) > 298*time.Second {
				t.Fatal("verified publication did not schedule its bounded refresh")
			}
			if _, err := owner.publishTextDescriptor(t.Context()); err != nil {
				t.Fatal(err)
			}
			owner.mu.Lock()
			unchanged := first.refreshAt == originalAt
			owner.mu.Unlock()
			if !unchanged {
				t.Fatal("exact retry extended refresh deadline")
			}
			// A refused withdrawal cannot cancel the publication's scheduler.
			owner.mu.Lock()
			owner.resolution = &textResolutionFlight{}
			owner.mu.Unlock()
			withdrawErr := owner.withdrawTextIntroduction(t.Context())
			owner.mu.Lock()
			owner.resolution = nil
			stillScheduled := owner.refresh == refresh && refresh.context.Err() == nil
			owner.mu.Unlock()
			if withdrawErr == nil || !stillScheduled {
				t.Fatal("refused withdrawal stopped automatic refresh")
			}
			reader := textPermissionContextFixture(t, endpoint, fixtureID(211), broker.Connection)
			source.issuePermission(t, reader, [3]uint32{64, 64, 0})
			readerJob, publisherJob := liveTextCapsuleJob(t, reader), liveTextCapsuleJob(t, owner)
			bounds := [3]int64{now.Add(time.Minute).Unix(), now.Add(time.Minute).Unix(), now.Add(time.Minute).Unix()}
			link := targetlink.Link{Network: endpoint.network, Target: published.Descriptor.Target}
			oldAttempt, err := reader.prepareTextIntroduction(t.Context(), readerJob, link, bounds)
			if err != nil {
				t.Fatal(err)
			}
			// A real retired Source must be reopened from retained selection/stock.
			owner.mu.Lock()
			oldSource, retainedSet := owner.prefix, owner.sourceSet
			owner.mu.Unlock()
			if err := oldSource.Close(); err != nil {
				t.Fatal(err)
			}
			gate.arm(t)
			owner.mu.Lock()
			first.refreshAt = time.Now().Add(-time.Second)
			owner.signalTextRegistrationsLocked()
			owner.mu.Unlock()
			select {
			case <-gate.held:
			case <-time.After(10 * time.Second):
				t.Fatal("replacement did not reach actual Store commit before ACK")
			}
			// Separate registration from ACK by a full clock tick; otherwise a
			// premature whole-second cutoff could pass the same-second oracle.
			ackTimer := time.NewTimer(time.Until(time.Now().UTC().Truncate(time.Second).Add(2 * time.Second)))
			select {
			case <-ackTimer.C:
			case <-t.Context().Done():
				ackTimer.Stop()
				t.Fatal(t.Context().Err())
			}
			deliverTextBeforeDescriptorACK(t, gate, reader, owner, readerJob, publisherJob, oldAttempt)
			pendingOperation := refuseTextBeforeDescriptorACK(t, gate, owner, publisherJob, oldAttempt)
			switchEarliest := time.Now().UTC().Truncate(time.Second)
			gate.open()
			var second *textIntroductionRegistration
			waitTextRefreshCondition(t, owner, func() bool {
				second = owner.registration
				return second != nil && second != first && !second.refreshAt.IsZero()
			})
			if _, err := owner.acceptTextIntroduction(t.Context(), publisherJob, pendingOperation); err != nil {
				t.Fatalf("same new capsule was not accepted after Descriptor ACK: %v", err)
			}
			owner.mu.Lock()
			valid := owner.previousRegistration == first && owner.previousUntil.After(time.Now()) &&
				!owner.previousUntil.After(time.Now().Add(60*time.Second)) &&
				owner.prefix != nil && owner.prefix != oldSource && owner.sourceSet == retainedSet
			owner.mu.Unlock()
			if !valid || first.recipient.Public(time.Now()) == [32]byte{} || second.recipient.Public(time.Now()) == [32]byte{} {
				t.Fatal("refresh lost bounded predecessor, source selection, or independent keys")
			}
			owner.mu.Lock()
			cutoff := owner.previousUntil
			acknowledgedAt := second.publishedAt
			if acknowledgedAt.Before(switchEarliest) || acknowledgedAt.After(time.Now()) || cutoff.After(acknowledgedAt.Add(60*time.Second)) {
				owner.mu.Unlock()
				t.Fatal("publication ACK timestamp or predecessor bound is invalid")
			}
			owner.mu.Unlock()
			if first.request.Expiry.After(switchEarliest.Add(60*time.Second)) && cutoff.Before(switchEarliest.Add(60*time.Second)) {
				t.Fatal("predecessor overlap started before Descriptor ACK")
			}
			if _, err := owner.publishTextDescriptor(t.Context()); err != nil {
				t.Fatal(err)
			}
			owner.mu.Lock()
			sameCutoff := owner.previousUntil == cutoff && second.publishedAt == acknowledgedAt
			owner.mu.Unlock()
			if !sameCutoff {
				t.Fatal("exact Descriptor retry extended predecessor cutoff")
			}
			raw := lookupTextPublishedProof(t, owner, link.Target)
			current, err := reachability.VerifyPrivate(raw, link.Target, endpoint.network, source.view.Profile.Digest, time.Now().UTC())
			if err != nil || current.Current.Digest != published.Current.Digest || current.Descriptor.Private.Revision != 2 ||
				current.Descriptor.Private.Slot == published.Descriptor.Private.Slot || current.Descriptor.Private.RecipientKey == published.Descriptor.Private.RecipientKey {
				t.Fatalf("replacement changed authority or reused recipient: %v", err)
			}
			newAttempt, err := reader.prepareTextIntroduction(t.Context(), readerJob, link, bounds)
			if err != nil {
				t.Fatal(err)
			}
			deliverTextRefreshAttempt(t, reader, owner, readerJob, publisherJob, newAttempt)
			// Advance only retirement scheduling, not any signature or authority clock.
			owner.mu.Lock()
			owner.previousUntil = time.Now().Add(-time.Second)
			owner.signalTextRegistrationsLocked()
			owner.mu.Unlock()
			waitTextRefreshCondition(t, owner, func() bool { return owner.previousRegistration == nil })
			select {
			case <-first.channel.Done():
			default:
				t.Fatal("retired predecessor channel remained live")
			}
			if first.recipient.Public(time.Now()) != [32]byte{} || second.recipient.Public(time.Now()) == [32]byte{} {
				t.Fatal("retirement did not independently erase the predecessor")
			}
			if err := owner.withdrawTextIntroduction(t.Context()); err != nil {
				t.Fatal(err)
			}
			select {
			case <-refresh.done:
			default:
				t.Fatal("withdrawal did not join refresh")
			}
			if second.recipient.Public(time.Now()) != [32]byte{} {
				t.Fatal("withdrawal retained current key")
			}
		})
	}
}

func waitTextRefreshCondition(t *testing.T, owner *textContext, condition func() bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		owner.mu.Lock()
		ok := condition()
		var err error
		if owner.refresh != nil {
			err = owner.refresh.err
		}
		owner.mu.Unlock()
		if ok {
			return
		}
		if err != nil {
			t.Fatalf("automatic refresh: %v", err)
		}
		select {
		case <-ctx.Done():
			t.Fatal("automatic refresh transition did not complete")
		case <-ticker.C:
		}
	}
}

func deliverTextRefreshAttempt(t *testing.T, reader, publisher *textContext, readerJob, publisherJob *textJobIdentity, prepared *textIntroductionAttempt) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()
	result := make(chan error, 1)
	go func() {
		accepted, err := publisher.receiveTextIntroduction(ctx, publisherJob)
		if err == nil && (accepted.digest != prepared.digest || accepted.plaintext != prepared.plaintext) {
			err = context.Canceled
		}
		result <- err
	}()
	sent := reader.submitTextIntroduction(ctx, readerJob, prepared)
	if sent != nil {
		cancel()
	}
	received := <-result
	if sent != nil || received != nil {
		t.Fatalf("real refreshed delivery: submit=%v receive=%v", sent, received)
	}
}

func TestTextRefreshRetainsOriginalCleanupFailure(t *testing.T) {
	endpoint, principal := textContextEndpoint(t)
	owner := admittedTextContext(t, endpoint, principal, broker.Administration)
	ctx, cancel := context.WithCancel(owner.lease.Context())
	defer cancel()
	flight := &textPublicationRefresh{context: ctx, cancel: cancel, done: make(chan struct{}), wake: make(chan struct{}, 1)}
	owner.mu.Lock()
	owner.refresh = flight
	owner.mu.Unlock()
	failed := errors.New("predecessor cleanup did not join")
	owner.failTextRefresh(flight, errors.Join(route.ErrClosedSourceCleanup, failed))
	close(flight.done)
	if _, err := owner.beginJob(endpoint, broker.Administration); err == nil {
		// Avoid leaving an unfinished fixture job behind on the failing version.
		owner.retireJob(owner.job)
		_ = owner.finishJobCleanup(owner.job, nil)
		t.Error("cleanup failure left admission available")
	}
	for range 2 {
		if err := owner.Close(); !errors.Is(err, failed) {
			t.Errorf("Close lost original cleanup failure: %v", err)
		}
	}
}
