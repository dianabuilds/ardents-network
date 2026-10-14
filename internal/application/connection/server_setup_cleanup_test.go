//go:build linux

package connection

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

type canceledHandoffOwner struct {
	entered chan struct{}
	stream  Stream
}

func (owner canceledHandoffOwner) Open(ctx context.Context, _ Request) (Stream, error) {
	close(owner.entered)
	<-ctx.Done()
	return owner.stream, nil
}

type joinedSetupStream struct {
	cleanupSequenceStream
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (stream *joinedSetupStream) Close() error {
	stream.once.Do(func() { close(stream.entered) })
	<-stream.release
	return stream.failure
}

func TestServerJoinsCanceledSetupCleanup(t *testing.T) {
	for _, failure := range []error{nil, errors.New("private cleanup sentinel")} {
		t.Run(map[bool]string{true: "failure", false: "success"}[failure != nil], func(t *testing.T) {
			stream := &joinedSetupStream{cleanupSequenceStream: cleanupSequenceStream{failure: failure}, entered: make(chan struct{}), release: make(chan struct{})}
			owner := canceledHandoffOwner{entered: make(chan struct{}), stream: stream}
			path := shortClientSocketPath(t)
			public, err := Listen(path, owner)
			if err != nil {
				t.Fatal(err)
			}
			var release sync.Once
			unblock := func() { release.Do(func() { close(stream.release) }) }
			defer public.Close()
			defer unblock()
			peer, err := net.Dial("unix", path)
			if err != nil {
				t.Fatal(err)
			}
			defer peer.Close()
			_ = peer.SetDeadline(time.Now().Add(3 * time.Second))
			raw, err := EncodeRequest(Request{Destination: TargetLink, Value: "explicit"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = peer.Write(raw); err != nil {
				t.Fatal(err)
			}
			select {
			case <-owner.entered:
			case <-time.After(3 * time.Second):
				t.Fatal("setup not entered")
			}
			// An optimistic byte cancels setup while leaving the socket writable for refusal.
			if _, err = peer.Write([]byte{1}); err != nil {
				t.Fatal(err)
			}
			select {
			case <-stream.entered:
			case <-time.After(3 * time.Second):
				t.Fatal("returned Stream was not closed")
			}
			unblock()
			body, err := io.ReadAll(peer)
			if err != nil {
				t.Fatal(err)
			}
			if len(body) == 0 || body[0] != 0 || !strings.Contains(string(body), string(LocalCancellation)) || strings.Contains(string(body), "private cleanup sentinel") {
				t.Fatalf("unsafe or incorrect refusal: %q", body)
			}
			closeErr := public.Close()
			if failure == nil && closeErr != nil || failure != nil && !errors.Is(closeErr, failure) {
				t.Fatalf("Close lost setup cleanup: %v", closeErr)
			}
			if repeated := public.Close(); repeated != closeErr {
				t.Fatalf("Close changed retained result: %v", repeated)
			}
		})
	}
}

type deadlineResetPeer struct {
	wake     chan struct{}
	failure  error
	restored time.Time
}

func (peer *deadlineResetPeer) Read([]byte) (int, error) {
	<-peer.wake
	return 0, &net.DNSError{IsTimeout: true}
}
func (peer *deadlineResetPeer) SetReadDeadline(deadline time.Time) error {
	if deadline.After(time.Now()) {
		peer.restored = deadline
		return peer.failure
	}
	close(peer.wake)
	return nil
}
func (*deadlineResetPeer) Close() error { return nil }

type deadlineSetupOwner struct {
	ctx context.Context
}

func (owner *deadlineSetupOwner) Open(ctx context.Context, _ Request) (Stream, error) {
	owner.ctx = ctx
	return &cleanupSequenceStream{}, nil
}

func TestSetupRetainsOriginalDeadlineAndEarlierCancellation(t *testing.T) {
	for _, earlier := range []bool{false, true} {
		t.Run(map[bool]string{false: "original", true: "earlier-parent"}[earlier], func(t *testing.T) {
			// Simulate time already spent receiving the header without sleeping.
			deadline := time.Now().Add(ReaderTimeout - 12*time.Second)
			parentEnd := deadline.Add(time.Minute)
			if earlier {
				parentEnd = deadline.Add(-time.Second)
			}
			parent, cancelParent := context.WithDeadline(t.Context(), parentEnd)
			defer cancelParent()
			owner := &deadlineSetupOwner{}
			server := &server{ctx: parent, owner: owner}
			peer := &deadlineResetPeer{wake: make(chan struct{})}
			stream, cancel, err := server.openAuthorizedAttachment(peer, Request{}, deadline)
			if err != nil {
				t.Fatal(err)
			}
			defer cancel()
			defer stream.Close()
			want := deadline
			if earlier {
				want = parentEnd
			}
			if got, ok := owner.ctx.Deadline(); !ok || !got.Equal(want) {
				t.Fatalf("setup renewed/lost original deadline: %v, want %v", got, want)
			}
			if !peer.restored.Equal(deadline) {
				t.Fatalf("socket deadline was cleared or renewed: %v", peer.restored)
			}
			cancelParent()
			if !errors.Is(owner.ctx.Err(), context.Canceled) {
				t.Fatalf("returned stream lost original cancellation: %v", owner.ctx.Err())
			}
		})
	}
}

func TestSetupDeadlineResetRetainsAndJoinsCleanup(t *testing.T) {
	setupErr := errors.New("reset sentinel")
	cleanupErr := errors.New("cleanup sentinel")
	owner := &cleanupSequenceOwner{failure: cleanupErr}
	server := &server{ctx: t.Context(), owner: owner}
	// This seam injects a deadline reset failure; it is not a real socket failure.
	attachment, cancel, err := server.openAuthorizedAttachment(&deadlineResetPeer{wake: make(chan struct{}), failure: setupErr}, Request{}, time.Now().Add(ReaderTimeout))
	if attachment != nil || cancel != nil || !errors.Is(err, setupErr) || !errors.Is(err, cleanupErr) {
		t.Fatalf("reset failure lost ownership/error: %v", err)
	}
	if !errors.Is(server.err, cleanupErr) {
		t.Fatalf("cleanup not retained: %v", server.err)
	}
	if outcome := refusal(err); outcome.Class != ServiceUnavailable {
		t.Fatalf("reset refusal = %+v", outcome)
	}
	for range 40 {
		server.retainCleanupFailure(errors.New("later failure"))
	}
	if server.err != cleanupErr {
		t.Fatalf("cleanup history replaced or grew: %v", server.err)
	}
}

func TestServerCloseWaitsForCanceledSetupStream(t *testing.T) {
	failure := errors.New("late setup cleanup")
	stream := &joinedSetupStream{cleanupSequenceStream: cleanupSequenceStream{failure: failure}, entered: make(chan struct{}), release: make(chan struct{})}
	owner := canceledHandoffOwner{entered: make(chan struct{}), stream: stream}
	path := shortClientSocketPath(t)
	public, err := Listen(path, owner)
	if err != nil {
		t.Fatal(err)
	}
	var release sync.Once
	unblock := func() { release.Do(func() { close(stream.release) }) }
	defer public.Close()
	defer unblock()
	peer, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	raw, err := EncodeRequest(Request{Destination: TargetLink, Value: "explicit"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = peer.Write(raw); err != nil {
		t.Fatal(err)
	}
	select {
	case <-owner.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("setup not entered")
	}
	closed := make(chan error, 1)
	go func() { closed <- public.Close() }()
	select {
	case <-stream.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("setup stream cleanup not entered")
	}
	select {
	case err := <-closed:
		t.Fatalf("Close returned before setup cleanup joined: %v", err)
	default:
	}
	unblock()
	select {
	case err := <-closed:
		if !errors.Is(err, failure) {
			t.Fatalf("Close lost joined error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Close did not join setup")
	}
}

func TestSetupCleanupCannotReclassifyRefusal(t *testing.T) {
	for _, setupErr := range []error{context.Canceled, Refuse(Outcome{Class: CapacityUnavailable, Reason: "bounded refusal"}), errors.New("setup sentinel")} {
		for _, cleanupErr := range []error{context.DeadlineExceeded, context.Canceled, errors.New("raw private cleanup")} {
			server := &server{}
			err := server.closeRefusedAttachment(&cleanupSequenceStream{failure: cleanupErr}, setupErr)
			if !errors.Is(err, setupErr) || !errors.Is(err, cleanupErr) {
				t.Fatalf("local errors lost: %v", err)
			}
			if got, want := refusal(err), refusal(setupErr); got != want {
				t.Fatalf("cleanup changed refusal: got %+v want %+v", got, want)
			}
			if server.err != cleanupErr {
				t.Fatalf("cleanup not retained: %v", server.err)
			}
		}
	}
}
