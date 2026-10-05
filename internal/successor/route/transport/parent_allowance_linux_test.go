//go:build linux

package transport

import (
	"context"
	"errors"
	"math"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

func TestParentReplenishmentPreservesConcurrentDebits(t *testing.T) {
	const allowance = uint64(33554432)
	s := &session{end: time.Now().Add(time.Minute), used: 1000, limit: 1400}
	// The verified ADMIT was charged before this witness. Another child then
	// consumes 100 bytes while verification/reservation/spend performs I/O.
	witness := s.used
	s.used += 100
	if err := s.replaceRemaining(witness, allowance); err != nil {
		t.Fatal(err)
	}
	if s.used != 1100 || s.limit-s.used != allowance-100 {
		t.Fatalf("refill erased racing traffic or added old reserve: used=%d remaining=%d", s.used, s.limit-s.used)
	}
	// Matching ACCEPT is a separate complete 16-byte header plus five-byte body.
	s.used += 21
	if s.limit-s.used != allowance-121 {
		t.Fatal("ACCEPT not independently charged")
	}
}

func TestParentAdmitChargesCompleteFrameBeforeVerification(t *testing.T) {
	for _, remaining := range []uint64{370, 371, 372} {
		t.Run(strconv.FormatUint(remaining, 10), func(t *testing.T) {
			local, peer := net.Pipe()
			defer peer.Close()
			s := prepareSession(t.Context(), local, time.Now().Add(time.Second), remaining, nil, false, &queueBudget{maximum: 64 << 20}, nil)
			observed := make(chan uint64, 1)
			s.parentControl = func(_ context.Context, _ ardp.Frame, witness, left uint64) error {
				if witness != 371 {
					t.Error("incomplete ADMIT debit", witness)
				}
				observed <- left
				return errors.New("verification probe refuses; no authority granted")
			}
			s.startReading()
			body := make([]byte, 355)
			body[0] = 2
			if err := ardp.WriteFrame(peer, ardp.Frame{Kind: ardp.KindAdmit, Body: body}); err != nil {
				t.Fatal(err)
			}
			<-s.readerDone
			_ = s.Close()
			select {
			case left := <-observed:
				if remaining != 372 || left != 1 {
					t.Fatal("exhausted parent reached verification", remaining, left)
				}
			default:
				if remaining == 372 {
					t.Fatal("positive original reserve did not reach verification")
				}
			}
		})
	}
}

func TestParentReplenishmentRefusesInvalidTransition(t *testing.T) {
	for _, s := range []*session{
		{end: time.Now().Add(time.Minute), used: 100, limit: 200, dedicated: true},
		{end: time.Now().Add(time.Minute), used: 100, limit: 200, stopped: true},
		{end: time.Now().Add(-time.Second), used: 100, limit: 200},
		{end: time.Now().Add(time.Minute), used: math.MaxUint64 - 5, limit: math.MaxUint64},
	} {
		beforeUsed, beforeLimit := s.used, s.limit
		if err := s.replaceRemaining(s.used, 33554432); err == nil {
			t.Fatal("invalid parent transition accepted")
		}
		if s.used != beforeUsed || s.limit != beforeLimit {
			t.Fatal("refusal changed accounting")
		}
	}
}

func TestParentRejectsWrongClassBeforeControlEffects(t *testing.T) {
	for _, class := range []byte{1, 3} {
		t.Run(strconv.Itoa(int(class)), func(t *testing.T) {
			local, peer := net.Pipe()
			defer peer.Close()
			s := prepareSession(t.Context(), local, time.Now().Add(time.Second), 33554432, nil, false, &queueBudget{maximum: 64 << 20}, nil)
			called := make(chan struct{}, 1)
			s.parentControl = func(context.Context, ardp.Frame, uint64, uint64) error {
				called <- struct{}{}
				return errors.New("probe refuses")
			}
			s.startReading()
			body := make([]byte, 355)
			body[0] = class
			if err := ardp.WriteFrame(peer, ardp.Frame{Kind: ardp.KindAdmit, Body: body}); err != nil {
				t.Fatal(err)
			}
			<-s.readerDone
			_ = s.Close()
			select {
			case <-called:
				t.Fatal("wrong class reached reservation/spend composition")
			default:
			}
		})
	}
}
