//go:build linux

package issuer

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/carrier"
)

func TestAdmittedIssuerRetainsHostingThroughOuterTermination(t *testing.T) {
	for _, transport := range []carrier.CarrierProfile{carrier.ClosedCarrierTCP, carrier.ClosedCarrierQUIC} {
		t.Run(string(transport), func(t *testing.T) {
			fixture := newTerminationFixture(t, transport, nil)
			connection := fixture.open(t, 1, fixture.controls[0])
			fixture.exchange(t, connection)
			select {
			case <-fixture.gate.entered:
			case <-time.After(3 * time.Second):
				t.Fatal("actual Outer CLOSE did not reach the physical writer")
			}
			if got := fixture.observe(t); got != 288<<10 || fixture.released.Load() != 0 {
				t.Fatalf("blocked terminal refunded Hosting: reserved=%d, releases=%d", got, fixture.released.Load())
			}
			fixture.gate.release()
			select {
			case <-fixture.releaseDone:
			case <-time.After(3 * time.Second):
				t.Fatal("joined termination did not release Hosting")
			}
			if got := fixture.observe(t); got != 0 || fixture.released.Load() != 1 {
				t.Fatalf("joined terminal reserve=%d, releases=%d", got, fixture.released.Load())
			}
			// A new child uses the same physical Carrier and a fresh genuine
			// Control token; terminal cleanup cannot damage its exact retry.
			sibling := fixture.open(t, 3, fixture.controls[1])
			fixture.exchange(t, sibling)
			select {
			case <-fixture.releaseDone:
			case <-time.After(3 * time.Second):
				t.Fatal("successful sibling did not release its own reservation")
			}
			if got := fixture.observe(t); got != 0 || fixture.released.Load() != 2 {
				t.Fatalf("sibling terminal reserve=%d, releases=%d", got, fixture.released.Load())
			}
		})
	}
}

func TestAdmittedIssuerFailedReplyAndTerminalTimeout(t *testing.T) {
	for _, transport := range []carrier.CarrierProfile{carrier.ClosedCarrierTCP, carrier.ClosedCarrierQUIC} {
		for _, ending := range []string{"reply-failure", "close-timeout"} {
			t.Run(string(transport)+"/"+ending, func(t *testing.T) {
				failure := errors.New("injected RESULT write failure")
				if ending == "close-timeout" {
					failure = nil
				}
				fixture := newTerminationFixture(t, transport, failure)
				fixture.expectCleanupFailure = true
				connection := fixture.open(t, 1, fixture.controls[0])
				fixture.gate.reply.Store(ending == "reply-failure")
				if ending == "reply-failure" {
					if err := ardp.WriteFrame(connection, ardp.Frame{Kind: ardp.KindOperation, Body: fixture.operation}); err != nil {
						t.Fatal(err)
					}
				} else {
					fixture.exchange(t, connection)
				}
				select {
				case <-fixture.gate.entered:
				case <-time.After(3 * time.Second):
					t.Fatal("terminal write did not reach physical gate")
				}
				if got := fixture.observe(t); got != 288<<10 || fixture.released.Load() != 0 {
					t.Fatalf("unjoined write refunded Hosting: %d / %d", got, fixture.released.Load())
				}
				if ending == "close-timeout" {
					// The production Outer CLOSE has a one-second write deadline.
					// Resume the actual transport after that bound has expired.
					<-time.After(1100 * time.Millisecond)
				}
				fixture.gate.release()
				if ending == "reply-failure" {
					if _, err := ardp.ReadFrame(connection); err == nil {
						t.Fatal("failed RESULT was reported successful")
					}
				}
				select {
				case <-fixture.releaseDone:
				case <-time.After(3 * time.Second):
					t.Fatal("failed writer did not join and release")
				}
				joined := fixture.server.drain(t.Context(), 3*time.Second)
				if ending == "close-timeout" {
					if !errors.Is(joined, os.ErrDeadlineExceeded) {
						t.Fatalf("terminal timeout lost from Drain: %v", joined)
					}
				} else if !errors.Is(joined, failure) {
					t.Fatalf("RESULT failure lost from Drain: %v", joined)
				}
				if got := fixture.observe(t); got != 0 || fixture.released.Load() != 1 {
					t.Fatalf("failed writer cleanup reserve=%d, releases=%d", got, fixture.released.Load())
				}
			})
		}
	}
}

