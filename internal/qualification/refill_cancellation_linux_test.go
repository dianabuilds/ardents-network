//go:build linux

package qualification

import (
	"bytes"
	"context"
	"io"
	"net"
	"sync"

	"testing"
	"time"

	applicationconnection "github.com/dianabuilds/ardents-network/internal/application/connection"
	"github.com/dianabuilds/ardents-network/internal/application/streamqualification"
	"github.com/dianabuilds/ardents-network/internal/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/route/client"
)

// These Service/worker attachment fixtures isolate runQualifiedStreams' joined
// stop boundary with a blocking context-aware refill seam. This supplies no
// network admission or successful installed qualification evidence.
type refillCallerStream struct {
	hello  *bytes.Reader
	mu     sync.Mutex
	closed chan struct{}
	once   sync.Once
	result chan applicationconnection.Outcome
	ready  func() error
}

func (stream *refillCallerStream) Read(value []byte) (int, error) {
	stream.mu.Lock()
	if stream.hello.Len() != 0 {
		n, err := stream.hello.Read(value)
		stream.mu.Unlock()
		return n, err
	}
	stream.mu.Unlock()
	<-stream.closed
	return 0, net.ErrClosed
}
func (stream *refillCallerStream) Write(value []byte) (int, error) {
	if len(value) == 1 && value[0] == 1 {
		if err := stream.ready(); err != nil {
			return 0, err
		}
	}
	return len(value), nil
}
func (stream *refillCallerStream) Close() error {
	stream.once.Do(func() {
		close(stream.closed)
		stream.result <- applicationconnection.Outcome{Class: applicationconnection.ServiceUnavailable, Reason: "caller fixture closed"}
		close(stream.result)
	})
	return nil
}
func (stream *refillCallerStream) CloseInput() error                          { return stream.Close() }
func (stream *refillCallerStream) Done() <-chan applicationconnection.Outcome { return stream.result }

type refillCallerWorker struct {
	pending    chan struct{}
	run        *Run
	attachment io.ReadWriteCloser
}

func (worker *refillCallerWorker) Run() *Run                            { return worker.run }
func (worker *refillCallerWorker) Now() time.Time                       { return time.Now() }
func (worker *refillCallerWorker) WorkerAttachment() io.ReadWriteCloser { return worker.attachment }
func (worker *refillCallerWorker) RefillSnapshot() Refill {
	return Refill{Live: true}
}
func (worker *refillCallerWorker) EnsureIssuerReserve(context.Context, int) error { return nil }
func (worker *refillCallerWorker) ReplenishPrefixes(ctx context.Context, present client.ClosedTokenPresenter) error {
	close(worker.pending)
	<-ctx.Done()
	return ctx.Err()
}
func (worker *refillCallerWorker) PresentRefill(_ context.Context, hello ardp.Hello, class uint8) ([]byte, error) {
	return nil, nil
}
func (worker *refillCallerWorker) Close() error { return nil }

// The worker seam observes the context supplied by the real periodic caller.
// Admitted network queue/framing regressions live in route/client; this test
// preserves the caller's independent cancellation and join obligation.
func TestRunQualifiedStreamsCancelsAndJoinsPeriodicRefill(t *testing.T) {
	run, err := NewRun(streamqualification.PublisherRole, streamqualification.ClientToPublisher, [32]byte{1})
	if err != nil {
		t.Fatal(err)
	}
	if err := run.BindInvocation([32]byte{2}); err != nil {
		t.Fatal(err)
	}
	var report streamqualification.Report
	acquire := func(context.Context) (func(), error) { return func() {}, nil }
	if err := run.Configure(&report, acquire, acquire, func() error { return nil }, func(context.Context, streamqualification.Report) error { return nil }); err != nil {
		t.Fatal(err)
	}
	attachment, peer := net.Pipe()
	defer peer.Close()
	worker := &refillCallerWorker{pending: make(chan struct{}), run: run, attachment: attachment}
	var streams []streamqualification.BoundStream
	for index := range 256 {
		stream := &refillCallerStream{hello: bytes.NewReader(nil), closed: make(chan struct{}), result: make(chan applicationconnection.Outcome, 1), ready: func() error { return nil }}
		streams = append(streams, streamqualification.BoundStream{ID: uint32(index*2 + 1), Stream: stream})
	}
	retained, stop := context.WithCancel(t.Context())
	defer stop()
	ended := make(chan error, 1)
	go func() { _, err := runQualifiedStreams(retained, worker, 0, streams); ended <- err }()
	select {
	case <-worker.pending:
	case err := <-ended:
		t.Fatalf("caller ended before refill: %v", err)
	case <-time.After(time.Second):
		t.Fatal("periodic refill did not start")
	}
	_ = peer.Close()
	select {
	case err := <-ended:
		if err == nil {
			t.Fatal("failed attachment became success")
		}
	case <-time.After(time.Second):
		stop()
		<-ended
		t.Fatal("periodic refill was not canceled and joined")
	}
	if retained.Err() != nil {
		t.Fatal("caller canceled retained authority")
	}
}
