//go:build linux

package endpoint

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// Only State/Instance acceptance and installed-worker observation are fixtures.
// The startup owner performs registration and publication before returning its
// Link; the caller supplies no publication or registration success callback.
func TestTextPublisherStartupOwnsPublicationBeyondCaller(t *testing.T) {
	for _, carrier := range []route.CarrierProfile{route.ClosedCarrierTCP, route.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			readerOwner, publisherOwner := textUnpublishedNetworkFixture(t, carrier)
			body := []byte("publication started by its qualified worker")
			job := liveTextCapsuleJob(t, publisherOwner)
			worker := textServiceWorkerFixture(t, &textServiceBinding{owner: publisherOwner, job: job}, body)
			startup, endStartup := context.WithTimeout(t.Context(), 30*time.Second)
			defer endStartup()
			run, err := worker.startPublication(startup)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = run.Close() })
			endStartup()
			publisherOwner.mu.Lock()
			published := publisherOwner.registration != nil && publisherOwner.registration.published
			publisherOwner.mu.Unlock()
			if !published {
				t.Fatal("startup returned before Descriptor acknowledgement")
			}
			readerJob := liveTextCapsuleJob(t, readerOwner)
			reader := textServiceWorkerFixture(t, &textServiceBinding{owner: readerOwner, job: readerJob}, nil)
			now := time.Now().UTC()
			deadline := now.Add(time.Minute).Unix()
			result, err := reader.readTarget(t.Context(), run.link, [3]int64{deadline, deadline, deadline})
			if err != nil || !bytes.Equal(result, body) {
				t.Fatalf("started publication read failed: %v", err)
			}
			select {
			case <-run.done:
				t.Fatalf("startup caller cancellation retired publication: %v", run.err)
			default:
			}
			if err := run.Withdraw(t.Context()); err != nil {
				t.Fatalf("idle publication withdrawal failed: %v", err)
			}
		})
	}
}
