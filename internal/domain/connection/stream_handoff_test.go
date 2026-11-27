package connection

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/dianabuilds/ardents-network/internal/application/endpointstream"
	executionruntime "github.com/dianabuilds/ardents-network/internal/domain/execution/runtime"
)

func TestDetachedInitialStateCannotExposeApplicationStream(t *testing.T) {
	for _, owner := range []*AuthenticatedAttachment{nil, {}} {
		if stream, err := owner.OpenStream(executionruntime.StreamContract{}); err == nil || stream != nil {
			t.Fatal("detached initial state supplied an Application stream")
		}
	}
}

// This retirement fixture has no successful Instance, Operation or transport.
func TestCopiedStreamCannotInterruptOriginalLifetime(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	original := &Stream{ctx: ctx, cancel: cancel}
	original.self = original
	copy := &Stream{self: original, ctx: ctx, cancel: cancel}
	if err := copy.Close(); !errors.Is(err, errIntegrity) || ctx.Err() != nil {
		t.Fatal("copied stream interrupted its original", err, ctx.Err())
	}
}

// A completed exchange of Terminal records still requires the original
// operation before the Stream publishes an Application outcome.
func TestTerminalFlagsCannotPublishSuccessWithoutOriginalOperation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream := &Stream{ctx: ctx, cancel: cancel, owner: &AuthenticatedAttachment{},
		transfer: completedTransfer(t), outcome: make(chan endpointstream.Outcome, 1), settledDone: make(chan struct{})}
	stream.mu.Lock()
	stream.settleLocked()
	stream.mu.Unlock()
	if stream.settled || stream.failure == nil || ctx.Err() == nil {
		t.Fatal("synthetic Terminal flags bypassed original authority")
	}
	select {
	case <-stream.outcome:
		t.Fatal("missing Operation published successful terminal outcome")
	default:
	}
}

// These private byte-handoff fixtures have no accepting Stream or authority.
// They isolate the final-fence rule after a real private-buffer read.
func TestReadDeliveryRefusesBytesAfterFailedFinalFence(t *testing.T) {
	staged := make([]byte, 32)
	n, readErr := bytes.NewBufferString("private Application bytes").Read(staged)
	if readErr != nil || n == 0 {
		t.Fatal(n, readErr)
	}
	for _, failure := range []error{context.Canceled, context.DeadlineExceeded, errIntegrity} {
		destination := bytes.Repeat([]byte{0x5a}, len(staged))
		original := bytes.Clone(destination)
		count, err := deliverRead(destination, staged, n, readErr, failure)
		if count != 0 || !errors.Is(err, failure) || !bytes.Equal(destination, original) {
			t.Fatal("failed final fence exposed plaintext/count", count, err, destination)
		}
	}
}

// This failure-only owner cannot authenticate or admit any Application bytes.
// Its early authority refusal must still preserve the stream's first failure.
func TestReadEarlyOwnerRefusalPreservesOriginalFailure(t *testing.T) {
	first := errors.New("original Connection failure")
	stream := &Stream{owner: &AuthenticatedAttachment{}, failure: first}
	stream.self = stream
	destination := []byte{0x5a}
	if n, err := stream.Read(destination); n != 0 || !errors.Is(err, first) || !errors.Is(err, errIntegrity) || destination[0] != 0x5a {
		t.Fatal("early owner refusal hid the first failure or exposed bytes", n, err)
	}
	if n, err := stream.Write([]byte("refused")); n != 0 || !errors.Is(err, first) || !errors.Is(err, errIntegrity) {
		t.Fatal("early owner refusal hid the first write failure", n, err)
	}
}

func TestReadDeliveryPreservesAcceptedBytesAndExactEOF(t *testing.T) {
	destination := make([]byte, 7)
	count, err := deliverRead(destination, []byte("accepted"), 7, nil, nil)
	if count != 7 || err != nil || string(destination) != "accepte" {
		t.Fatal(count, err, destination)
	}
	if count, err := deliverRead(destination, nil, 0, io.EOF, nil); count != 0 || err != io.EOF {
		t.Fatal("authenticated EOF sentinel changed", count, err)
	}
	if count, err := deliverRead(destination, nil, 0, io.EOF, context.Canceled); count != 0 || !errors.Is(err, io.EOF) || !errors.Is(err, context.Canceled) {
		t.Fatal("failed final fence lost original causes", count, err)
	}
}
