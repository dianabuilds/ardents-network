//go:build linux

package connection

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

func TestSuccessfulAcceptSurvivesDelayedEmitterCompletion(t *testing.T) {
	parent, cancel := context.WithCancel(t.Context())
	defer cancel()
	lifetime := newOpening(parent)
	defer lifetime.close()
	emitter, peer := net.Pipe()
	defer emitter.Close()
	defer peer.Close()
	if err := emitter.SetWriteDeadline(lifetime.end); err != nil {
		t.Fatal(err)
	}
	if err := peer.SetReadDeadline(lifetime.end.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() {
		result <- lifetime.accept(func() error {
			if _, err := emitter.Write([]byte{1}); err != nil {
				return err
			}
			// The peer has observed ACCEPT; delay the emitter's resumption
			// beyond the setup deadline, as scheduler contention can do.
			timer := time.NewTimer(time.Until(lifetime.end.Add(100 * time.Millisecond)))
			defer timer.Stop()
			<-timer.C
			return nil
		})
	}()
	var status [1]byte
	if _, err := io.ReadFull(peer, status[:]); err != nil {
		t.Fatal(err)
	}
	if status[0] != 1 {
		t.Fatal("peer did not observe ACCEPT")
	}
	if err := <-result; err != nil {
		t.Fatalf("observed ACCEPT lost to opening timer: %v", err)
	}
	if err := lifetime.ctx.Err(); err != nil {
		t.Fatalf("opening timer canceled accepted lifetime: %v", err)
	}
	cancel()
	if !errors.Is(context.Cause(lifetime.ctx), context.Canceled) {
		t.Fatal("accepted lifetime lost caller cancellation")
	}
}

type acceptedLifetimeOwner struct {
	entered chan context.Context
	stream  *acceptedLifetimeStream
	blocked bool
}

func (owner *acceptedLifetimeOwner) Open(ctx context.Context, _ Request) (Stream, error) {
	if owner.stream != nil {
		owner.stream.ctx = ctx
	}
	owner.entered <- ctx
	if owner.blocked {
		<-ctx.Done()
		if owner.stream == nil {
			return nil, ctx.Err()
		}
	}
	return owner.stream, nil
}

func TestOpeningTimeoutCancelsOwner(t *testing.T) {
	owner := &acceptedLifetimeOwner{entered: make(chan context.Context, 1), blocked: true}
	path := shortClientSocketPath(t)
	server, err := Listen(path, owner)
	if err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	joined := false
	go func() {
		opened, err := Dial(t.Context(), path, Request{Destination: TargetLink, Value: "ardents-target:v1:blocked-opening"})
		if opened != nil {
			err = errors.Join(err, opened.Close(), errors.New("timed out Open returned a Client"))
		}
		result <- err
	}()
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Errorf("server cleanup: %v", err)
		}
		if joined {
			return
		}
		select {
		case <-result:
		case <-time.After(time.Second):
			t.Error("Dial cleanup did not join")
		}
	})
	var entered context.Context
	select {
	case entered = <-owner.entered:
	case <-time.After(time.Second):
		t.Fatal("Open did not enter")
	}
	timer := time.NewTimer(11 * time.Second)
	defer timer.Stop()
	select {
	case <-entered.Done():
		if !errors.Is(context.Cause(entered), context.DeadlineExceeded) {
			t.Fatalf("opening lost timeout cause: %v", context.Cause(entered))
		}
	case <-timer.C:
		t.Fatal("Open outlived the whole ten-second budget")
	}
	select {
	case err := <-result:
		joined = true
		var refusal SetupRefusalError
		if !errors.Is(err, context.DeadlineExceeded) && (!errors.As(err, &refusal) || refusal.Outcome().Class != LocalTimeout) {
			t.Fatalf("Dial lost bounded timeout outcome: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out Dial did not join")
	}
}

