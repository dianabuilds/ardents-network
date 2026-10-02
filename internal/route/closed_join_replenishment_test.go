//go:build linux

package route

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/ardp"
)

// Pairing, admission and time use the existing real-owner fixture. Finished
// stream markers model activated I/O only; these tests are not network evidence.
func replenishableJoinFixture(t *testing.T) (*closedJoinFixture, *ClosedJoinSide, *ClosedJoinSide) {
	t.Helper()
	f := newClosedJoinFixture(t)
	first, err := f.join(t, 1, 21, 10)
	if err != nil {
		t.Fatal(err)
	}
	second, err := f.join(t, 2, 21, 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, side := range []*ClosedJoinSide{first, second} {
		if _, err := side.Result(); err != nil {
			t.Fatal(err)
		}
		if err := side.ConfirmResult(); err != nil {
			t.Fatal(err)
		}
		finished := make(chan struct{})
		close(finished)
		side.stream = &closedJoinStream{finished: finished}
	}
	return f, first, second
}

func joinRefillFrame() ardp.Frame {
	return ardp.Frame{Kind: ardp.KindAdmit, Body: append([]byte{2}, bytes.Repeat([]byte{91}, 354)...)}
}

func TestClosedJoinReplenishmentKeepsOriginalAuthorityAndIndependentReserve(t *testing.T) {
	f, side, peer := replenishableJoinFixture(t)
	originalHello, originalExporter := side.hello, side.exporter
	originalDeadline, originalWall := side.deadline, side.wallDeadline
	peerLimit, peerUsed := peer.byteLimit, peer.used
	side.used = side.byteLimit - 4096
	var releases atomic.Int32
	var verifications int
	f.pairs.replenish = func(input ClosedAdmissionVerification) (func() error, error) {
		verifications++
		if input.Hello != originalHello || input.Exporter != originalExporter ||
			input.Class != 2 || input.Deadline != originalDeadline || !bytes.Equal(input.Token, joinRefillFrame().Body[1:]) {
			t.Error("refill substituted original authority")
		}
		return func() error { releases.Add(1); return nil }, nil
	}
	// Pairing's ten-second deadline must not replace admitted data lifetime.
	f.clock.Store(f.now.Add(11 * time.Second).Unix())
	var encoded bytes.Buffer
	if err := ardp.WriteFrame(&encoded, joinRefillFrame()); err != nil {
		t.Fatal(err)
	}
	before := side.used
	frame, err := side.readFrame(&encoded)
	if err != nil {
		t.Fatal(err)
	}
	if side.used != before+ardp.HeaderSize+355 {
		t.Fatal("ADMIT was not charged to the old reserve")
	}
	if err := side.replenish(t.Context(), frame); err != nil {
		t.Fatal(err)
	}
	if side.byteLimit-side.used != 32<<20 || side.deadline != originalDeadline || side.wallDeadline != originalWall ||
		peer.byteLimit != peerLimit || peer.used != peerUsed || releases.Load() != 0 {
		t.Fatal("refill changed lifetime, counterpart, or released live hosting")
	}
	// A second successful refill resets the remainder; it does not add to it.
	if err := side.replenish(t.Context(), frame); err != nil {
		t.Fatal(err)
	}
	if side.byteLimit-side.used != 32<<20 || verifications != 2 {
		t.Fatal("reserve accumulated")
	}
	side.Close()
	side.Close()
	if releases.Load() != 2 {
		t.Fatalf("hosting returned %d times", releases.Load())
	}
}

func TestClosedJoinReplenishmentRejectsBeforeVerification(t *testing.T) {
	for _, condition := range []string{"unpaired", "unconfirmed", "expired", "aborted", "exhausted", "wrong-class", "child-lane", "wrong-kind"} {
		t.Run(condition, func(t *testing.T) {
			f, side, _ := replenishableJoinFixture(t)
			frame := joinRefillFrame()
			switch condition {
			case "unpaired":
				side.pair.paired = false
			case "unconfirmed":
				side.confirmed = false
			case "expired":
				f.clock.Store(side.deadline.Unix())
			case "aborted":
				side.Abort()
			case "exhausted":
				side.used = side.byteLimit + 1
			case "wrong-class":
				frame.Body[0] = 1
			case "child-lane":
				frame.Lane = 1
			case "wrong-kind":
				frame.Kind = ardp.KindOperation
			}
			f.pairs.replenish = func(ClosedAdmissionVerification) (func() error, error) {
				t.Error("invalid refill reached token spend")
				return nil, nil
			}
			before := side.byteLimit
			if side.replenish(t.Context(), frame) == nil {
				t.Fatal("invalid refill accepted")
			}
			if side.byteLimit != before {
				t.Fatal("refused refill changed allowance")
			}
		})
	}
}

func TestClosedJoinReplenishmentCannotBorrowForItsOwnADMIT(t *testing.T) {
	_, side, _ := replenishableJoinFixture(t)
	side.used = side.byteLimit - ardp.HeaderSize - 354
	var encoded bytes.Buffer
	if err := ardp.WriteFrame(&encoded, joinRefillFrame()); err != nil {
		t.Fatal(err)
	}
	if _, err := side.readFrame(&encoded); err == nil {
		t.Fatal("ADMIT borrowed its final byte from a future reserve")
	}
	if encoded.Len() != 355 {
		t.Fatal("unreserved body read")
	}
}

func TestClosedJoinReplenishmentDoesNotBlockPairCancellation(t *testing.T) {
	f, side, _ := replenishableJoinFixture(t)
	entered, resume := make(chan struct{}), make(chan struct{})
	var released atomic.Int32
	f.pairs.replenish = func(ClosedAdmissionVerification) (func() error, error) {
		close(entered)
		<-resume
		return func() error { released.Add(1); return nil }, nil
	}
	before := side.byteLimit
	outcome := make(chan error, 1)
	go func() { outcome <- side.replenish(t.Context(), joinRefillFrame()) }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		close(resume)
		t.Fatal("verification did not begin")
	}
	canceled := make(chan struct{})
	go func() { side.Abort(); close(canceled) }()
	select {
	case <-canceled:
	case <-time.After(time.Second):
		close(resume)
		<-outcome
		t.Fatal("durable verification blocked pairing cancellation")
	}
	close(resume)
	if err := <-outcome; err == nil {
		t.Fatal("canceled refill applied")
	}
	if side.byteLimit != before || released.Load() != 1 {
		t.Fatal("canceled verification retained reserve")
	}
}

