//go:build linux

package endpoint

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/endpoint/publication"
	"github.com/dianabuilds/ardents-network/internal/network/duty"
	routecarrier "github.com/dianabuilds/ardents-network/internal/route/carrier"
	"github.com/dianabuilds/ardents-network/internal/route/client"
	"github.com/dianabuilds/ardents-network/internal/service/reachability"
	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

func TestTextPublicationRefreshRetriesConcurrentRoleCommit(t *testing.T) {
	gate := newDescriptorACKGate()
	gate.open()
	endpoint, owner, _, first := startRegisteredPublisherNetwork(t, routecarrier.ClosedCarrierQUIC, gate)
	if _, err := owner.publishDescriptor(t.Context()); err != nil {
		t.Fatal(err)
	}
	owner.mu.Lock()
	oldSource, refresh := owner.source.currentLocked(), owner.publication.refresh.Current()
	owner.mu.Unlock()
	if oldSource == nil || refresh == nil {
		t.Fatal("published registration has no Source or refresh owner")
	}
	if err := oldSource.Close(); err != nil {
		t.Fatal(err)
	}
	writer, err := duty.Open(duty.Config{Root: endpoint.closedRoleRoot, Clock: time.Now})
	if err != nil {
		t.Fatal(err)
	}
	owner.mu.Lock()
	first.refreshAt = time.Now().Add(-time.Second)
	owner.publication.signalRegistrationsLocked()
	owner.mu.Unlock()
	timer := time.NewTimer(1200 * time.Millisecond)
	select {
	case <-refresh.Done:
		timer.Stop()
		_ = writer.Close()
		t.Fatalf("transient local-role commit ended refresh: %v", owner.publication.refresh.Outcome(refresh))
	case <-timer.C:
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	waitRefreshCondition(t, owner, func() bool {
		return owner.publication.pair.registration != nil && owner.publication.pair.registration != first && owner.publication.pair.previousRegistration == first
	})
}

func TestTextPublicationRefreshRetriesOnlyConflictReadTimeout(t *testing.T) {
	contention := roleMemberFailureAt("conflict-read", context.DeadlineExceeded)
	for name, test := range map[string]struct {
		cause error
		want  bool
	}{
		"contention":       {contention, true},
		"other role stage": {roleMemberFailureAt("binding", context.DeadlineExceeded), false},
		"other cause":      {roleMemberFailureAt("conflict-read", errors.New("corrupt generation")), false},
		"bare timeout":     {context.DeadlineExceeded, false},
	} {
		t.Run(name, func(t *testing.T) {
			if got := refreshSourceContention(test.cause); got != test.want {
				t.Fatalf("retry classification = %t", got)
			}
		})
	}
}

// Only timer scheduling, accepted State and worker qualification are fixtures.
// Both key generations, issuance, registration, publication and capsule delivery
// use their real owners over each Carrier. No elapsed-300s or JOIN claim.
func TestTextPublicationAutomaticallyRefreshesAndRetiresPredecessor(t *testing.T) {
	for _, carrier := range []routecarrier.CarrierProfile{routecarrier.ClosedCarrierTCP, routecarrier.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			gate := newDescriptorACKGate()
			defer gate.open()
			endpoint, owner, source, first := startRegisteredPublisherNetwork(t, carrier, gate)
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
			published, err := owner.publishDescriptor(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			owner.mu.Lock()
			refresh, originalAt, originalExpiry := owner.publication.refresh.Current(), first.refreshAt, first.request.Expiry
			owner.mu.Unlock()
			if refresh == nil || originalAt != first.createdAt.Add(300*time.Second) || time.Until(originalAt) > 298*time.Second {
				t.Fatal("verified publication did not schedule its bounded refresh")
			}
			owner.mu.Lock()
			for range 16 {
				owner.startRefreshLocked(first)
				owner.publication.signalRegistrationsLocked()
			}
			sameScheduler := owner.publication.refresh.Current() == refresh
			unchangedBounds := first.refreshAt == originalAt && first.request.Expiry == originalExpiry
			owner.mu.Unlock()
			if !sameScheduler || !unchangedBounds {
				t.Fatal("repeated ACK wake replaced scheduler or moved refresh bounds")
			}
			if _, err := owner.publishDescriptor(t.Context()); err != nil {
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
			owner.resolution = &resolutionFlight{}
			owner.mu.Unlock()
			withdrawErr := owner.withdrawIntroduction(t.Context())
			owner.mu.Lock()
			owner.resolution = nil
			stillScheduled := owner.publication.refresh.Current() == refresh && refresh.Context.Err() == nil
			owner.mu.Unlock()
			if withdrawErr == nil || !stillScheduled {
				t.Fatal("refused withdrawal stopped automatic refresh")
			}
			reader := permissionContextFixture(t, endpoint, fixtureID(211), broker.Connection)
			source.issuePermission(t, reader, [3]uint32{64, 64, 0})
			readerJob, publisherJob := liveCapsuleJob(t, reader), liveCapsuleJob(t, owner)
			bounds := [3]int64{now.Add(time.Minute).Unix(), now.Add(time.Minute).Unix(), now.Add(time.Minute).Unix()}
			link := targetlink.Link{Network: endpoint.network, Target: published.Descriptor.Target}
			oldAttempt, err := reader.prepareIntroduction(t.Context(), readerJob, link, bounds)
			if err != nil {
				t.Fatal(err)
			}
			// A real retired Source must be reopened from retained selection/stock.
			owner.mu.Lock()
			oldSource, retainedSet := owner.source.currentLocked(), owner.source.set
			owner.mu.Unlock()
			if err := oldSource.Close(); err != nil {
				t.Fatal(err)
			}
			gate.arm(t)
			owner.mu.Lock()
			first.refreshAt = time.Now().Add(-time.Second)
			owner.publication.signalRegistrationsLocked()
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
			oldAccepted := deliverBeforeDescriptorACK(t, gate, reader, owner, readerJob, publisherJob, oldAttempt)
			pendingOperation := refuseBeforeDescriptorACK(t, gate, owner, publisherJob, oldAttempt)
			switchEarliest := time.Now().UTC().Truncate(time.Second)
			gate.open()
			var second *introductionRegistration
			waitRefreshCondition(t, owner, func() bool {
				second = owner.publication.pair.registration
				return second != nil && second != first && !second.refreshAt.IsZero()
			})
			if _, err := owner.acceptIntroduction(t.Context(), publisherJob, pendingOperation); err != nil {
				t.Fatalf("same new capsule was not accepted after Descriptor ACK: %v", err)
			}
			owner.mu.Lock()
			valid := owner.publication.pair.previousRegistration == first && owner.publication.pair.previousUntil.After(time.Now()) &&
				!owner.publication.pair.previousUntil.After(time.Now().Add(60*time.Second)) &&
				owner.source.currentLocked() != nil && owner.source.currentLocked() != oldSource && owner.source.set == retainedSet
			owner.mu.Unlock()
			if !valid || first.recipient.Public(time.Now()) == [32]byte{} || second.recipient.Public(time.Now()) == [32]byte{} {
				t.Fatal("refresh lost bounded predecessor, source selection, or independent keys")
			}
			owner.mu.Lock()
			cutoff := owner.publication.pair.previousUntil
			acknowledgedAt := second.publishedAt
			if acknowledgedAt.Before(switchEarliest) || acknowledgedAt.After(time.Now()) || cutoff.After(acknowledgedAt.Add(60*time.Second)) {
				owner.mu.Unlock()
				t.Fatal("publication ACK timestamp or predecessor bound is invalid")
			}
			owner.mu.Unlock()
			if first.request.Expiry.After(switchEarliest.Add(60*time.Second)) && cutoff.Before(switchEarliest.Add(60*time.Second)) {
				t.Fatal("predecessor overlap started before Descriptor ACK")
			}
			if _, err := owner.publishDescriptor(t.Context()); err != nil {
				t.Fatal(err)
			}
			owner.mu.Lock()
			sameCutoff := owner.publication.pair.previousUntil == cutoff && second.publishedAt == acknowledgedAt
			owner.mu.Unlock()
			if !sameCutoff {
				t.Fatal("exact Descriptor retry extended predecessor cutoff")
			}
			raw := lookupPublishedProof(t, owner, link.Target)
			current, err := reachability.VerifyPrivate(raw, link.Target, endpoint.network, source.view.Profile.Digest, time.Now().UTC())
			if err != nil || current.Current.Digest != published.Current.Digest || current.Descriptor.Private.Revision != 2 ||
				current.Descriptor.Private.Slot == published.Descriptor.Private.Slot || current.Descriptor.Private.RecipientKey == published.Descriptor.Private.RecipientKey {
				t.Fatalf("replacement changed authority or reused recipient: %v", err)
			}
			newAttempt, err := reader.prepareIntroduction(t.Context(), readerJob, link, bounds)
			if err != nil {
				t.Fatal(err)
			}
			deliverRefreshAttempt(t, reader, owner, readerJob, publisherJob, newAttempt)
			// Advance only retirement scheduling, not any signature or authority clock.
			owner.mu.Lock()
			owner.publication.pair.previousUntil = time.Now().Add(-time.Second)
			owner.publication.signalRegistrationsLocked()
			owner.mu.Unlock()
			waitRefreshCondition(t, owner, func() bool { return owner.publication.pair.previousRegistration == nil })
			select {
			case <-first.channel.Done():
			default:
				t.Fatal("retired predecessor channel remained live")
			}
			if first.recipient.Public(time.Now()) != [32]byte{} || second.recipient.Public(time.Now()) == [32]byte{} {
				t.Fatal("retirement did not independently erase the predecessor")
			}
			// An established Connection can outlive the 60-second predecessor
			// overlap. Recovery must resolve and seal to the acknowledged current
			// recipient without changing its immutable logical authority.
			clientRecovery := oldAttempt.binding.serviceRecovery()
			clientRecovery.Generation, clientRecovery.Role = 2, "client"
			clientRecovery.Deadline = time.Now().UTC().Add(8 * time.Second).Truncate(time.Second)
			publisherRecovery := oldAccepted.binding.serviceRecovery()
			publisherRecovery.Generation, publisherRecovery.Role = 2, "publisher"
			publisherRecovery.Deadline = clientRecovery.Deadline
			recovery, err := reader.prepareRecovery(t.Context(), readerJob, oldAttempt.binding, clientRecovery)
			if err != nil {
				t.Fatal(err)
			}
			defer clear(recovery.operation)
			type recoveryResult struct {
				attempt *introductionAttempt
				err     error
			}
			receivedRecovery := make(chan recoveryResult, 1)
			go func() {
				attempt, receiveErr := owner.receiveRecovery(t.Context(), publisherJob, oldAccepted.binding, publisherRecovery)
				receivedRecovery <- recoveryResult{attempt: attempt, err: receiveErr}
			}()
			if err := reader.submitIntroduction(t.Context(), readerJob, recovery); err != nil {
				t.Fatal(err)
			}
			acceptedRecovery := <-receivedRecovery
			if acceptedRecovery.err != nil || recovery.plaintext.Revision != second.request.Revision ||
				acceptedRecovery.attempt == nil || acceptedRecovery.attempt.plaintext.Revision != second.request.Revision {
				t.Fatalf("recovery did not advance to current Introduction recipient: client revision=%d Publisher=%v: %v",
					recovery.plaintext.Revision, acceptedRecovery.attempt, acceptedRecovery.err)
			}
			if err := owner.withdrawIntroduction(t.Context()); err != nil {
				t.Fatal(err)
			}
			select {
			case <-refresh.Done:
			default:
				t.Fatal("withdrawal did not join refresh")
			}
			owner.mu.Lock()
			for range 16 {
				owner.startRefreshLocked(second)
				owner.publication.signalRegistrationsLocked()
			}
			stopped := owner.publication.refresh.Current() == refresh && refresh.Context.Err() != nil && !owner.publication.pair.openingInProgressLocked() && owner.resolution == nil &&
				owner.publication.pair.registration == nil && owner.publication.pair.pendingRegistration == nil
			owner.mu.Unlock()
			if !stopped {
				t.Fatal("wake after Stop attempted another publication refresh")
			}
			if second.recipient.Public(time.Now()) != [32]byte{} {
				t.Fatal("withdrawal retained current key")
			}
		})
	}
}

// A permission is current only for its original hour and there is no automatic
// renewal route. A scheduled refresh that reaches that boundary must fail
// closed: it cannot retain an accepting registration or revive it on retry.
func TestTextPublicationRefreshExpiresPermissionWithoutResurrection(t *testing.T) {
	for _, carrier := range []routecarrier.CarrierProfile{routecarrier.ClosedCarrierTCP, routecarrier.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			gate := newDescriptorACKGate()
			defer gate.open()
			endpoint, owner, _, first := startRegisteredPublisherNetwork(t, carrier, gate)
			if _, err := owner.publishDescriptor(t.Context()); err != nil {
				t.Fatal(err)
			}
			owner.mu.Lock()
			refresh := owner.publication.refresh.Current()
			expires := owner.tokens.Permission.Accepted.NotAfter
			first.refreshAt = time.Now().Add(-time.Second)
			if refresh == nil || expires.IsZero() {
				owner.mu.Unlock()
				t.Fatal("published registration did not retain its refresh and permission expiry")
			}
			endpoint.clock = func() time.Time { return expires }
			owner.publication.signalRegistrationsLocked()
			owner.mu.Unlock()
			select {
			case <-refresh.Done:
			case <-time.After(10 * time.Second):
				t.Fatal("expired permission did not finish scheduled refresh")
			}
			owner.mu.Lock()
			retired := owner.publication.pair.registration == nil && owner.publication.pair.previousRegistration == nil && owner.publication.refresh.Outcome(refresh) != nil
			owner.mu.Unlock()
			if !retired {
				t.Fatal("expired permission retained accepting refresh readiness")
			}
			if first.recipient.Public(time.Now()) != [32]byte{} {
				t.Fatal("expired permission retained the current recipient key")
			}
			if _, err := owner.publishDescriptor(t.Context()); err == nil {
				t.Fatal("expired permission revived publication through exact retry")
			}
		})
	}
}

