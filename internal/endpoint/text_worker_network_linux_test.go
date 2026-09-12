//go:build linux

package endpoint

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func exchangeTextWorkersThroughNetwork(t *testing.T, carrier route.CarrierProfile, body []byte, launch func(*testing.T, *textContext, []byte) *qualifiedTextWorker) {
	t.Helper()
	readerOwner, publisherOwner := textUnpublishedNetworkFixture(t, carrier)
	reader := launch(t, readerOwner, nil)
	publisher := launch(t, publisherOwner, body)
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	startup, endStartup := context.WithCancel(ctx)
	run, err := publisher.startPublication(startup)
	endStartup()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = run.Close() })
	until := time.Now().UTC().Add(2 * time.Minute).Unix()
	actual, readErr := readTextWorkerResultFixture(t, ctx, reader, run.link, [3]int64{until, until, until})
	if readErr != nil {
		t.Fatalf("worker network read failed: %v; Publisher cleanup: %v", readErr, run.Close())
	}
	withdrawalErr := run.Withdraw(ctx)
	if withdrawalErr != nil || !bytes.Equal(actual, body) {
		t.Fatalf("worker network document length %d (wanted %d): withdrawal %v", len(actual), len(body), withdrawalErr)
	}
	if !reader.completedCurrent() {
		t.Fatal("result escaped before reader retirement/currentness")
	}
	// Withdraw joins the Publisher worker and its complete owning context. The
	// standalone worker currentness check intentionally cannot revive that owner.
	select {
	case <-publisherOwner.done:
	default:
		t.Fatal("withdrawal returned before Publisher context retirement")
	}
	if err := publisher.Close(); err != nil {
		t.Fatal(err)
	}
}

// Only installed launch/cgroup observation is replaced by the explicit worker
// fixture. Both worker protocols, Grants, network legs and Service auth are real.
func TestTextWorkersReadTargetThroughJoinedNetwork(t *testing.T) {
	for _, carrier := range []route.CarrierProfile{route.ClosedCarrierTCP, route.ClosedCarrierQUIC} {
		t.Run(string(carrier), func(t *testing.T) {
			exchangeTextWorkersThroughNetwork(t, carrier, bytes.Repeat([]byte("x"), 64<<10), func(t *testing.T, owner *textContext, snapshot []byte) *qualifiedTextWorker {
				job := liveTextCapsuleJob(t, owner)
				return textServiceWorkerFixture(t, &textServiceBinding{owner: owner, job: job}, snapshot)
			})
		})
	}
}