func TestAdmittedIssuerLateDrainRetainsTerminalFailure(t *testing.T) {
	for _, transport := range []carrier.CarrierProfile{carrier.ClosedCarrierTCP, carrier.ClosedCarrierQUIC} {
		t.Run(string(transport), func(t *testing.T) {
			failure := errors.New("injected physical CLOSE failure")
			fixture := newTerminationFixture(t, transport, failure)
			connection := fixture.open(t, 1, fixture.controls[0])
			fixture.exchange(t, connection)
			select {
			case <-fixture.gate.entered:
			case <-time.After(3 * time.Second):
				t.Fatal("actual Outer CLOSE did not reach the physical writer")
			}
			if err := fixture.server.drain(t.Context(), 30*time.Millisecond); !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("Drain did not retain the blocked child: %v", err)
			}
			if got := fixture.observe(t); got != 288<<10 || fixture.released.Load() != 0 {
				t.Fatalf("timed-out Drain refunded Hosting: reserved=%d, releases=%d", got, fixture.released.Load())
			}
			fixture.gate.release()
			for range 2 {
				if err := fixture.server.drain(t.Context(), 3*time.Second); !errors.Is(err, failure) {
					t.Fatalf("joined Drain lost terminal failure: %v", err)
				}
			}
			if got := fixture.observe(t); got != 0 || fixture.released.Load() != 1 {
				t.Fatalf("repeated Drain reserve=%d, releases=%d", got, fixture.released.Load())
			}
		})
	}
}

func TestAdmittedIssuerCancelledOrRefusedOperationRetainsTermination(t *testing.T) {
	for _, transport := range []carrier.CarrierProfile{carrier.ClosedCarrierTCP, carrier.ClosedCarrierQUIC} {
		for _, ending := range []string{"client-cancel", "invalid-operation"} {
			t.Run(string(transport)+"/"+ending, func(t *testing.T) {
				fixture := newTerminationFixture(t, transport, nil)
				connection := fixture.open(t, 1, fixture.controls[0])
				if ending == "client-cancel" {
					if err := connection.(interface{ CloseWrite() error }).CloseWrite(); err != nil {
						t.Fatal(err)
					}
				} else if err := ardp.WriteFrame(connection, ardp.Frame{Kind: ardp.KindKeepalive}); err != nil {
					t.Fatal(err)
				}
				fixture.workers.Go(func() { _, _ = io.Copy(io.Discard, connection) })
				select {
				case <-fixture.gate.entered:
				case <-time.After(3 * time.Second):
					t.Fatal("cancelled/refused operation did not reach Outer termination")
				}
				if got := fixture.observe(t); got != 288<<10 || fixture.released.Load() != 0 {
					t.Fatalf("operation failure released before termination: %d / %d", got, fixture.released.Load())
				}
				fixture.gate.release()
				select {
				case <-fixture.releaseDone:
				case <-time.After(3 * time.Second):
					t.Fatal("operation failure did not finish cleanup")
				}
				if got := fixture.observe(t); got != 0 || fixture.released.Load() != 1 {
					t.Fatalf("operation failure cleanup: %d / %d", got, fixture.released.Load())
				}
			})
		}
	}
}

func TestAdmittedIssuerRetainsHostingUntilFailedPhysicalCloseJoins(t *testing.T) {
	for _, transport := range []carrier.CarrierProfile{carrier.ClosedCarrierTCP, carrier.ClosedCarrierQUIC} {
		t.Run(string(transport), func(t *testing.T) {
			writeFailure := errors.New("physical frame failure")
			closeFailure := errors.New("physical close failure")
			fixture := newTerminationFixture(t, transport, writeFailure)
			connection := fixture.open(t, 1, fixture.controls[0])
			fixture.gate.physicalFailure = closeFailure
			fixture.gate.physical.Store(true)
			fixture.exchange(t, connection)
			select {
			case <-fixture.gate.entered:
			case <-time.After(3 * time.Second):
				t.Fatal("terminal frame did not reach writer")
			}
			fixture.gate.release()
			select {
			case <-fixture.gate.physicalEntered:
			case <-time.After(3 * time.Second):
				t.Fatal("failed writer did not retire actual Carrier")
			}
			if got := fixture.observe(t); got != 288<<10 || fixture.released.Load() != 0 {
				t.Fatalf("unjoined physical close refunded Hosting: %d / %d", got, fixture.released.Load())
			}
			if err := fixture.server.drain(t.Context(), 30*time.Millisecond); !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("Drain did not wait for physical close: %v", err)
			}
			fixture.gate.releasePhysical()
			for range 2 {
				joined := fixture.server.drain(t.Context(), 3*time.Second)
				if !errors.Is(joined, writeFailure) || !errors.Is(joined, closeFailure) {
					t.Fatalf("joined Drain lost physical causes: %v", joined)
				}
			}
			if got := fixture.observe(t); got != 0 || fixture.released.Load() != 1 {
				t.Fatalf("joined physical close cleanup: %d / %d", got, fixture.released.Load())
			}
		})
	}
}
