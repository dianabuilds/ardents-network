package connection

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// F-23: one Attachment retirement result is retained exactly once, even when
// two lifecycle owners release the same failed carrier.
func TestStreamRetainsAttachmentRetirementResultExactlyOnce(t *testing.T) {
	sentinel := errors.New("retained attachment retirement failure")
	var calls atomic.Int32
	carrier, peer := net.Pipe()
	defer peer.Close()
	attachment, err := NewAttachment(carrier, 1, [32]byte{1}, [32]byte{2}, func() error {
		calls.Add(1)
		_ = carrier.Close()
		return sentinel
	})
	if err != nil {
		t.Fatal(err)
	}
	stream := &Stream{done: make(chan struct{})}
	stream.cond = sync.NewCond(&stream.mu)
	stream.retireAttachment(attachment)
	stream.retireAttachment(attachment)
	stream.retireAttachment(nil)
	if calls.Load() != 1 {
		t.Fatalf("close callback ran %d times", calls.Load())
	}
	result := stream.RetirementResult()
	if !errors.Is(result, sentinel) {
		t.Fatalf("retirement result %v does not retain the sentinel", result)
	}
	if joined, ok := result.(interface{ Unwrap() []error }); ok && len(joined.Unwrap()) != 1 {
		t.Fatalf("retirement result duplicated the retained failure: %v", result)
	}
}

// F-23: the ordinary RunBounded path closes done only after the physical
// Attachment retirement and retains its failure for the post-Done reader.
func TestRunBoundedRetiresAttachmentBeforeDoneAndRetainsCloseFailure(t *testing.T) {
	clientCarrier, publisherCarrier := net.Pipe()
	clientApplication, clientUser := halfClosePair()
	publisherApplication, publisherUser := halfClosePair()
	defer clientUser.Close()
	defer publisherUser.Close()
	sentinel := errors.New("publisher transport retirement failed")
	retired := make(chan struct{})
	connectionContext, exporter, key := [32]byte{11}, [32]byte{12}, [32]byte{13}
	clientAttachment, err := NewAttachment(clientCarrier, 1, connectionContext, exporter, nil)
	if err != nil {
		t.Fatal(err)
	}
	publisherAttachment, err := NewAttachment(publisherCarrier, 1, connectionContext, exporter, func() error {
		_ = publisherCarrier.Close()
		close(retired)
		return sentinel
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	client, err := NewStream(StreamConfig{Context: ctx, Application: clientApplication, Initial: clientAttachment,
		ContinuityKey: key, Authorized: time.Now(), Client: true})
	if err != nil {
		t.Fatal(err)
	}
	publisher, err := NewStream(StreamConfig{Context: ctx, Application: publisherApplication, Initial: publisherAttachment,
		ContinuityKey: key, Authorized: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	go func() { _, err := client.RunBounded(3, 3); results <- err }()
	go func() { _, err := publisher.RunBounded(3, 3); results <- err }()
	go func() { _, _ = clientUser.Write([]byte("one")); _ = clientUser.CloseInput() }()
	go func() { _, _ = publisherUser.Write([]byte("two")); _ = publisherUser.CloseInput() }()
	if _, err := io.ReadFull(clientUser, make([]byte, 3)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(publisherUser, make([]byte, 3)); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := <-results; err != nil {
			t.Fatalf("native stream failed: %v", err)
		}
	}
	select {
	case <-publisher.done:
	default:
		t.Fatal("publisher done remained open after its RunBounded returned")
	}
	select {
	case <-retired:
	default:
		t.Fatal("publisher done closed before the physical Attachment retirement")
	}
	if result := publisher.RetirementResult(); !errors.Is(result, sentinel) {
		t.Fatalf("publisher retirement result %v does not retain the close failure", result)
	}
	if result := client.RetirementResult(); result != nil {
		t.Fatalf("client retained an unexpected retirement result: %v", result)
	}
}

// F-23: the terminal-control tail also retains its physical retirement
// failure; its done channel already closed only after stream.close.
func TestTerminalTailRetainsAttachmentRetirementResult(t *testing.T) {
	application, applicationPeer := halfClosePair()
	defer applicationPeer.Close()
	carrier, peer := net.Pipe()
	defer peer.Close()
	sentinel := errors.New("tail transport retirement failed")
	attachment, err := NewAttachment(carrier, 1, [32]byte{1}, [32]byte{2}, func() error {
		_ = carrier.Close()
		return sentinel
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	stream := &Stream{ctx: t.Context(), application: application, current: attachment, authorized: now, started: now,
		opener:   func(context.Context, Recovery) (*Attachment, error) { return nil, nil },
		recovery: Recovery{NoNewRecoveryAfter: now.Add(time.Minute).Unix()}, localTerminal: true, remoteTerminal: true,
		terminalAcknowledgedGeneration: 1, terminalAckPending: true, terminalAckSent: true,
		terminalAckGeneration: 1, terminalAckConfirmedGeneration: 1, terminalConfirmationPending: true,
		terminalConfirmationSent: true, ackSignal: make(chan struct{}, 1), done: make(chan struct{}),
		resources: func(string, int) uint32 { return 0 }}
	stream.cond = sync.NewCond(&stream.mu)
	if !stream.startTerminalTail(func() {}, nil) {
		t.Fatal("eligible terminal-control tail was not started")
	}
	if err := stream.RetireTerminalTail(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-stream.done:
	case <-time.After(time.Second):
		t.Fatal("terminal-control tail did not retire")
	}
	if result := stream.RetirementResult(); !errors.Is(result, sentinel) {
		t.Fatalf("tail retirement result %v does not retain the close failure", result)
	}
}
