package connection

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
)

// This refused proposal fixture has no accepting Operation, Instance or Route.
// It isolates original callback join and ticket retirement, never Service I/O.
func TestPublisherRejectionJoinsCallbackPublishedDuringPreparation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		episode, started := recoveryFixture(t)
		ticket, err := episode.begin(1, started)
		if err != nil {
			t.Fatal(err)
		}
		child, cancel := context.WithCancel(context.Background())
		defer cancel()
		stream := &Stream{owner: &AuthenticatedAttachment{}, recovery: episode}
		proposal := &PublisherReplacement{stream: stream, ticket: ticket, ctx: child, cancel: cancel,
			callerDone: make(chan struct{}), interruptionDone: make(chan struct{}), prepared: make(chan struct{})}
		proposal.self = proposal
		stream.publisherProposal = proposal
		proposal.stopCaller = context.AfterFunc(context.Background(), func() { close(proposal.callerDone) })
		result := make(chan error, 1)
		go func() { result <- proposal.Close() }()
		synctest.Wait()
		// The actual preparation actor can publish its interruption after
		// Close has already sealed/cancelled the proposal, before it finishes.
		interruptionStarted, finishInterruption := make(chan struct{}), make(chan struct{})
		proposal.stopInterruption = context.AfterFunc(child, func() {
			defer close(proposal.interruptionDone)
			close(interruptionStarted)
			<-finishInterruption
		})
		close(proposal.prepared)
		synctest.Wait()
		select {
		case <-interruptionStarted:
		default:
			t.Fatal("cancelled preparation did not run its original interruption")
		}
		select {
		case err := <-result:
			t.Fatalf("rejection retired before its published callback joined: %v", err)
		default:
		}
		if ticket.consumed || episode.active != ticket {
			t.Fatal("unjoined callback released the original proposal")
		}
		close(finishInterruption)
		if err := <-result; !errors.Is(err, context.Canceled) {
			t.Fatalf("rejection lost original cancellation: %v", err)
		}
		if !ticket.consumed || episode.active != nil {
			t.Fatal("joined rejection did not retire its exact ticket")
		}
	})
}
