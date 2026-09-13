//go:build linux

package endpoint

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// A client can observe the failed Carrier before the Publisher's native
// Connection does. The production initial receiver must retain that already
// authenticated next-generation capsule for the live logical Connection,
// rather than refusing it before the Publisher starts its recovery opener.
func TestTextRecoveryDeliveryMayArriveBeforePublisherFailureDetection(t *testing.T) {
	reader, publisher, destination := textJoinedNetworkFixture(t, route.ClosedCarrierTCP)
	readerJob, publisherJob := liveTextCapsuleJob(t, reader), liveTextCapsuleJob(t, publisher)
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()
	now := time.Now().UTC()
	bounds := [3]int64{now.Add(7 * time.Second).Unix(), now.Add(7 * time.Second).Unix(), now.Add(7 * time.Second).Unix()}
	initial, err := reader.prepareTextIntroduction(ctx, readerJob, destination, bounds)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(initial.operation)
	type received struct {
		attempt *textIntroductionAttempt
		err     error
	}
	accepted := make(chan received, 1)
	go func() {
		attempt, receiveErr := publisher.receiveTextIntroduction(ctx, publisherJob)
		accepted <- received{attempt: attempt, err: receiveErr}
	}()
	if err := reader.submitTextIntroduction(ctx, readerJob, initial); err != nil {
		t.Fatal(err)
	}
	remote := <-accepted
	if remote.err != nil {
		t.Fatal(remote.err)
	}

	clientRequest := initial.binding.textServiceRecovery()
	clientRequest.Generation, clientRequest.Deadline, clientRequest.Role = 2, now.Add(6*time.Second), "client"
	publisherRequest := remote.attempt.binding.textServiceRecovery()
	publisherRequest.Generation, publisherRequest.Deadline, publisherRequest.Role = 2, clientRequest.Deadline, "publisher"
	early, err := reader.prepareTextRecovery(ctx, readerJob, initial.binding, clientRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(early.operation)
	stopInitial := holdTextInitialIntroductionReceiver(t, ctx, publisher, publisherJob)

	publisher.mu.Lock()
	before := publisher.introductionOpenings[3]
	publisher.mu.Unlock()
	submitted := make(chan error, 1)
	go func() { submitted <- reader.submitTextIntroduction(ctx, readerJob, early) }()
	waitTextIntroductionOpening(t, ctx, publisher, before)
	select {
	case err := <-submitted:
		t.Fatalf("early recovery delivery was completed before its live owner waited: %v", err)
	default:
	}

	recovered, err := publisher.receiveTextRecovery(ctx, publisherJob, remote.attempt.binding, publisherRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(recovered.operation)
	if recovered.plaintext.AttachmentGeneration != 2 || recovered.plaintext.ConnectionNonce != initial.plaintext.ConnectionNonce {
		t.Fatalf("early recovery identity = generation %d connection %x", recovered.plaintext.AttachmentGeneration,
			recovered.plaintext.ConnectionNonce)
	}
	if err := <-submitted; err != nil {
		t.Fatal(err)
	}

	// A speculative later capsule is still owned: its deadline must refuse it
	// while the registration can answer, rather than leaving one of the sixteen
	// claimed deliveries stranded when no local recovery waiter appears.
	nextRequest := initial.binding.textServiceRecovery()
	nextRequest.Generation, nextRequest.Deadline, nextRequest.Role = 3, time.Now().UTC().Add(2*time.Second), "client"
	next, err := reader.prepareTextRecovery(ctx, readerJob, initial.binding, nextRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(next.operation)
	publisher.mu.Lock()
	before = publisher.introductionOpenings[3]
	publisher.mu.Unlock()
	nextSubmitted := make(chan error, 1)
	go func() { nextSubmitted <- reader.submitTextIntroduction(ctx, readerJob, next) }()
	waitTextIntroductionOpening(t, ctx, publisher, before)
	if err := <-nextSubmitted; err == nil {
		t.Fatal("expired buffered recovery delivery was accepted")
	}
	publisher.mu.Lock()
	buffered := len(remote.attempt.binding.recovery.delivery)
	publisher.mu.Unlock()
	if buffered != 0 {
		t.Fatalf("expired recovery owner retained %d deliveries", buffered)
	}
	publisher.mu.Lock()
	registrationDone := publisher.registration.channel.Done()
	publisher.mu.Unlock()
	select {
	case <-registrationDone:
		t.Fatal("expired recovery delivery ended the shared Publisher registration")
	default:
	}
	if err := remote.attempt.binding.releaseTextIntroductionRecovery(); err != nil {
		t.Fatal(err)
	}
	publisher.mu.Lock()
	retained := len(publisher.introductionRecovery)
	publisher.mu.Unlock()
	if retained != 0 {
		t.Fatalf("retired logical Connection retained %d recovery owners", retained)
	}
	stopInitial()
}

// Once the shared registration claims a capsule, refusal belongs to that
// registration rather than to the individual waiter whose cancellation may
// have raced with inspection.
func TestTextIntroductionOrphanRefusalOutlivesCanceledWaiter(t *testing.T) {
	reader, publisher, destination := textJoinedNetworkFixture(t, route.ClosedCarrierTCP)
	readerJob, publisherJob := liveTextCapsuleJob(t, reader), liveTextCapsuleJob(t, publisher)
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()
	now := time.Now().UTC()
	bounds := [3]int64{now.Add(7 * time.Second).Unix(), now.Add(7 * time.Second).Unix(), now.Add(7 * time.Second).Unix()}
	attempt, err := reader.prepareTextIntroduction(ctx, readerJob, destination, bounds)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(attempt.operation)
	request, capsule, err := route.DecodeClosedIntroductionSubmission(attempt.operation)
	if err != nil {
		t.Fatal(err)
	}
	publisher.mu.Lock()
	recipient := publisher.registration.recipient.Public(time.Now().UTC())
	publisher.mu.Unlock()
	facts := attempt.plaintext
	facts.AttachmentGeneration = 9
	clear(capsule.Ciphertext)
	capsule.Ciphertext = nil
	capsule.Encapsulation = [32]byte{}
	capsule, _, err = route.SealClosedIntroduction(capsule, recipient, facts)
	if err != nil {
		t.Fatal(err)
	}
	attempt.operation, err = route.EncodeClosedIntroductionSubmission(request, capsule)
	clear(capsule.Ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	submitted := make(chan error, 1)
	go func() { submitted <- reader.submitTextIntroduction(ctx, readerJob, attempt) }()
	delivery, err := publisher.nextTextIntroductionDelivery(ctx)
	if err != nil {
		t.Fatal(err)
	}
	key, expires, err := publisher.inspectTextIntroductionDelivery(ctx, publisherJob, delivery)
	if err != nil {
		t.Fatal(err)
	}
	if key.generation != 9 {
		t.Fatalf("orphan generation = %d", key.generation)
	}
	waiter, stopWaiter := context.WithCancel(ctx)
	stopWaiter()
	if waiter.Err() == nil {
		t.Fatal("waiter cancellation precondition missing")
	}
	if err := delivery.Complete(waiter, 1); err == nil {
		t.Fatal("canceled waiter unexpectedly completed the orphan delivery")
	}
	if err := publisher.refuseTextIntroductionDelivery(delivery, expires); err != nil {
		t.Fatal(err)
	}
	if err := <-submitted; err == nil {
		t.Fatal("orphan delivery was accepted")
	}
	publisher.mu.Lock()
	registrationDone := publisher.registration.channel.Done()
	publisher.mu.Unlock()
	select {
	case <-registrationDone:
		t.Fatal("canceled waiter ended the shared Publisher registration")
	default:
	}
}

func waitTextIntroductionOpening(t *testing.T, ctx context.Context, owner *textContext, before time.Time) {
	t.Helper()
	for {
		owner.mu.Lock()
		opened := owner.introductionOpenings[3]
		owner.mu.Unlock()
		if opened.After(before) {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("Publisher did not inspect the early recovery delivery: %v", ctx.Err())
		default:
			runtime.Gosched()
		}
	}
}
