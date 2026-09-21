//go:build linux

package connection_test

import (
	"bytes"
	"context"
	"io"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/interfacev2/connection"
)

type reverseOwner struct {
	calls  atomic.Int32
	opened chan *reverseAfterEOFStream
}

func (owner *reverseOwner) Open(_ context.Context, _ connection.Request) (connection.Stream, error) {
	owner.calls.Add(1)
	reader, writer := io.Pipe()
	stream := &reverseAfterEOFStream{
		reader: reader,
		writer: writer,
		done:   make(chan connection.Outcome, 1),
	}
	owner.opened <- stream
	return stream, nil
}

type reverseAfterEOFStream struct {
	reader *io.PipeReader
	writer *io.PipeWriter
	done   chan connection.Outcome
	mu     sync.Mutex
	input  bytes.Buffer

	inputChunks  []int
	responseCaps []int
	readIndex    int
	inputOnce    sync.Once
	closeOnce    sync.Once
}

func (stream *reverseAfterEOFStream) Read(body []byte) (int, error) {
	responseCaps := [...]int{13, 71, 19, 97}
	limit := responseCaps[stream.readIndex%len(responseCaps)]
	stream.readIndex++
	if len(body) > limit {
		body = body[:limit]
	}
	read, err := stream.reader.Read(body)
	if read > 0 {
		stream.mu.Lock()
		stream.responseCaps = append(stream.responseCaps, read)
		stream.mu.Unlock()
	}
	return read, err
}

func (stream *reverseAfterEOFStream) Write(body []byte) (int, error) {
	stream.mu.Lock()
	defer stream.mu.Unlock()
	stream.inputChunks = append(stream.inputChunks, len(body))
	return stream.input.Write(body)
}

func (stream *reverseAfterEOFStream) CloseInput() error {
	var closeErr error
	stream.inputOnce.Do(func() {
		stream.mu.Lock()
		response := slices.Clone(stream.input.Bytes())
		stream.mu.Unlock()
		slices.Reverse(response)
		_, writeErr := stream.writer.Write(response)
		closeErr = stream.writer.CloseWithError(writeErr)
		stream.done <- connection.Outcome{Class: connection.CleanClose}
		close(stream.done)
	})
	return closeErr
}

func (stream *reverseAfterEOFStream) Close() error {
	stream.closeOnce.Do(func() {
		_ = stream.reader.Close()
		_ = stream.writer.Close()
	})
	return nil
}

func (stream *reverseAfterEOFStream) Done() <-chan connection.Outcome { return stream.done }

func (stream *reverseAfterEOFStream) observations() ([]byte, []int, []int) {
	stream.mu.Lock()
	defer stream.mu.Unlock()
	return slices.Clone(stream.input.Bytes()), slices.Clone(stream.inputChunks), slices.Clone(stream.responseCaps)
}

func TestTypedConnectionPreservesAllBytesAcrossUnequalFragmentsAfterInputEOF(t *testing.T) {
	owner := &reverseOwner{opened: make(chan *reverseAfterEOFStream, 1)}
	path := shortSocketPath(t)
	server, err := connection.Listen(path, owner)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Errorf("close server: %v", err)
		}
	})

	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	stream, err := connection.Dial(ctx, path, connection.Request{
		Destination: connection.TargetLink,
		Value:       "all-byte-conformance",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := stream.Close(); err != nil {
			t.Errorf("close stream: %v", err)
		}
	})
	opened := <-owner.opened

	payload := make([]byte, 256)
	for value := range payload {
		payload[value] = byte(value)
	}
	inputChunks := []int{1, 17, 63, 175}
	offset := 0
	for _, size := range inputChunks {
		written, writeErr := stream.Write(payload[offset : offset+size])
		if writeErr != nil || written != size {
			t.Fatalf("write %d bytes = %d, %v", size, written, writeErr)
		}
		offset += size
	}
	if err := stream.CloseInput(); err != nil {
		t.Fatal(err)
	}

	var response bytes.Buffer
	clientReadCaps := [...]int{5, 29, 101, 7}
	for index := 0; ; index++ {
		buffer := make([]byte, clientReadCaps[index%len(clientReadCaps)])
		read, readErr := stream.Read(buffer)
		response.Write(buffer[:read])
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			t.Fatal(readErr)
		}
	}

	wantResponse := slices.Clone(payload)
	slices.Reverse(wantResponse)
	if !bytes.Equal(response.Bytes(), wantResponse) {
		t.Fatalf("response differs at all-byte boundary: got %d bytes", response.Len())
	}
	ownerInput, ownerInputChunks, responseCaps := opened.observations()
	if !bytes.Equal(ownerInput, payload) {
		t.Fatalf("owner input differs: got %d bytes", len(ownerInput))
	}
	if !slices.Equal(ownerInputChunks, inputChunks) {
		t.Fatalf("owner input chunks = %v, want %v", ownerInputChunks, inputChunks)
	}
	if len(responseCaps) < 2 || slices.Equal(responseCaps, inputChunks) {
		t.Fatalf("response fragments did not differ from input: %v", responseCaps)
	}
	outcome, open := <-stream.Done()
	if !open || outcome.Class != connection.CleanClose {
		t.Fatalf("terminal = %+v, open=%t", outcome, open)
	}
	if owner.calls.Load() != 1 {
		t.Fatalf("owner Open calls = %d", owner.calls.Load())
	}
}
