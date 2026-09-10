//go:build linux

package endpoint

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	"github.com/dianabuilds/ardents-network/internal/route"
)

// The reader delays its document request until withdrawal has stopped new
// admissions. Its already authenticated Connection must still deliver the body.
func TestTextPublisherWithdrawalDrainsAdmittedRead(t *testing.T) {
	for _, carrier := range []route.CarrierProfile{route.ClosedCarrierTCP, route.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			readerOwner, publisherOwner := textUnpublishedNetworkFixture(t, carrier)
			body := []byte("an admitted read survives publication withdrawal")
			publisherJob := liveTextCapsuleJob(t, publisherOwner)
			publisher := textServiceWorkerFixture(t, &textServiceBinding{owner: publisherOwner, job: publisherJob}, body)
			run, err := publisher.startPublication(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = run.Close() })
			readerJob := liveTextCapsuleJob(t, readerOwner)
			reader := textServiceWorkerFixture(t, &textServiceBinding{owner: readerOwner, job: readerJob}, nil)
			bounded, finish, err := reader.beginOperation(t.Context(), broker.Connection)
			if err != nil {
				t.Fatal(err)
			}
			until := time.Now().UTC().Add(time.Minute).Unix()
			attempt, err := readerOwner.prepareTextIntroduction(bounded, readerJob, run.link, [3]int64{until, until, until})
			if err != nil {
				finish()
				t.Fatal(err)
			}
			stream, err := readerOwner.openTextJoinedService(bounded, readerJob, attempt)
			if err != nil {
				finish()
				t.Fatal(err)
			}
			withdrawn := make(chan error, 1)
			go func() { withdrawn <- run.Withdraw(t.Context()) }()
			select {
			case <-publisherOwner.publicationDrain:
			case <-time.After(3 * time.Second):
				finish()
				_ = stream.Close()
				t.Fatal("withdrawal did not stop admissions")
			}
			actual, readErr := reader.completeServiceRead(t.Context(), bounded, finish, stream, nil)
			if readErr != nil || !bytes.Equal(actual, body) {
				t.Fatalf("admitted read did not drain: %v; native: %v; cleanup: %v; withdrawal: %v", readErr, stream.runErr, stream.finishErr, <-withdrawn)
			}
			if err := <-withdrawn; err != nil {
				t.Fatalf("withdrawal failed after drain: %v", err)
			}
			if err := run.Withdraw(t.Context()); err == nil {
				t.Fatal("repeated withdrawal succeeded")
			}
		})
	}
}

func TestTextPublisherWithdrawalBoundsStalledRead(t *testing.T) {
	for _, carrier := range []route.CarrierProfile{route.ClosedCarrierTCP} {
		t.Run(string(carrier), func(t *testing.T) {
			readerOwner, publisherOwner := textUnpublishedNetworkFixture(t, carrier)
			body := []byte("an admitted read survives publication withdrawal")
			publisherJob := liveTextCapsuleJob(t, publisherOwner)
			publisher := textServiceWorkerFixture(t, &textServiceBinding{owner: publisherOwner, job: publisherJob}, body)
			run, err := publisher.startPublication(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = run.Close() })
			readerJob := liveTextCapsuleJob(t, readerOwner)
			reader := textServiceWorkerFixture(t, &textServiceBinding{owner: readerOwner, job: readerJob}, nil)
			bounded, finish, err := reader.beginOperation(t.Context(), broker.Connection)
			if err != nil {
				t.Fatal(err)
			}
			until := time.Now().UTC().Add(time.Minute).Unix()
			attempt, err := readerOwner.prepareTextIntroduction(bounded, readerJob, run.link, [3]int64{until, until, until})
			if err != nil {
				finish()
				t.Fatal(err)
			}
			stream, err := readerOwner.openTextJoinedService(bounded, readerJob, attempt)
			if err != nil {
				finish()
				t.Fatal(err)
			}
			withdrawn := make(chan error, 1)
			go func() { withdrawn <- run.Withdraw(t.Context()) }()
			select {
			case <-publisherOwner.publicationDrain:
			case <-time.After(3 * time.Second):
				finish()
				_ = stream.Close()
				t.Fatal("withdrawal did not stop admissions")
			}
			// No document request is sent. The original Connection lifetime is a
			// minute, so only the withdrawal bound can terminate this stalled read.
			started := time.Now()
			withdrawErr := <-withdrawn
			elapsed := time.Since(started)
			if !errors.Is(withdrawErr, context.DeadlineExceeded) {
				t.Errorf("stalled withdrawal lost its deadline failure: %v", withdrawErr)
			}
			if elapsed < 4*time.Second || elapsed > 8*time.Second {
				t.Errorf("stalled read did not use finite five-second drain: %v", elapsed)
			}
			_ = stream.Close()
			finish()
			_ = reader.Close()
		})
	}
}
