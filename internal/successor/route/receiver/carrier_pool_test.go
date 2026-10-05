package receiver

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	framing "github.com/dianabuilds/ardents-network/internal/successor/route/channel"
	"github.com/dianabuilds/ardents-network/internal/successor/route/role"
)

// These are pool-mechanism oracles, with injected validation and gated physical
// failure. Successful Network/Admission authority is covered by command tests.
func TestNodePoolOneOpeningAndIndependentWaiterCancellation(t *testing.T) {
	var p nodePool
	a := role.Authority{}
	a.Duty.NodeID = [32]byte{1}
	entered, gate := make(chan struct{}), make(chan struct{})
	var opened, released atomic.Int32
	physical := newLifecycleConn(false)
	open := func(context.Context) (*framing.Session, func(), error) {
		opened.Add(1)
		close(entered)
		<-gate
		return framing.New(context.Background(), physical, time.Now().Add(5*time.Second), 32<<20, nil, false, framing.NewBudget(64<<20), nil), func() { released.Add(1) }, nil
	}
	validate := func() error { return nil }
	firstResult := make(chan *pooledCarrier, 1)
	go func() {
		c, err := p.borrow(t.Context(), a, validate, open)
		if err != nil {
			t.Error(err)
		}
		firstResult <- c
	}()
	<-entered
	waiter, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := p.borrow(waiter, a, validate, open); !errors.Is(err, context.Canceled) {
		t.Fatal("waiter cancellation lost", err)
	}
	close(gate)
	first := <-firstResult
	second, err := p.borrow(t.Context(), a, validate, open)
	if err != nil || first != second || opened.Load() != 1 {
		t.Fatal("directed pair opened twice", err, opened.Load())
	}
	if err := first.release(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-physical.closed:
		t.Fatal("first borrower erased healthy sibling")
	default:
	}
	if released.Load() != 0 {
		t.Fatal("shared control reserve released early")
	}
	if err := second.release(); err != nil {
		t.Fatal(err)
	}
	if released.Load() != 1 {
		t.Fatal("last joined borrower did not release once")
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestNodePoolWithdrawalJoinsUnpublishableLateDial(t *testing.T) {
	var p nodePool
	a := role.Authority{}
	a.Duty.NodeID = [32]byte{1}
	entered, gate := make(chan struct{}), make(chan struct{})
	physical := newLifecycleConn(false)
	late := errors.New("late physical retirement failed")
	physical.closeFailure = late
	var released atomic.Int32
	result := make(chan error, 1)
	go func() {
		_, err := p.borrow(t.Context(), a, func() error { return nil }, func(context.Context) (*framing.Session, func(), error) {
			close(entered)
			<-gate
			return framing.New(context.Background(), physical, time.Now().Add(5*time.Second), 32<<20, nil, false, framing.NewBudget(64<<20), nil), func() { released.Add(1) }, nil
		})
		result <- err
	}()
	<-entered
	p.interrupt()
	joined := make(chan error, 1)
	go func() { joined <- p.Close() }()
	select {
	case <-joined:
		t.Fatal("Close did not join owned dial")
	default:
	}
	close(gate)
	if err := lifecycleResult(t, result); !errors.Is(err, late) {
		t.Fatal("late failed Carrier published or error lost", err)
	}
	if err := lifecycleResult(t, joined); !errors.Is(err, late) {
		t.Fatal("pool lost late physical failure", err)
	}
	if released.Load() != 1 {
		t.Fatal("late result reservation leaked")
	}
}

func TestNodePoolRetainsLateWriterFailureAfterBorrowerJoin(t *testing.T) {
	var p nodePool
	late := errors.New("late pooled physical write failed")
	physical := newLifecycleConn(true)
	physical.writeIgnoresClose, physical.partial = true, late
	var released atomic.Int32
	var l *framing.Lane
	c, err := p.borrow(t.Context(), role.Authority{}, func() error { return nil }, func(context.Context) (*framing.Session, func(), error) {
		s, lane, err := newJoinedSession(context.Background(), physical, time.Now().Add(5*time.Second), 32<<20, nil, framing.NewBudget(64<<20))
		l = lane
		return s, func() { released.Add(1) }, err
	})
	if err != nil {
		t.Fatal(err)
	}
	written := make(chan error, 1)
	go func() { _, err := l.Write([]byte("started")); written <- err }()
	<-physical.writes
	joined := make(chan error, 1)
	go func() { joined <- c.release() }()
	<-physical.closed
	if released.Load() != 0 {
		t.Fatal("released before writer joined")
	}
	close(physical.writeGate)
	if err := lifecycleResult(t, written); !errors.Is(err, late) {
		t.Fatal(err)
	}
	if err := lifecycleResult(t, joined); !errors.Is(err, late) {
		t.Fatal(err)
	}
	for range 2 {
		if err := p.Close(); !errors.Is(err, late) {
			t.Fatal("pool discarded joined write failure", err)
		}
	}
}
