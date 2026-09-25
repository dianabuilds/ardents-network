//go:build linux

package endpoint

import (
	"bytes"
	"context"
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	introductioncapsule "github.com/dianabuilds/ardents-network/internal/route/capsule"
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
			publisherOwner.mu.Lock()
			initialWaiters := len(publisherOwner.introductionDispatch.waiters)
			publisherOwner.mu.Unlock()
			go func() { defer close(served); serveErr = publisher.serveNetwork(ctx) }()
			t.Cleanup(func() {
				cancel()
				select {
				case <-served:
				case <-time.After(10 * time.Second):
					t.Error("Publisher producer did not join during test cleanup")
				}
			})
			waitTextIntroductionWaiters(t, ctx, publisherOwner, initialWaiters+1)
			// A syntactically valid capsule with broken authentication must be
			// refused without terminating this snapshot's receive loop.
			for _, fault := range []struct {
				wrongTarget, unknownGeneration bool
			}{{}, {wrongTarget: true}, {unknownGeneration: true}} {
				refusedJob := liveTextCapsuleJob(t, readerOwner)
				refusedWorker := textServiceWorkerFixture(t, &textServiceBinding{owner: readerOwner, job: refusedJob}, nil)
				refusalUntil := time.Now().UTC().Add(2 * time.Minute).Unix()
				refused, err := readerOwner.prepareTextIntroduction(ctx, refusedJob, destination, [3]int64{refusalUntil, refusalUntil, refusalUntil})
				if err != nil {
					t.Fatal(err)
				}
				publisherOwner.mu.Lock()
				beforeRefusal := publisherOwner.introductionAdmission.openings
				publisherOwner.mu.Unlock()
				request, capsule, err := introductioncapsule.DecodeSubmission(refused.operation)
				if err != nil {
					t.Fatal(err)
				}
				if fault.wrongTarget || fault.unknownGeneration {
					publisherOwner.mu.Lock()
					recipient := publisherOwner.registration.recipient.Public(time.Now().UTC())
					publisherOwner.mu.Unlock()
					facts := refused.plaintext
					if fault.wrongTarget {
						facts.Target = fixtureID(190)
					}
					if fault.unknownGeneration {
						facts.AttachmentGeneration = 9
					}
					clear(capsule.Ciphertext)
					capsule.Ciphertext = nil
					capsule.Encapsulation = [32]byte{}
					capsule, _, err = introductioncapsule.Seal(capsule, recipient, facts)
					if err != nil {
						t.Fatal(err)
					}
				} else {
					capsule.Ciphertext[len(capsule.Ciphertext)-1] ^= 1
				}
				refused.operation, err = introductioncapsule.EncodeSubmission(request, capsule)
				clear(capsule.Ciphertext)
				if err != nil {
					t.Fatal(err)
				}
				if _, decoded, err := introductioncapsule.DecodeSubmission(refused.operation); err != nil {
					t.Fatal(err)
				} else {
					clear(decoded.Ciphertext)
				}
				if err := readerOwner.submitTextIntroduction(ctx, refusedJob, refused); err == nil {
					t.Fatal("corrupt capsule was accepted")
				}
				publisherOwner.mu.Lock()
				receivedRefusal := publisherOwner.introductionAdmission.openings != beforeRefusal
				publisherOwner.mu.Unlock()
				if !receivedRefusal {
					t.Fatal("corrupt capsule did not reach Publisher opening boundary")
				}
				if err := refusedWorker.Close(); err != nil {
					t.Fatal(err)
				}
			}
			// The three adversarial openings above intentionally consume the
			// Publisher's real four-per-second cryptographic-opening allowance.
			// Start the two independent valid reads in the next rate window so a
			// faster CI runner cannot turn the fifth opening into the test oracle.
			publisherOwner.mu.Lock()
			resumeAt := publisherOwner.introductionAdmission.openings[3].Add(time.Second + 10*time.Millisecond)
			publisherOwner.mu.Unlock()
			if wait := time.Until(resumeAt); wait > 0 {
				timer := time.NewTimer(wait)
				select {
				case <-timer.C:
				case <-ctx.Done():
					if !timer.Stop() {
						select {
						case <-timer.C:
						default:
						}
					}
					t.Fatal(ctx.Err())
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
			pending := len(publisherOwner.introductionExchanges.active)
			publisherOwner.mu.Unlock()
			if pending != 0 {
				t.Fatalf("Publisher retained %d exchanges after cancellation", pending)
			}
		})
	}
}

func waitTextIntroductionWaiters(t *testing.T, ctx context.Context, owner *textContext, minimum int) {
	t.Helper()
	for {
		owner.mu.Lock()
		ready := len(owner.introductionDispatch.waiters) >= minimum
		owner.mu.Unlock()
		if ready {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("Publisher did not register %d Introduction waiters before the bound: %v", minimum, ctx.Err())
		default:
			runtime.Gosched()
		}
	}
}