func TestOpeningTimeoutRejectsLateStream(t *testing.T) {
	stream := &acceptedLifetimeStream{data: make(chan byte, 1), closed: make(chan struct{}), done: make(chan Outcome, 1)}
	owner := &acceptedLifetimeOwner{entered: make(chan context.Context, 1), blocked: true, stream: stream}
	path := shortClientSocketPath(t)
	server, err := Listen(path, owner)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Errorf("server cleanup: %v", err)
		}
	})
	peer, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	cleanupUnixConnection(t, peer)
	if err := peer.SetDeadline(time.Now().Add(13 * time.Second)); err != nil {
		t.Fatal(err)
	}
	request, err := EncodeRequest(Request{Destination: TargetLink, Value: "ardents-target:v1:late-opening"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := peer.Write(request); err != nil {
		t.Fatal(err)
	}
	var status [1]byte
	if _, err := io.ReadFull(peer, status[:]); err != nil || status[0] != 0 {
		t.Fatalf("late Stream received ACCEPT or no bounded refusal: %d / %v", status[0], err)
	}
	outcome, err := readRefusal(peer)
	if err != nil || outcome.Class != LocalTimeout {
		t.Fatalf("late Stream refusal: %+v / %v", outcome, err)
	}
	select {
	case <-stream.closed:
	default:
		t.Fatal("late Stream cleanup was not joined before refusal")
	}
}

type acceptedLifetimeStream struct {
	ctx    context.Context
	data   chan byte
	closed chan struct{}
	done   chan Outcome
	once   sync.Once
}

func (stream *acceptedLifetimeStream) Read(body []byte) (int, error) {
	if len(body) == 0 {
		return 0, nil
	}
	select {
	case value := <-stream.data:
		body[0] = value
		return 1, nil
	case <-stream.ctx.Done():
		_ = stream.Close()
		return 0, io.EOF
	case <-stream.closed:
		return 0, io.EOF
	}
}

func (*acceptedLifetimeStream) Write(body []byte) (int, error) { return len(body), nil }
func (*acceptedLifetimeStream) CloseInput() error              { return nil }
func (stream *acceptedLifetimeStream) Done() <-chan Outcome    { return stream.done }
func (stream *acceptedLifetimeStream) Close() error {
	stream.once.Do(func() {
		close(stream.closed)
		stream.done <- Outcome{Class: LocalCancellation}
		close(stream.done)
	})
	return nil
}

func TestAcceptedStreamOutlivesOpeningTimer(t *testing.T) {
	stream := &acceptedLifetimeStream{data: make(chan byte, 1), closed: make(chan struct{}), done: make(chan Outcome, 1)}
	owner := &acceptedLifetimeOwner{entered: make(chan context.Context, 1), stream: stream}
	path := shortClientSocketPath(t)
	server, err := Listen(path, owner)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Errorf("server cleanup: %v", err)
		}
	})
	opened, err := Dial(t.Context(), path, Request{Destination: TargetLink, Value: "ardents-target:v1:accepted-lifetime"})
	if err != nil {
		t.Fatal(err)
	}
	cleanupClient(t, opened)
	accepted := <-owner.entered
	// Cross the original real 15-second setup timer without changing the
	// owner or clock. The accepted Stream must still carry actual bytes.
	timer := time.NewTimer(15200 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-accepted.Done():
		t.Fatalf("accepted Stream retained the opening timer: %v", accepted.Err())
	case <-timer.C:
	case <-t.Context().Done():
		t.Fatal("accepted lifetime observation interrupted")
	}
	stream.data <- 'x'
	var actual [1]byte
	if _, err := io.ReadFull(opened, actual[:]); err != nil || actual[0] != 'x' {
		t.Fatalf("accepted Stream lost data after setup window: %q / %v", actual, err)
	}
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-accepted.Done():
	default:
		t.Fatal("server Close lost accepted lifetime cancellation")
	}
}

func TestOpeningBudgetIncludesPartialRequest(t *testing.T) {
	owner := &acceptedLifetimeOwner{entered: make(chan context.Context, 1), blocked: true}
	path := shortClientSocketPath(t)
	server, err := Listen(path, owner)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Errorf("server cleanup: %v", err)
		}
	})
	peer, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: path, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	cleanupUnixConnection(t, peer)
	if err := peer.SetDeadline(time.Now().Add(12 * time.Second)); err != nil {
		t.Fatal(err)
	}
	request, err := EncodeRequest(Request{Destination: TargetLink, Value: "ardents-target:v1:partial-opening"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := peer.Write(request[:1]); err != nil {
		t.Fatal(err)
	}
	timer := time.NewTimer(8 * time.Second)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-t.Context().Done():
		t.Fatal("partial request observation interrupted")
	}
	if _, err := peer.Write(request[1:]); err != nil {
		t.Fatal(err)
	}
	var entered context.Context
	select {
	case entered = <-owner.entered:
	case <-time.After(time.Second):
		t.Fatal("partial request never reached Open")
	}
	select {
	case <-entered.Done():
		if !errors.Is(context.Cause(entered), context.DeadlineExceeded) {
			t.Fatalf("partial request timeout cause: %v", context.Cause(entered))
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Endpoint work reset the request's opening budget")
	}
	var status [1]byte
	if _, err := io.ReadFull(peer, status[:]); err != nil || status[0] != 0 {
		t.Fatalf("partial request received late ACCEPT: %d / %v", status[0], err)
	}
	outcome, err := readRefusal(peer)
	if err != nil || outcome.Class != LocalTimeout {
		t.Fatalf("partial request lost timeout outcome: %+v / %v", outcome, err)
	}
}

func TestAcceptedStreamRetainsCallerCancellation(t *testing.T) {
	stream := &acceptedLifetimeStream{data: make(chan byte, 1), closed: make(chan struct{}), done: make(chan Outcome, 1)}
	owner := &acceptedLifetimeOwner{entered: make(chan context.Context, 1), stream: stream}
	path := shortClientSocketPath(t)
	server, err := Listen(path, owner)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Errorf("server cleanup: %v", err)
		}
	})
	caller, cancel := context.WithCancel(t.Context())
	defer cancel()
	opened, err := Dial(caller, path, Request{Destination: TargetLink, Value: "ardents-target:v1:accepted-cancellation"})
	if err != nil {
		t.Fatal(err)
	}
	cleanupClient(t, opened)
	accepted := <-owner.entered
	cancel()
	select {
	case <-stream.closed:
	case <-time.After(time.Second):
		t.Fatal("accepted Stream lost caller cancellation")
	}
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	if accepted.Err() == nil {
		t.Fatal("accepted lifetime was not joined after caller loss")
	}
}
