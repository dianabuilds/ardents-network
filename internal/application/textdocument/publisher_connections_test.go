//go:build linux

package textdocument_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/interfacev2/connection"
	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
)

type publisherHarness struct {
	incoming chan connection.Stream
	finished chan struct{}
	err      error
	cancel   context.CancelFunc
}

func startPublisherHarness(t *testing.T, body []byte, decorate func(net.Conn) io.ReadWriteCloser) *publisherHarness {
	t.Helper()
	return startPublisherHarnessWithWorker(t, body, decorate, nil)
}

func startPublisherHarnessWithWorker(t *testing.T, body []byte, decorate, decorateWorker func(net.Conn) io.ReadWriteCloser) *publisherHarness {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	endpoint, worker := net.Pipe()
	var workerAttachment io.ReadWriteCloser = worker
	if decorateWorker != nil {
		workerAttachment = decorateWorker(worker)
	}
	workerFinished := make(chan struct{})
	go func() {
		defer close(workerFinished)
		_ = textdocument.RunWorker(ctx, workerAttachment, textdocument.PublisherWorker)
	}()
	t.Cleanup(func() { cancel(); _ = endpoint.Close(); _ = worker.Close(); <-workerFinished })
	if err := textdocument.InitializeWorker(ctx, endpoint, textdocument.PublisherWorker, [32]byte{1}, body); err != nil {
		t.Fatal(err)
	}
	var attachment io.ReadWriteCloser = endpoint
	if decorate != nil {
		attachment = decorate(endpoint)
	}
	harness := &publisherHarness{incoming: make(chan connection.Stream), finished: make(chan struct{}), cancel: cancel}
	go func() {
		defer close(harness.finished)
		harness.err = textdocument.ServeWorkerConnections(ctx, attachment, harness.incoming)
	}()
	t.Cleanup(func() { cancel(); <-harness.finished })
	return harness
}

func (harness *publisherHarness) admit(t *testing.T, stream connection.Stream) {
	t.Helper()
	select {
	case harness.incoming <- stream:
	case <-harness.finished:
		t.Fatalf("Publisher stopped before admission: %v", harness.err)
	case <-time.After(5 * time.Second):
		t.Fatal("Publisher admission stalled")
	}
}

func (harness *publisherHarness) drain(t *testing.T) {
	t.Helper()
	close(harness.incoming)
	select {
	case <-harness.finished:
		if harness.err != nil {
			t.Fatal(harness.err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Publisher did not drain admitted streams")
	}
}

type publisherFixture struct {
	input     *bytes.Reader
	gate      <-chan struct{}
	started   chan struct{}
	startOnce sync.Once
	closed    chan struct{}
	closeOnce sync.Once
	terminal  chan connection.Outcome
	doneOnce  sync.Once
	mu        sync.Mutex
	response  bytes.Buffer
	closeErr  error
}

func newPublisherFixture(request []byte, gate <-chan struct{}) *publisherFixture {
	return &publisherFixture{input: bytes.NewReader(request), gate: gate, started: make(chan struct{}),
		closed: make(chan struct{}), terminal: make(chan connection.Outcome, 1)}
}

func (stream *publisherFixture) Read(body []byte) (int, error) { return stream.input.Read(body) }
func (stream *publisherFixture) Write(body []byte) (int, error) {
	stream.startOnce.Do(func() { close(stream.started) })
	if stream.gate != nil {
		select {
		case <-stream.gate:
		case <-stream.closed:
			return 0, io.ErrClosedPipe
		}
	}
	stream.mu.Lock()
	defer stream.mu.Unlock()
	select {
	case <-stream.closed:
		return 0, io.ErrClosedPipe
	default:
		return stream.response.Write(body)
	}
}
func (stream *publisherFixture) CloseInput() error {
	stream.finish(connection.CleanClose)
	return nil
}
func (stream *publisherFixture) finish(class connection.OutcomeClass) {
	stream.doneOnce.Do(func() { stream.terminal <- connection.Outcome{Class: class}; close(stream.terminal) })
}
func (stream *publisherFixture) Done() <-chan connection.Outcome { return stream.terminal }
func (stream *publisherFixture) Close() error {
	stream.closeOnce.Do(func() { close(stream.closed); stream.finish(connection.LocalCancellation) })
	return stream.closeErr
}
func (stream *publisherFixture) assertResponse(t *testing.T, body []byte) {
	t.Helper()
	stream.mu.Lock()
	defer stream.mu.Unlock()
	response := stream.response.Bytes()
	if len(response) != len(body)+13 || string(response[:8]) != "ARDTXT01" || response[8] != 0 ||
		binary.BigEndian.Uint32(response[9:13]) != uint32(len(body)) || !bytes.Equal(response[13:], body) {
		t.Fatalf("Publisher response is not the exact snapshot (%d bytes)", len(response))
	}
	select {
	case <-stream.closed:
	default:
		t.Fatal("Publisher returned without closing the Service")
	}
}

func publisherRequest() []byte {
	body := make([]byte, 512)
	copy(body, "ARDTXT01\x01")
	return body
}

func TestPublisherWorkerConnectionsShareSnapshotAndIsolateMalformedRequest(t *testing.T) {
	for _, size := range []int{0, 64 << 10, textdocument.MaximumBytes} {
		t.Run(strconv.Itoa(size)+" bytes", func(t *testing.T) {
			body := bytes.Repeat([]byte("x"), size)
			harness := startPublisherHarness(t, body, nil)
			valid := newPublisherFixture(publisherRequest(), nil)
			malformed := newPublisherFixture(append(publisherRequest(), 1), nil)
			another := newPublisherFixture(publisherRequest(), nil)
			harness.admit(t, valid)
			harness.admit(t, malformed)
			harness.admit(t, another)
			harness.drain(t)
			valid.assertResponse(t, body)
			another.assertResponse(t, body)
			if malformed.response.Len() != 0 {
				t.Fatal("malformed request received snapshot bytes")
			}
		})
	}
}

func TestPublisherWorkerSlowServiceDoesNotBlockOtherStreams(t *testing.T) {
	body := bytes.Repeat([]byte("z"), 160<<10)
	harness := startPublisherHarness(t, body, nil)
	gate := make(chan struct{})
	slow := newPublisherFixture(publisherRequest(), gate)
	fast := newPublisherFixture(publisherRequest(), nil)
	harness.admit(t, slow)
	harness.admit(t, fast)
	select {
	case <-fast.closed:
		fast.assertResponse(t, body)
	case <-time.After(5 * time.Second):
		t.Fatal("one backpressured Service blocked another")
	}
	close(gate)
	harness.drain(t)
	slow.assertResponse(t, body)
}

func TestPublisherWorkerCancellationJoinsBlockedWrites(t *testing.T) {
	harness := startPublisherHarness(t, bytes.Repeat([]byte("a"), 160<<10), nil)
	stream := newPublisherFixture(publisherRequest(), make(chan struct{}))
	harness.admit(t, stream)
	select {
	case <-stream.started:
	case <-time.After(5 * time.Second):
		t.Fatal("Service writer did not start")
	}
	harness.cancel()
	select {
	case <-harness.finished:
		if !errors.Is(harness.err, context.Canceled) {
			t.Fatalf("cancel outcome = %v", harness.err)
		}
		select {
		case <-stream.closed:
		default:
			t.Fatal("cancel returned before Service Close")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Publisher cancellation did not join its blocked writes")
	}
}
