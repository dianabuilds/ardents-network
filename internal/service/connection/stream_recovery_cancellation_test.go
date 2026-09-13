package connection

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"
)

func TestRecoveryCancellationClosesAndJoinsProposedContinuity(t *testing.T) {
	now := time.Now()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	failedCarrier, failedPeer := net.Pipe()
	defer failedPeer.Close()
	failed, err := NewAttachment(failedCarrier, 1, [32]byte{1}, [32]byte{2}, nil)
	if err != nil {
		t.Fatal(err)
	}
	proposedCarrier, proposedPeer := net.Pipe()
	defer proposedPeer.Close()
	closed := make(chan struct{})
	var closeOnce sync.Once
	proposed, err := NewAttachment(proposedCarrier, 2, [32]byte{3}, [32]byte{4}, func() {
		closeOnce.Do(func() {
			_ = proposedCarrier.Close()
			close(closed)
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	opened := make(chan struct{})
	stream := &Stream{
		ctx: ctx, networkID: [32]byte{5}, recovery: Recovery{NoNewRecoveryAfter: now.Add(time.Minute).Unix()},
		opener: func(context.Context, Recovery) (*Attachment, error) {
			close(opened)
			return proposed, nil
		},
		continuity: [32]byte{6}, client: true, authorized: now, started: now, lastProgress: now,
		resources: func(string, int) uint32 { return 0 }, current: failed, ackSignal: make(chan struct{}, 1),
	}
	stream.cond = sync.NewCond(&stream.mu)
	result := make(chan error, 1)
	go func() { result <- stream.recoverAttachment(failed) }()
	joined := false
	t.Cleanup(func() {
		cancel()
		_ = proposedPeer.Close()
		if joined {
			return
		}
		select {
		case <-result:
		case <-time.After(time.Second):
			t.Error("recovery cleanup did not join proposed Continuity I/O")
		}
	})
	select {
	case <-opened:
	case <-time.After(time.Second):
		t.Fatal("recovery did not open the proposed Attachment")
	}
	record, err := Read(proposedPeer)
	if err != nil || record.Continuity == nil {
		t.Fatalf("recovery did not reach Continuity exchange: %+v, %v", record, err)
	}
	cancel()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("cancellation did not close proposed Continuity carrier")
	}
	select {
	case err := <-result:
		joined = true
		if !errors.Is(err, context.Canceled) || errors.Is(err, ErrActiveViolation) {
			t.Fatalf("recovery cancellation outcome = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("recovery did not join proposed Continuity I/O")
	}
	stream.mu.Lock()
	current, recoveries := stream.current, stream.recoveries
	stream.mu.Unlock()
	if current != failed || recoveries != 0 {
		t.Fatal("cancelled proposed Attachment crossed cutover")
	}
}

func TestCutoverRejectsAttachmentAfterConnectionCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	failed := &Attachment{generation: 1}
	proposed := &Attachment{generation: 2}
	stream := &Stream{ctx: ctx, current: failed}
	cancel()
	peer := ContinuityPeer{PeerNonce: [32]byte{1}, LocalNonce: [32]byte{2}}
	if err := stream.commitAttachment(failed, proposed, peer); !errors.Is(err, context.Canceled) || errors.Is(err, ErrActiveViolation) || stream.current != failed {
		t.Fatal("cancelled Connection accepted a late cutover")
	}
}

func TestCutoverRetainsExistingConnectionFailure(t *testing.T) {
	terminal := errors.New("existing Connection failure")
	failed := &Attachment{generation: 1}
	proposed := &Attachment{generation: 2}
	stream := &Stream{terminal: terminal, current: failed}
	peer := ContinuityPeer{PeerNonce: [32]byte{1}, LocalNonce: [32]byte{2}}
	if err := stream.commitAttachment(failed, proposed, peer); !errors.Is(err, terminal) || errors.Is(err, ErrActiveViolation) || stream.current != failed {
		t.Fatal("late cutover replaced the existing Connection failure")
	}
}
