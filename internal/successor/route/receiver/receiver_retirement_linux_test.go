//go:build linux

package receiver

import (
	"context"
	"errors"
	framing "github.com/dianabuilds/ardents-network/internal/successor/route/channel"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
	"testing"
	"time"
)

type retiredReceiverListener struct{}

func (retiredReceiverListener) Accept(context.Context, time.Duration) (transport.ClosedSharedCarrier, error) {
	return transport.ClosedSharedCarrier{}, context.Canceled
}
func (retiredReceiverListener) Close() error { return nil }

// Mechanical terminal-owner oracle: no substitute successful authority or
// Admission. Both real receiving session paths call finishSession; genuine
// admitted operation and peer refusal are covered in command Carrier tests.
func TestReceiverRetainsJoinedWriterFailure(t *testing.T) {
	late := errors.New("late receiving physical write failed")
	physical := newLifecycleConn(true)
	physical.writeIgnoresClose, physical.partial = true, late
	r := &Receiver{}
	s, l, err := newJoinedSession(context.Background(), physical, time.Now().Add(5*time.Second), 32<<20, nil, framing.NewBudget(64<<20))
	if err != nil {
		t.Fatal(err)
	}
	written := make(chan error, 1)
	go func() { _, err := l.Write([]byte("started")); written <- err }()
	<-physical.writes
	joined := make(chan error, 1)
	go func() { joined <- r.finishSession(s) }()
	<-physical.closed
	select {
	case <-joined:
		t.Fatal("receiver completed before write joined")
	default:
	}
	close(physical.writeGate)
	if err := lifecycleResult(t, written); !errors.Is(err, late) {
		t.Fatal(err)
	}
	if err := lifecycleResult(t, joined); !errors.Is(err, late) {
		t.Fatal(err)
	}
	r.mu.Lock()
	retained := r.err
	r.mu.Unlock()
	if !errors.Is(retained, late) {
		t.Fatal("receiver discarded joined write failure", retained)
	}
	// The listener is already joined in this isolated terminal-owner oracle.
	r.ctx, r.cancel = context.WithCancel(context.Background())
	r.listener, r.done = retiredReceiverListener{}, make(chan struct{})
	close(r.done)
	for range 2 {
		if err := r.Close(); !errors.Is(err, late) {
			t.Fatal("receiver Close replaced joined failure", err)
		}
	}
}

func TestReceiverDoesNotRetainPeerSessionRefusal(t *testing.T) {
	r := &Receiver{}
	s := framing.New(context.Background(), newLifecycleConn(false), time.Now().Add(5*time.Second), 32<<20, nil, false, framing.NewBudget(64<<20), nil)
	refusal := errors.New("peer session refused")
	s.Retire(refusal)
	if err := r.finishSession(s); !errors.Is(err, refusal) {
		t.Fatal(err)
	}
	if r.err != nil {
		t.Fatal("peer refusal poisoned receiver", r.err)
	}
}
