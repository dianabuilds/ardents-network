//go:build linux

package endpoint

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// Only installed worker observation and accepted State are fixtures. The
// Publisher's production producer owns delivery, JOIN and authenticated stream
// handover; the test does not construct or feed Publisher Service streams.
func TestTextPublisherNetworkRetainsSnapshotAcrossReaders(t *testing.T) {
	for _, carrier := range []route.CarrierProfile{route.ClosedCarrierTCP, route.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			readerOwner, publisherOwner, destination := textJoinedNetworkFixture(t, carrier)
			body := bytes.Repeat([]byte("retained snapshot\n"), 4096)
			publisherJob := liveTextCapsuleJob(t, publisherOwner)
			publisher := textServiceWorkerFixture(t, &textServiceBinding{owner: publisherOwner, job: publisherJob}, body)
			ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
			defer cancel()
			served := make(chan struct{})
			var serveErr error
			go func() { defer close(served); serveErr = publisher.serveNetwork(ctx) }()
			t.Cleanup(func() {
				cancel()
				select {
				case <-served:
				case <-time.After(10 * time.Second):
					t.Error("Publisher producer did not join during test cleanup")
				}
			})
			// A syntactically valid capsule with broken authentication must be
			// refused without terminating this snapshot's receive loop.
			for _, wrongTarget := range []bool{false, true} {
				refusedJob := liveTextCapsuleJob(t, readerOwner)
				refusedWorker := textServiceWorkerFixture(t, &textServiceBinding{owner: readerOwner, job: refusedJob}, nil)
				refusalUntil := time.Now().UTC().Add(2 * time.Minute).Unix()
				refused, err := readerOwner.prepareTextIntroduction(ctx, refusedJob, destination, [3]int64{refusalUntil, refusalUntil, refusalUntil})
				if err != nil {
					t.Fatal(err)
				}
				publisherOwner.mu.Lock()
				beforeRefusal := publisherOwner.introductionOpenings
				publisherOwner.mu.Unlock()
				request, capsule, err := route.DecodeClosedIntroductionSubmission(refused.operation)
				if err != nil {
					t.Fatal(err)
				}
				if wrongTarget {
					publisherOwner.mu.Lock()
					recipient := publisherOwner.registration.recipient.Public(time.Now().UTC())
					publisherOwner.mu.Unlock()
					facts := refused.plaintext
					facts.Target = fixtureID(190)
					clear(capsule.Ciphertext)
					capsule.Ciphertext = nil
					capsule.Encapsulation = [32]byte{}
					capsule, _, err = route.SealClosedIntroduction(capsule, recipient, facts)
					if err != nil {
						t.Fatal(err)
					}
				} else {
					capsule.Ciphertext[len(capsule.Ciphertext)-1] ^= 1
				}
				refused.operation, err = route.EncodeClosedIntroductionSubmission(request, capsule)
				clear(capsule.Ciphertext)
				if err != nil {
					t.Fatal(err)
				}
				if _, decoded, err := route.DecodeClosedIntroductionSubmission(refused.operation); err != nil {
					t.Fatal(err)
				} else {
					clear(decoded.Ciphertext)
				}
				if err := readerOwner.submitTextIntroduction(ctx, refusedJob, refused); err == nil {
					t.Fatal("corrupt capsule was accepted")
				}
				publisherOwner.mu.Lock()
				receivedRefusal := publisherOwner.introductionOpenings != beforeRefusal
				publisherOwner.mu.Unlock()
				if !receivedRefusal {
					t.Fatal("corrupt capsule did not reach Publisher opening boundary")
				}
				if err := refusedWorker.Close(); err != nil {
					t.Fatal(err)
				}
			}
			for range 2 {
				readerJob := liveTextCapsuleJob(t, readerOwner)
				reader := textServiceWorkerFixture(t, &textServiceBinding{owner: readerOwner, job: readerJob}, nil)
				until := time.Now().UTC().Add(2 * time.Minute).Unix()
				actual, err := reader.readTarget(ctx, destination, [3]int64{until, until, until})
				if err != nil || !bytes.Equal(actual, body) {
					cancel()
					select {
					case <-served:
					case <-time.After(10 * time.Second):
						t.Fatal("Publisher did not join after read failure")
					}
					t.Fatalf("network read length %d wanted %d: %v; Publisher: %v", len(actual), len(body), err, serveErr)
				}
				if !reader.completedCurrent() {
					t.Fatal("reader returned before retirement")
				}
				select {
				case <-served:
					t.Fatalf("Publisher ended after an individual read: %v", serveErr)
				default:
				}
				publisherOwner.mu.Lock()
				retained := publisherOwner.job == publisherJob && !publisherJob.retired
				publisherOwner.mu.Unlock()
				if !retained {
					t.Fatal("individual read replaced or retired Publisher snapshot job")
				}
			}
			cancel()
			select {
			case <-served:
				if !errors.Is(serveErr, context.Canceled) {
					t.Fatalf("canceled Publisher outcome: %v", serveErr)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("Publisher did not join canceled producer and worker")
			}
			if !publisher.completedCurrent() {
				t.Fatal("Publisher returned before joined worker retirement")
			}
			publisherOwner.mu.Lock()
			pending := len(publisherOwner.introductionExchanges)
			publisherOwner.mu.Unlock()
			if pending != 0 {
				t.Fatalf("Publisher retained %d exchanges after cancellation", pending)
			}
		})
	}
}
