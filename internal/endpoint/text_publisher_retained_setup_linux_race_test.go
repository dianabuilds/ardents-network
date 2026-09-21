//go:build linux && race

package endpoint

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/interfacev2/connection"
	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
)

type retainedQualificationMemoryStream struct {
	inbound   *retainedQualificationMemoryDirection
	outbound  *retainedQualificationMemoryDirection
	done      chan connection.Outcome
	closeOnce sync.Once
}

type retainedQualificationMemoryDirection struct {
	mu            sync.Mutex
	drained       *sync.Cond
	frames        chan []byte
	closing       chan struct{}
	closed        chan struct{}
	closeOnce     sync.Once
	closingSet    bool
	inflight      int
	writeAdmitted chan struct{}
}

func (direction *retainedQualificationMemoryDirection) write(body []byte) (int, error) {
	direction.mu.Lock()
	if direction.closingSet {
		direction.mu.Unlock()
		return 0, io.ErrClosedPipe
	}
	direction.inflight++
	admitted := direction.writeAdmitted
	direction.writeAdmitted = nil
	direction.mu.Unlock()
	if admitted != nil {
		admitted <- struct{}{}
	}
	var written int
	var outcome error
	select {
	case direction.frames <- append([]byte(nil), body...):
		written = len(body)
	case <-direction.closing:
		outcome = io.ErrClosedPipe
	}
	direction.mu.Lock()
	direction.inflight--
	if direction.inflight == 0 {
		direction.drained.Broadcast()
	}
	direction.mu.Unlock()
	return written, outcome
}

func (direction *retainedQualificationMemoryDirection) close() {
	direction.mu.Lock()
	if !direction.closingSet {
		direction.closingSet = true
		close(direction.closing)
	}
	for direction.inflight != 0 {
		direction.drained.Wait()
	}
	direction.mu.Unlock()
	direction.closeOnce.Do(func() { close(direction.closed) })
	<-direction.closed
}

func (direction *retainedQualificationMemoryDirection) observeNextWriteAdmission() <-chan struct{} {
	direction.mu.Lock()
	defer direction.mu.Unlock()
	admitted := make(chan struct{}, 1)
	direction.writeAdmitted = admitted
	return admitted
}

func (direction *retainedQualificationMemoryDirection) read(into []byte) (int, error) {
	select {
	case body := <-direction.frames:
		return copy(into, body), nil
	default:
	}
	select {
	case body := <-direction.frames:
		return copy(into, body), nil
	case <-direction.closing:
		<-direction.closed
		// A Write ordered before CloseInput remains readable before EOF.
		select {
		case body := <-direction.frames:
			return copy(into, body), nil
		default:
			return 0, io.EOF
		}
	}
}

type retainedQualificationMemoryOwner struct {
	mu      sync.Mutex
	streams []*retainedQualificationMemoryStream
}

func (owner *retainedQualificationMemoryOwner) pair() (*retainedQualificationMemoryStream, *retainedQualificationMemoryStream) {
	leftToRight := newRetainedQualificationMemoryDirection()
	rightToLeft := newRetainedQualificationMemoryDirection()
	left := &retainedQualificationMemoryStream{inbound: rightToLeft, outbound: leftToRight, done: make(chan connection.Outcome, 1)}
	right := &retainedQualificationMemoryStream{inbound: leftToRight, outbound: rightToLeft, done: make(chan connection.Outcome, 1)}
	owner.mu.Lock()
	owner.streams = append(owner.streams, left, right)
	owner.mu.Unlock()
	return left, right
}

func newRetainedQualificationMemoryDirection() *retainedQualificationMemoryDirection {
	direction := &retainedQualificationMemoryDirection{
		frames: make(chan []byte, 1), closing: make(chan struct{}), closed: make(chan struct{}),
	}
	direction.drained = sync.NewCond(&direction.mu)
	return direction
}

func (owner *retainedQualificationMemoryOwner) closeAndAssert(t *testing.T) {
	t.Helper()
	owner.mu.Lock()
	streams := append([]*retainedQualificationMemoryStream(nil), owner.streams...)
	owner.mu.Unlock()
	for _, stream := range streams {
		if err := stream.Close(); err != nil {
			t.Errorf("retained memory Stream close: %v", err)
		}
	}
	for _, stream := range streams {
		outcome, ok := <-stream.Done()
		if !ok || outcome.Class == "" {
			t.Error("retained memory Stream terminal outcome unavailable")
		}
		if _, open := <-stream.Done(); open {
			t.Error("retained memory Stream published more than one terminal outcome")
		}
	}
}

func (stream *retainedQualificationMemoryStream) Read(into []byte) (int, error) {
	return stream.inbound.read(into)
}

func (stream *retainedQualificationMemoryStream) Write(body []byte) (int, error) {
	return stream.outbound.write(body)
}

func (stream *retainedQualificationMemoryStream) CloseInput() error {
	stream.outbound.close()
	return nil
}

func (stream *retainedQualificationMemoryStream) Done() <-chan connection.Outcome { return stream.done }