func TestClosedJoinReplenishmentRetainsSpendFailureAndCleanupFailure(t *testing.T) {
	f, side, _ := replenishableJoinFixture(t)
	failedSpend := errors.New("durable spend refused")
	f.pairs.replenish = func(ClosedAdmissionVerification) (func() error, error) { return nil, failedSpend }
	before := side.byteLimit
	if err := side.replenish(t.Context(), joinRefillFrame()); !errors.Is(err, failedSpend) || side.byteLimit != before {
		t.Fatal("failed spend changed allowance or lost cause")
	}
	failedCleanup := errors.New("hosting release failed")
	f.pairs.replenish = func(ClosedAdmissionVerification) (func() error, error) {
		return func() error { return failedCleanup }, nil
	}
	if err := side.replenish(t.Context(), joinRefillFrame()); err != nil {
		t.Fatal(err)
	}
	side.Close()
	if !errors.Is(side.cleanupErr, failedCleanup) {
		t.Fatal("cleanup failure erased")
	}
}

// The side stays live here: operation cancellation itself must bar commit,
// independently of when the Serve cancellation worker marks the pair stopped.
func TestClosedJoinRefillOperationCancellationBarsCommit(t *testing.T) {
	f, side, _ := replenishableJoinFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	entered, resume := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(resume) }) }
	var reserved, released atomic.Int32
	f.pairs.replenish = func(ClosedAdmissionVerification) (func() error, error) {
		reserved.Add(1)
		close(entered)
		<-resume
		return func() error { released.Add(1); return nil }, nil
	}
	before := side.byteLimit
	finished := make(chan struct{})
	var outcome error
	collect := func() bool {
		select {
		case <-finished:
			return true
		case <-time.After(time.Second):
			t.Error("cancelled refill worker did not join")
			return false
		}
	}
	// Register before launch: even readiness failure must release the policy
	// gate, cancel and join the worker before the fixture closes its owners.
	t.Cleanup(func() {
		cancel()
		unblock()
		if !collect() {
			return
		}
		if !errors.Is(outcome, context.Canceled) {
			t.Errorf("refill terminal cancellation lost: %v", outcome)
		}
		if side.byteLimit != before || released.Load() != reserved.Load() {
			t.Error("cancelled refill cleanup changed allowance or retained host reservation")
		}
		side.Close()
		if side.cleanupErr != nil {
			t.Errorf("refill side cleanup: %v", side.cleanupErr)
		}
	})
	go func() {
		outcome = side.replenish(ctx, joinRefillFrame())
		close(finished)
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("refill policy not reached")
	}
	cancel()
	unblock()
	if !collect() {
		t.FailNow()
	}
	if !errors.Is(outcome, context.Canceled) {
		t.Fatalf("operation cancellation lost: %v", outcome)
	}
	if side.byteLimit != before || released.Load() != 1 {
		t.Fatal("cancelled operation committed refill or retained host reservation")
	}
}
