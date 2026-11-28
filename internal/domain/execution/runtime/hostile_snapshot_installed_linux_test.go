//go:build linux && document_worker_installed

package runtime

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/endpointstream"
)

// The request and terminal outcome are local fixtures. The snapshot response
// crosses the original qualified Publisher worker after victim cleanup.
func requireHostilePublisherSnapshot(t *testing.T, parent context.Context, operation *Operation, document []byte) {
	t.Helper()
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	var request [0]byte
	stream := &hostileSnapshotStream{
		Reader: bytes.NewReader(request[:]), limit: len(document),
		terminal: make(chan endpointstream.Outcome, 1), closedSignal: make(chan struct{}),
	}
	t.Cleanup(func() { _ = stream.Close() })
	contract, err := operation.BindStreamContract(endpointstream.ByteStreamSegmentDirectionalLimit, endpointstream.ByteStreamSegmentDirectionalLimit)
	if err != nil {
		t.Fatal(err)
	}
	transferred, err := operation.DeliverPublisherStream(ctx, contract, stream)
	if err != nil || !transferred {
		t.Fatalf("original Publisher did not accept the local stream: %v", err)
	}
	select {
	case <-stream.closedSignal:
	case <-ctx.Done():
		t.Fatal("original Publisher did not complete the snapshot exchange")
	}
	stream.mu.Lock()
	defer stream.mu.Unlock()
	body := stream.response.Bytes()
	if !stream.completed || !bytes.Equal(body, document) {
		t.Fatal("original Publisher did not return its exact immutable snapshot after victim cleanup")
	}
}

type hostileSnapshotStream struct {
	*bytes.Reader
	mu           sync.Mutex
	response     bytes.Buffer
	limit        int
	completed    bool
	closed       bool
	terminal     chan endpointstream.Outcome
	closedSignal chan struct{}
}

func (stream *hostileSnapshotStream) Write(body []byte) (int, error) {
	stream.mu.Lock()
	defer stream.mu.Unlock()
	if stream.completed || stream.closed {
		return 0, io.ErrClosedPipe
	}
	if len(body) > stream.limit-stream.response.Len() {
		return 0, errors.New("Publisher snapshot exceeded its response bound")
	}
	return stream.response.Write(body)
}

func (stream *hostileSnapshotStream) CloseInput() error {
	stream.mu.Lock()
	defer stream.mu.Unlock()
	if stream.completed || stream.closed {
		return io.ErrClosedPipe
	}
	stream.completed = true
	stream.terminal <- endpointstream.Outcome{Class: endpointstream.CleanClose}
	close(stream.terminal)
	return nil
}

func (stream *hostileSnapshotStream) Done() <-chan endpointstream.Outcome { return stream.terminal }

func (stream *hostileSnapshotStream) Close() error {
	stream.mu.Lock()
	defer stream.mu.Unlock()
	if !stream.closed {
		stream.closed = true
		if !stream.completed {
			stream.terminal <- endpointstream.Outcome{Class: endpointstream.LocalCancellation}
			close(stream.terminal)
		}
		close(stream.closedSignal)
	}
	return nil
}