func (stream *retainedQualificationMemoryStream) Close() error {
	stream.closeOnce.Do(func() {
		stream.inbound.close()
		stream.outbound.close()
		stream.done <- connection.Outcome{Class: connection.CleanClose}
		close(stream.done)
	})
	return nil
}

func TestRetainedQualificationMemoryStreamCloseInterruptsBlockedWrite(t *testing.T) {
	owner := &retainedQualificationMemoryOwner{}
	defer owner.closeAndAssert(t)
	writer, _ := owner.pair()
	if _, err := writer.Write([]byte{1}); err != nil {
		t.Fatal(err)
	}
	admitted := writer.outbound.observeNextWriteAdmission()
	writeDone := make(chan error, 1)
	go func() {
		_, err := writer.Write([]byte{2})
		writeDone <- err
	}()
	<-admitted
	if err := writer.CloseInput(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-writeDone:
		if !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("blocked Write result = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("CloseInput did not interrupt blocked Write")
	}
}

// The real loopback/cryptographic composition keeps the full 4x64 assertion
// in the non-race profile. Under the race detector, prove the same concurrent
// Reader ownership, workload binding, retained identity set, and ready barrier
// at the in-memory stream seam without making wall-clock throughput the oracle.
func TestTextPublisherBuildsRetainedQualificationSetAcrossFourReaders(t *testing.T) {
	const readerCount, streamsPerReader = 4, 64
	type readerResult struct {
		streams []streamqualification.BoundStream
		err     error
	}
	ctx, cancel := context.WithCancel(t.Context())
	var readerWork sync.WaitGroup
	streamOwner := &retainedQualificationMemoryOwner{}
	defer func() {
		cancel()
		readerWork.Wait()
		streamOwner.closeAndAssert(t)
	}()
	delivered := make(chan connection.Stream, readerCount*streamsPerReader)
	readers := make(chan readerResult, readerCount)
	for reader := 0; reader < readerCount; reader++ {
		reader := reader
		readerWork.Add(1)
		go func() {
			defer readerWork.Done()
			streams, err := openQualificationReaderStreams(ctx, streamsPerReader, qualificationReaderSetupParallelism,
				time.Time{}, 0, func(ctx context.Context, index int) (streamqualification.BoundStream, error) {
					readerStream, publisherStream := streamOwner.pair()
					id := qualificationStreamID(reader, index)
					if _, err := readerStream.Write(qualificationHello(streamqualification.ClientToPublisher, fixtureID(246), id)); err != nil {
						return streamqualification.BoundStream{}, err
					}
					select {
					case delivered <- publisherStream:
						return streamqualification.BoundStream{ID: id, Stream: readerStream}, nil
					case <-ctx.Done():
						return streamqualification.BoundStream{}, ctx.Err()
					}
				})
			readers <- readerResult{streams: streams, err: err}
		}()
	}

	wanted := readerCount * streamsPerReader
	readerByID := make(map[uint32]connection.Stream, wanted)
	for range readerCount {
		result := <-readers
		if result.err != nil {
			t.Fatal(result.err)
		}
		for _, bound := range result.streams {
			if bound.ID == 0 || bound.Stream == nil || readerByID[bound.ID] != nil {
				t.Fatalf("invalid retained Reader stream %d", bound.ID)
			}
			readerByID[bound.ID] = bound.Stream
		}
	}
	publisherByID := make(map[uint32]connection.Stream, wanted)
	for range wanted {
		stream := <-delivered
		var hello [45]byte
		if _, err := io.ReadFull(stream, hello[:]); err != nil {
			t.Fatal(err)
		}
		id := binary.BigEndian.Uint32(hello[9:13])
		expected := qualificationHello(streamqualification.ClientToPublisher, fixtureID(246), id)
		if id == 0 || publisherByID[id] != nil || string(hello[:]) != string(expected) {
			t.Fatalf("invalid retained Publisher stream %d", id)
		}
		publisherByID[id] = stream
	}
	if len(readerByID) != wanted || len(publisherByID) != wanted {
		t.Fatalf("retained setup = %d Publisher / %d Reader streams", len(publisherByID), len(readerByID))
	}
	for id, publisherStream := range publisherByID {
		if _, err := publisherStream.Write([]byte{1}); err != nil {
			t.Fatalf("Publisher ready %d: %v", id, err)
		}
		var ready [1]byte
		if _, err := io.ReadFull(readerByID[id], ready[:]); err != nil || ready[0] != 1 {
			t.Fatalf("Reader ready %d unavailable: %v", id, err)
		}
		if err := readerByID[id].CloseInput(); err != nil {
			t.Fatalf("Reader input close %d: %v", id, err)
		}
		if read, err := publisherStream.Read(ready[:]); read != 0 || err != io.EOF {
			t.Fatalf("Publisher EOF %d = %d, %v", id, read, err)
		}
		if err := publisherStream.CloseInput(); err != nil {
			t.Fatalf("Publisher input close %d: %v", id, err)
		}
		if read, err := readerByID[id].Read(ready[:]); read != 0 || err != io.EOF {
			t.Fatalf("Reader EOF %d = %d, %v", id, read, err)
		}
	}
}