func waitRefreshCondition(t *testing.T, owner *dutyContext, condition func() bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		owner.mu.Lock()
		ok := condition()
		var err error
		refresh := owner.publication.refresh.Current()
		err = owner.publication.refresh.Outcome(refresh)
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

func deliverRefreshAttempt(t *testing.T, reader, publisher *dutyContext, readerJob, publisherJob *jobIdentity, prepared *introductionAttempt) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()
	result := make(chan error, 1)
	go func() {
		accepted, err := publisher.receiveIntroduction(ctx, publisherJob)
		if err == nil && (accepted.digest != prepared.digest || accepted.plaintext != prepared.plaintext) {
			err = context.Canceled
		}
		result <- err
	}()
	sent := reader.submitIntroduction(ctx, readerJob, prepared)
	if sent != nil {
		cancel()
	}
	received := <-result
	if sent != nil || received != nil {
		t.Fatalf("real refreshed delivery: submit=%v receive=%v", sent, received)
	}
}

func TestTextRefreshRetainsOriginalCleanupFailure(t *testing.T) {
	endpoint, principal := dutyContextEndpoint(t)
	owner := admittedDutyContext(t, endpoint, principal, broker.Administration)
	ctx, cancel := context.WithCancel(owner.lease.Context())
	defer cancel()
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseRefresh := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(releaseRefresh)
	flight := owner.publication.refresh.Start(ctx, func(*publication.Refresh) { <-release })
	owner.mu.Lock()
	reported := make(chan string, 1)
	owner.publication.refreshFailure = func(failure string) { reported <- failure }
	owner.mu.Unlock()
	failed := errors.New("predecessor cleanup did not join")
	owner.failRefresh(flight, "predecessor-retirement", errors.Join(client.ErrClosedSourceCleanup, failed))
	select {
	case failure := <-reported:
		if failure != "predecessor-retirement" {
			t.Fatalf("refresh failure category = %q", failure)
		}
	case <-time.After(time.Second):
		t.Fatal("refresh failure did not report its fixed category")
	}
	releaseRefresh()
	if err := owner.publication.refresh.Join(flight); !errors.Is(err, failed) {
		t.Fatalf("refresh lifecycle lost cleanup failure: %v", err)
	}
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

func TestTextRefreshFailureStagePreservesUnderlyingCause(t *testing.T) {
	cause := errors.New("source preparation failed")
	failure := publication.RefreshFailureAt("rotation-source", cause)
	if publication.RefreshFailureStage(failure) != "rotation-source" || !errors.Is(failure, cause) {
		t.Fatalf("refresh stage did not retain classification and cause: %v", failure)
	}
	if publication.RefreshFailureStage(cause) != "rotation" {
		t.Fatal("uncategorized refresh failure received a fabricated stage")
	}
}
