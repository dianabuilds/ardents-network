package channel

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"io"
	"math"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

func TestParentReplenishmentPreservesConcurrentDebits(t *testing.T) {
	const allowance = uint64(33554432)
	local, peer := net.Pipe()
	defer peer.Close()
	s := Prepare(t.Context(), local, time.Now().Add(5*time.Second), 1400, nil, false, &Budget{maximum: 4 << 20}, Handlers{})
	defer s.Close()
	// Seed the cumulative witness after the original ADMIT. Subsequent traffic
	// and ACCEPT use the actual framing writer rather than changing counters.
	s.used = 1000
	witness := uint64(1000)
	child := lifecycleLane(t, s, 1)
	written := make(chan error, 1)
	go func() {
		written <- s.write(child, ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: make([]byte, 84)}, false)
	}()
	traffic := make([]byte, 100) // independent 16-byte header + 84-byte payload
	if _, err := io.ReadFull(peer, traffic); err != nil {
		t.Fatal(err)
	}
	if err := <-written; err != nil {
		t.Fatal(err)
	}
	go func() { written <- s.AcceptParent(t.Context(), witness, allowance) }()
	ack := make([]byte, 21)
	if _, err := io.ReadFull(peer, ack); err != nil {
		t.Fatal(err)
	}
	if err := <-written; err != nil {
		t.Fatal(err)
	}
	// Fixed independent generation-3 ACCEPT: lane zero, status zero, 64 KiB.
	expected, err := hex.DecodeString("415244500003050000000000000000050000010000")
	if err != nil || !bytes.Equal(ack, expected) {
		t.Fatalf("parent ACK grammar changed: %x (%v)", ack, err)
	}
	if s.used != 1121 || s.limit-s.used != allowance-121 {
		t.Fatalf("replacement erased traffic or missed ACK debit: used=%d remaining=%d", s.used, s.limit-s.used)
	}
}
func TestParentAdmitChargesCompleteFrameBeforeVerification(t *testing.T) {
	for _, remaining := range []uint64{370, 371, 372} {
		t.Run(strconv.FormatUint(remaining, 10), func(t *testing.T) {
			local, peer := net.Pipe()
			defer peer.Close()
			observed := make(chan uint64, 1)
			handlers := Handlers{ParentControl: func(_ context.Context, _ ardp.Frame, witness, left uint64) error {
				if witness != 371 {
					t.Error("incomplete ADMIT debit", witness)
				}
				observed <- left
				return errors.New("verification probe refuses; no authority granted")
			}}
			s := Prepare(t.Context(), local, time.Now().Add(time.Second), remaining, nil, false, &Budget{maximum: 64 << 20}, handlers)
			s.Start()
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
	for _, s := range []*Session{
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
			called := make(chan struct{}, 1)
			handlers := Handlers{ParentControl: func(context.Context, ardp.Frame, uint64, uint64) error {
				called <- struct{}{}
				return errors.New("probe refuses")
			}}
			s := Prepare(t.Context(), local, time.Now().Add(time.Second), 33554432, nil, false, &Budget{maximum: 64 << 20}, handlers)
			s.Start()
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
