package connection

import (
	"bytes"
	"context"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

func TestRunBoundedOrdersNewDataAndTerminalAfterRecoveredDataReplay(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	oldClient, oldPublisher, fault := newTerminalFaultAdapter(t, terminalFault{dropData: true})
	defer fault.Close()
	clientCarrier, publisherCarrier := net.Pipe()
	gate := &replayDataGate{Conn: clientCarrier, blocked: make(chan struct{}), release: make(chan struct{})}
	defer gate.Close()
	defer publisherCarrier.Close()
	clientApplication, clientUser := halfClosePair()
	publisherApplication, publisherUser := halfClosePair()
	defer clientUser.Close()
	defer publisherUser.Close()
	request := bytes.Repeat([]byte("r"), MaximumDataBytes+1)
	expected := append(append([]byte(nil), request...), '!')
	watched := &idleInputReadBarrier{Application: clientApplication, want: len(request), entered: make(chan struct{})}
	connectionContext, exporter, key := [32]byte{1}, [32]byte{2}, [32]byte{3}
	deadline := time.Now().Add(time.Minute).Unix()
	recovery := Recovery{NoNewRecoveryAfter: deadline}
	client, err := NewStream(StreamConfig{Context: ctx, Application: watched, Initial: terminalRecoveryAttachment(t, oldClient, 1, connectionContext, exporter),
		ContinuityKey: key, Authorized: time.Now(), Client: true, Recovery: recovery, OpenAttachment: func(context.Context, Recovery) (*Attachment, error) {
			return terminalRecoveryAttachment(t, gate, 2, connectionContext, exporter), nil
		}})
	if err != nil {
		t.Fatal(err)
	}
	publisher, err := NewStream(StreamConfig{Context: ctx, Application: publisherApplication, Initial: terminalRecoveryAttachment(t, oldPublisher, 1, connectionContext, exporter),
		ContinuityKey: key, Authorized: time.Now(), Recovery: recovery, OpenAttachment: func(context.Context, Recovery) (*Attachment, error) {
			return terminalRecoveryAttachment(t, publisherCarrier, 2, connectionContext, exporter), nil
		}})
	if err != nil {
		t.Fatal(err)
	}
	type result struct {
		outcome Outcome
		err     error
	}
	results := make(chan result, 2)
	go func() {
		outcome, runErr := client.RunBounded(uint32(len(request)+1), 64)
		results <- result{outcome, runErr}
	}()
	go func() {
		outcome, runErr := publisher.RunBounded(64, uint32(len(request)+1))
		results <- result{outcome, runErr}
	}()
	readerDone := make(chan struct{})
	got := make(chan error, 1)
	go func() {
		defer close(readerDone)
		data := make([]byte, len(request)+1)
		_, readErr := io.ReadFull(publisherUser, data)
		if readErr == nil && !bytes.Equal(data, expected) {
			readErr = io.ErrUnexpectedEOF
		}
		if readErr == nil {
			var after [1]byte
			if count, closeErr := publisherUser.Read(after[:]); count != 0 || closeErr != io.EOF {
				readErr = io.ErrUnexpectedEOF
			}
		}
		got <- readErr
	}()
	remaining := 2
	t.Cleanup(func() {
		cancel()
		gate.Close()
		_ = clientUser.Close()
		_ = publisherUser.Close()
		select {
		case <-readerDone:
		case <-time.After(time.Second):
			t.Error("ordered replay reader cleanup did not join")
		}
		for range remaining {
			select {
			case <-results:
			case <-time.After(time.Second):
				t.Error("ordered replay stream cleanup did not join")
			}
		}
		for _, stream := range []*Stream{client, publisher} {
			select {
			case <-stream.done:
			case <-time.After(time.Second):
				t.Errorf("ordered replay tail cleanup did not join: %s", terminalRecoveryState(stream))
			}
		}
	})
	if _, err = clientUser.Write(request); err != nil {
		t.Fatal(err)
	}
	select {
	case <-watched.entered:
	case <-ctx.Done():
		t.Fatal("sender did not enter its next Application read")
	}
	select {
	case <-fault.prefixAcknowledged:
	case <-ctx.Done():
		t.Fatal("delivered data prefix was not acknowledged")
	}
	fault.Close()
	select {
	case <-gate.blocked:
	case <-ctx.Done():
		t.Fatal("recovered Data replay did not reach the gated write")
	}
	if _, err = clientUser.Write([]byte("!")); err != nil {
		t.Fatal(err)
	}
	if err = clientUser.CloseWrite(); err != nil {
		t.Fatal(err)
	}
	gate.Release()
	select {
	case err = <-got:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("ordered recovered request and Terminal did not arrive")
	}
	if err = publisherUser.CloseWrite(); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		select {
		case result := <-results:
			remaining--
			if result.err != nil || result.outcome.Generation != 2 || result.outcome.Recoveries != 1 {
				t.Fatalf("stream result = %+v, %v; client=%s publisher=%s", result.outcome, result.err,
					terminalRecoveryState(client), terminalRecoveryState(publisher))
			}
		case <-ctx.Done():
			t.Fatalf("streams did not finish after ordered replay: client=%s publisher=%s", terminalRecoveryState(client), terminalRecoveryState(publisher))
		}
	}
}

type replayDataGate struct {
	net.Conn
	blocked, release chan struct{}
	blockOnce        sync.Once
	releaseOnce      sync.Once
}

func (gate *replayDataGate) Write(value []byte) (int, error) {
	if len(value) > 2 && value[2] == kindData {
		gate.blockOnce.Do(func() {
			close(gate.blocked)
			<-gate.release
		})
	}
	return gate.Conn.Write(value)
}

func (gate *replayDataGate) Release() {
	gate.releaseOnce.Do(func() { close(gate.release) })
}

func (gate *replayDataGate) Close() error {
	gate.Release()
	return gate.Conn.Close()
}
