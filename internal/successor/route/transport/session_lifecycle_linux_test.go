//go:build linux

package transport

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

// lifecycleConn gates actual physical output and completion independently.
// It supplies no successful authority, Admission, or resource-transfer result.
type lifecycleConn struct {
	mu                    sync.Mutex
	closed                chan struct{}
	closeOnce             sync.Once
	writeGate             chan struct{}
	writes                chan []byte
	output                []byte
	writeEnd              time.Time
	partial, closeFailure error
	input                 *bytes.Reader
	readGate              chan struct{}
	writeIgnoresClose     bool
}

func newLifecycleConn(gated bool) *lifecycleConn {
	c := &lifecycleConn{closed: make(chan struct{}), writeGate: make(chan struct{}), writes: make(chan []byte, 32)}
	if !gated {
		close(c.writeGate)
	}
	return c
}
func (c *lifecycleConn) Read(p []byte) (int, error) {
	if c.input != nil && c.input.Len() > 0 {
		return c.input.Read(p)
	}
	if c.readGate != nil {
		<-c.readGate
	}
	<-c.closed
	return 0, net.ErrClosed
}
func (c *lifecycleConn) Write(p []byte) (int, error) {
	c.writes <- append([]byte(nil), p...)
	if c.writeIgnoresClose {
		<-c.writeGate
	} else {
		select {
		case <-c.writeGate:
		case <-c.closed:
			return 0, net.ErrClosed
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.partial != nil {
		c.output = append(c.output, p[:3]...)
		return 3, c.partial
	}
	c.output = append(c.output, p...)
	return len(p), nil
}
func (c *lifecycleConn) Close() error {
	c.closeOnce.Do(func() { close(c.closed) })
	return c.closeFailure
}
func (c *lifecycleConn) LocalAddr() net.Addr             { return &net.TCPAddr{} }
func (c *lifecycleConn) RemoteAddr() net.Addr            { return &net.TCPAddr{} }
func (c *lifecycleConn) SetDeadline(t time.Time) error   { return c.SetWriteDeadline(t) }
func (c *lifecycleConn) SetReadDeadline(time.Time) error { return nil }
func (c *lifecycleConn) SetWriteDeadline(t time.Time) error {
	c.mu.Lock()
	c.writeEnd = t
	c.mu.Unlock()
	return nil
}

// A closed socket rejects deadline changes even while an original physical
// writer has not yet returned. This matches the TCP retirement boundary.
type retiredDeadlineConn struct{ *lifecycleConn }

func (c *retiredDeadlineConn) SetWriteDeadline(end time.Time) error {
	select {
	case <-c.closed:
		return net.ErrClosed
	default:
		return c.lifecycleConn.SetWriteDeadline(end)
	}
}

func TestSessionLifecycleLaneCloseAfterParentRetirementDoesNotResetDeadline(t *testing.T) {
	physical := &retiredDeadlineConn{newLifecycleConn(true)}
	physical.writeIgnoresClose = true
	late := errors.New("original writer late physical failure")
	physical.partial = late
	s := newSession(t.Context(), physical, time.Now().Add(5*time.Second), 32<<20, nil, false, &queueBudget{maximum: 64 << 20}, nil)
	defer s.Close()
	l := lifecycleLane(t, s, 1)
	written := make(chan error, 1)
	go func() { _, err := l.Write([]byte("original output")); written <- err }()
	select {
	case <-physical.writes:
	case <-time.After(time.Second):
		t.Fatal("original writer did not start")
	}
	authority := errors.New("original authority retired")
	s.retire(authority)
	closed := make(chan error, 1)
	go func() { closed <- l.Close() }()
	// Close takes the lane lock before joining its already selected writer.
	deadline := time.Now().Add(time.Second)
	for {
		s.mu.Lock()
		closing := l.localClosed
		s.mu.Unlock()
		if closing {
			break
		}
		if !time.Now().Before(deadline) {
			t.Fatal("lane close did not start")
		}
		runtime.Gosched()
	}
	close(physical.writeGate)
	if err := lifecycleResult(t, written); !errors.Is(err, late) {
		t.Fatal("original physical error lost", err)
	}
	if err := lifecycleResult(t, closed); err != nil {
		t.Fatal("retired parent caused a new deadline operation", err)
	}
	if err := s.Close(); !errors.Is(err, authority) || !errors.Is(err, late) || errors.Is(err, net.ErrClosed) {
		t.Fatal("joined result lost original provenance or added closed-socket deadline failure", err)
	}
}

func lifecycleLane(t *testing.T, s *session, id uint32) *lane {
	t.Helper()
	if !s.queues.child() {
		t.Fatal("test child capacity unavailable")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.newLaneLocked(id)
}
func lifecycleResult(t *testing.T, ch <-chan error) error {
	t.Helper()
	select {
	case err := <-ch:
		return err
	case <-time.After(2 * time.Second):
		t.Fatal("owned operation did not complete")
		return nil
	}
}
func lifecycleWriterEntered(t *testing.T, l *lane) {
	t.Helper()
	end := time.Now().Add(2 * time.Second)
	for time.Now().Before(end) {
		if !l.writeMu.TryLock() {
			return
		}
		l.writeMu.Unlock()
		runtime.Gosched()
	}
	t.Fatal("lane did not enter its queued write")
}

func TestSessionLifecycleCloseRechecksParentAfterWriterJoin(t *testing.T) {
	physical := newLifecycleConn(true)
	physical.writeIgnoresClose = true
	s := newSession(t.Context(), physical, time.Now().Add(time.Minute), 32<<20, nil, false, &queueBudget{maximum: 64 << 20}, nil)
	defer s.Close()
	var gateOnce sync.Once
	releaseWriter := func() { gateOnce.Do(func() { close(physical.writeGate) }) }
	defer releaseWriter()
	l := lifecycleLane(t, s, 1)
	l.openEmitted = true
	written := make(chan error, 1)
	go func() { _, err := l.Write([]byte("accepted writer")); written <- err }()
	select {
	case <-physical.writes:
	case <-time.After(2 * time.Second):
		t.Fatal("accepted physical writer did not start")
	}
	closed := make(chan error, 1)
	go func() { closed <- l.Close() }()
	closing := false
	until := time.Now().Add(2 * time.Second)
	for time.Now().Before(until) {
		s.mu.Lock()
		closing = l.localClosed
		s.mu.Unlock()
		if closing {
			break
		}
		runtime.Gosched()
	}
	if !closing {
		t.Fatal("Close did not begin joining the accepted writer")
	}
	parentFailure := errors.New("original parent retired during writer join")
	s.retire(parentFailure)
	releaseWriter()
	if err := lifecycleResult(t, written); err != nil {
		t.Fatal("accepted late writer result changed", err)
	}
	if err := lifecycleResult(t, closed); err != nil {
		t.Fatal("joined local Close attempted another frame on retired parent", err)
	}
	if err := s.Close(); !errors.Is(err, parentFailure) {
		t.Fatal("local join erased original parent failure", err)
	}
	select {
	case frame := <-physical.writes:
		t.Fatal("retired parent received a new terminal frame", frame)
	default:
	}
}

func TestSessionLifecycleQueuedRetirementPreservesSibling(t *testing.T) {
	for _, mode := range []string{"deadline", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			physical := newLifecycleConn(true)
			queues := &queueBudget{maximum: 64 << 20}
			s := newSession(context.Background(), physical, time.Now().Add(5*time.Second), 32<<20, nil, false, queues, nil)
			defer s.Close()
			first, queued, sibling := lifecycleLane(t, s, 1), lifecycleLane(t, s, 3), lifecycleLane(t, s, 5)
			firstDone := make(chan error, 1)
			go func() { _, err := first.Write([]byte("first")); firstDone <- err }()
			select {
			case <-physical.writes:
			case <-time.After(2 * time.Second):
				t.Fatal("physical writer did not start")
			}
			queuedDone := make(chan error, 1)
			go func() { _, err := queued.Write([]byte("must not emit")); queuedDone <- err }()
			lifecycleWriterEntered(t, queued)
			if mode == "deadline" {
				queued.SetWriteDeadline(time.Now().Add(-time.Second))
			} else {
				if err := queued.Close(); err != nil {
					t.Fatal(err)
				}
			}
			err := lifecycleResult(t, queuedDone)
			if mode == "deadline" && !errors.Is(err, os.ErrDeadlineExceeded) || mode == "cancel" && !errors.Is(err, net.ErrClosed) {
				t.Fatalf("wrong queued refusal: %v", err)
			}
			s.mu.Lock()
			stopped, credit := s.stopped, queued.credit
			s.mu.Unlock()
			if stopped || credit != window {
				t.Fatalf("unemitted work poisoned parent=%v or consumed credit=%d", stopped, credit)
			}
			physical.mu.Lock()
			deadline := physical.writeEnd
			physical.mu.Unlock()
			if deadline != first.end {
				t.Fatal("queued retirement changed the active sibling deadline")
			}
			close(physical.writeGate)
			if err := lifecycleResult(t, firstDone); err != nil {
				t.Fatal(err)
			}
			if _, err := sibling.Write([]byte("sibling")); err != nil {
				t.Fatal(err)
			}
			physical.mu.Lock()
			output := append([]byte(nil), physical.output...)
			physical.mu.Unlock()
			reader := bytes.NewReader(output)
			for _, want := range []uint32{1, 5} {
				frame, err := ardp.ReadFrame(reader)
				if err != nil || frame.Lane != want || frame.Kind != ardp.KindBytes {
					t.Fatalf("unexpected emitted lane/frame: %+v %v", frame, err)
				}
			}
			if reader.Len() != 0 {
				t.Fatal("cancelled output or unknown-lane CLOSE was emitted")
			}
		})
	}
}

func TestSessionLifecyclePartialFramePoisonsPhysicalOwner(t *testing.T) {
	poison := errors.New("injected partial physical frame")
	physical := newLifecycleConn(false)
	physical.partial = poison
	s := newSession(context.Background(), physical, time.Now().Add(5*time.Second), 32<<20, nil, false, &queueBudget{maximum: 64 << 20}, nil)
	first, sibling := lifecycleLane(t, s, 1), lifecycleLane(t, s, 3)
	if _, err := first.Write([]byte("charged payload")); !errors.Is(err, poison) {
		t.Fatalf("partial failure lost: %v", err)
	}
	if _, err := sibling.Write([]byte("forbidden")); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("poisoned sibling remained usable: %v", err)
	}
	s.mu.Lock()
	credit := first.credit
	s.mu.Unlock()
	if credit != window-uint32(len("charged payload")) {
		t.Fatal("failed physical frame refunded payload credit")
	}
	if err := s.Close(); !errors.Is(err, poison) {
		t.Fatalf("joined close lost physical poison: %v", err)
	}
	if err := s.Close(); !errors.Is(err, poison) {
		t.Fatalf("repeated close replaced physical poison: %v", err)
	}
}

func TestSessionLifecycleCloseWaitsForLatePhysicalWriterFailure(t *testing.T) {
	late := errors.New("late started physical writer failed")
	physical := newLifecycleConn(true)
	physical.writeIgnoresClose = true
	physical.partial = late
	queues := &queueBudget{maximum: 64 << 20}
	release, err := queues.channel()
	if err != nil {
		t.Fatal(err)
	}
	s := newSession(context.Background(), physical, time.Now().Add(5*time.Second), 32<<20, nil, false, queues, nil)
	l := lifecycleLane(t, s, 1)
	written := make(chan error, 1)
	go func() { _, err := l.Write([]byte("started payload")); written <- err }()
	select {
	case <-physical.writes:
	case <-time.After(2 * time.Second):
		t.Fatal("physical frame never started")
	}
	joined := make(chan error, 1)
	go func() { err := s.Close(); release(); joined <- err }()
	<-physical.closed
	select {
	case err := <-joined:
		t.Fatalf("Close completed before physical writer joined: %v", err)
	default:
	}
	queues.mu.Lock()
	held := queues.used
	queues.mu.Unlock()
	if held != (16<<10)+uint64(ardp.HeaderSize+len("started payload")) {
		t.Fatal("control reservation released before physical writer joined")
	}
	close(physical.writeGate)
	if err := lifecycleResult(t, written); !errors.Is(err, late) {
		t.Fatalf("started writer lost its error: %v", err)
	}
	if err := lifecycleResult(t, joined); !errors.Is(err, late) {
		t.Fatalf("joined Close suppressed late physical error: %v", err)
	}
	if err := s.Close(); !errors.Is(err, late) {
		t.Fatalf("repeated Close replaced late physical error: %v", err)
	}
}

func TestSessionLifecycleCreditIgnoresCompletedPayloadDeadline(t *testing.T) {
	local, peer := net.Pipe()
	defer peer.Close()
	s := newSession(context.Background(), local, time.Now().Add(5*time.Second), 32<<20, nil, false, &queueBudget{maximum: 64 << 20}, nil)
	defer s.Close()
	l := lifecycleLane(t, s, 1)
	if err := l.SetWriteDeadline(time.Now().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() {
		if err := ardp.WriteFrame(peer, ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: []byte("kept")}); err != nil {
			result <- err
			return
		}
		f, err := ardp.ReadFrame(peer)
		if err == nil && (f.Kind != ardp.KindCredit || f.Lane != 1 || !bytes.Equal(f.Body, []byte{0, 0, 0, 4})) {
			err = errors.New("wrong credit frame")
		}
		result <- err
	}()
	value := make([]byte, 4)
	if n, err := l.Read(value); n != 4 || err != nil || string(value) != "kept" {
		t.Fatalf("live consumption failed after payload expiry: %q %d %v", value, n, err)
	}
	if err := lifecycleResult(t, result); err != nil {
		t.Fatal(err)
	}
}

func TestSessionLifecycleExpiredChildCloseDoesNotRetireParent(t *testing.T) {
	local, peer := net.Pipe()
	defer peer.Close()
	s := newSession(context.Background(), local, time.Now().Add(time.Minute), 32<<20, nil, false, &queueBudget{maximum: 64 << 20}, nil)
	defer s.Close()
	expired := lifecycleLane(t, s, 1)
	live := lifecycleLane(t, s, 3)
	s.mu.Lock()
	expired.end = time.Now().Add(-time.Second)
	expired.readEnd, expired.writeEnd = expired.end, expired.end
	expired.openEmitted = true
	s.mu.Unlock()
	if err := expired.Close(); err != nil {
		t.Fatal("expired child attempted terminal output beyond its original bound", err)
	}
	if _, err := expired.Read(make([]byte, 1)); !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatal("expired child lost its refusal", err)
	}
	written := make(chan error, 1)
	go func() { _, err := live.Write([]byte("sibling")); written <- err }()
	frame, err := ardp.ReadFrame(peer)
	if err != nil || frame.Kind != ardp.KindBytes || frame.Lane != 3 || string(frame.Body) != "sibling" {
		t.Fatal("expired child poisoned original live sibling", frame, err)
	}
	if err := lifecycleResult(t, written); err != nil {
		t.Fatal(err)
	}
}

func TestSessionLifecycleCallerDeadlineDeniesNewTerminalOutput(t *testing.T) {
	physical := newLifecycleConn(false)
	s := newSession(context.Background(), physical, time.Now().Add(time.Minute), 32<<20, nil, false, &queueBudget{maximum: 64 << 20}, nil)
	defer s.Close()
	l := lifecycleLane(t, s, 1)
	caller, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	l.caller, l.openEmitted = caller, true
	// The original timer has expired. A still-future wall reading cannot
	// resurrect its output authority or manufacture an extra CLOSE attempt.
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case frame := <-physical.writes:
		t.Fatal("expired original caller emitted new terminal traffic", frame)
	default:
	}
}

func TestSessionLifecycleOriginalReadExpiryRetainsDeadlineCause(t *testing.T) {
	physical := newLifecycleConn(false)
	s := newSession(t.Context(), physical, time.Now().Add(time.Minute), 32<<20, nil, false, &queueBudget{maximum: 64 << 20}, nil)
	defer s.Close()
	l := lifecycleLane(t, s, 1)
	l.openEmitted = true
	l.end = time.Now().Add(20 * time.Millisecond)
	l.readEnd, l.writeEnd = l.end, l.end
	if _, err := l.Read(make([]byte, 1)); !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatal("original read did not expire", err)
	}
	if context.Cause(l.ctx) != os.ErrDeadlineExceeded {
		t.Fatal("original expiry lost its cause before downstream cancellation", context.Cause(l.ctx))
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case frame := <-physical.writes:
		t.Fatal("expired read initiated terminal output", frame)
	default:
	}
}

func TestSessionLifecycleShortReadDeadlineDoesNotExpireOriginalLane(t *testing.T) {
	physical := newLifecycleConn(false)
	s := newSession(t.Context(), physical, time.Now().Add(time.Minute), 32<<20, nil, false, &queueBudget{maximum: 64 << 20}, nil)
	defer s.Close()
	l := lifecycleLane(t, s, 1)
	l.openEmitted = true
	if err := l.SetReadDeadline(time.Now().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Read(make([]byte, 1)); !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatal("selected short read did not expire", err)
	}
	if context.Cause(l.ctx) != nil {
		t.Fatal("short local read deadline expired original lane", context.Cause(l.ctx))
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	frame, err := ardp.ReadFrame(bytes.NewReader(<-physical.writes))
	if err != nil || frame.Kind != ardp.KindClose || frame.Lane != l.id {
		t.Fatal("short read suppressed still-permitted terminal cleanup", frame, err)
	}
}

func TestSessionLifecycleQueuedCloseExpiryPreservesParentWriter(t *testing.T) {
	physical := newLifecycleConn(true)
	s := newSession(context.Background(), physical, time.Now().Add(time.Minute), 32<<20, nil, false, &queueBudget{maximum: 64 << 20}, nil)
	defer s.Close()
	active := lifecycleLane(t, s, 1)
	expiring := lifecycleLane(t, s, 3)
	expiring.openEmitted = true
	if err := expiring.bound(time.Now().Add(150 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	written := make(chan error, 1)
	go func() { _, err := active.Write([]byte("live sibling")); written <- err }()
	<-physical.writes
	closed := make(chan error, 1)
	go func() { closed <- expiring.Close() }()
	closeErr := lifecycleResult(t, closed)
	close(physical.writeGate)
	if closeErr != nil {
		t.Fatal("unemitted expired CLOSE became parent failure", closeErr)
	}
	if err := lifecycleResult(t, written); err != nil {
		t.Fatal("expiry interrupted live sibling writer", err)
	}
	s.mu.Lock()
	stopped := s.stopped
	s.mu.Unlock()
	if stopped {
		t.Fatal("queued expiry retired framing parent")
	}
	select {
	case frame := <-physical.writes:
		t.Fatal("queued CLOSE emitted after its original expiry", frame)
	default:
	}
}

func TestSessionLifecyclePhysicalCloseTimeoutIsNotQueueExpiry(t *testing.T) {
	physical := newLifecycleConn(false)
	physical.partial = os.ErrDeadlineExceeded
	s := newSession(context.Background(), physical, time.Now().Add(time.Minute), 32<<20, nil, false, &queueBudget{maximum: 64 << 20}, nil)
	l := lifecycleLane(t, s, 1)
	l.openEmitted = true
	if err := l.Close(); !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatal("started physical timeout was discarded", err)
	}
	if err := s.Close(); !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatal("parent lost physical terminal failure", err)
	}
}

func TestSessionLifecycleCloseJoinsBeforeResourceRelease(t *testing.T) {
	physical := newLifecycleConn(false)
	physical.readGate = make(chan struct{})
	closeFailure := errors.New("physical Close failed")
	physical.closeFailure = closeFailure
	openRaw, err := ardp.EncodeFrame(ardp.Frame{Kind: ardp.KindOpen, Lane: 1, Body: make([]byte, 50)})
	if err != nil {
		t.Fatal(err)
	}
	physical.input = bytes.NewReader(openRaw)
	queues := &queueBudget{maximum: 64 << 20}
	release, err := queues.channel()
	if err != nil {
		t.Fatal(err)
	}
	childEntered, childGate := make(chan struct{}), make(chan struct{})
	s := newSession(context.Background(), physical, time.Now().Add(5*time.Second), 32<<20, nil, false, queues, func(context.Context, *lane, []byte) error { close(childEntered); <-childGate; return nil })
	select {
	case <-childEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("owned child did not start")
	}
	joined := make(chan error, 1)
	go func() { err := s.Close(); release(); joined <- err }()
	<-physical.closed
	select {
	case err := <-joined:
		t.Fatalf("returned before reader join: %v", err)
	default:
	}
	queues.mu.Lock()
	used, children := queues.used, queues.children
	queues.mu.Unlock()
	if used != 16<<10 || children != 1 {
		t.Fatalf("physical reservation released before join: used=%d children=%d", used, children)
	}
	close(physical.readGate)
	select {
	case <-s.readerDone:
	case <-time.After(2 * time.Second):
		t.Fatal("reader did not join")
	}
	select {
	case err := <-joined:
		t.Fatalf("returned before child join: %v", err)
	default:
	}
	close(childGate)
	if err := lifecycleResult(t, joined); !errors.Is(err, closeFailure) {
		t.Fatalf("retained close failure lost: %v", err)
	}
	queues.mu.Lock()
	used, children = queues.used, queues.children
	queues.mu.Unlock()
	if used != 0 || children != 0 {
		t.Fatalf("joined reservation not released: used=%d children=%d", used, children)
	}
}

func TestSessionLifecycleControlAndRetainedPayload(t *testing.T) {
	queues := &queueBudget{maximum: (16 << 10) + 4}
	release, err := queues.channel()
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	local, peer := net.Pipe()
	defer peer.Close()
	s := newSession(context.Background(), local, time.Now().Add(5*time.Second), 32<<20, nil, false, queues, nil)
	defer s.Close()
	l := lifecycleLane(t, s, 1)
	result := make(chan error, 1)
	go func() {
		err := ardp.WriteFrame(peer, ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: []byte("held")})
		if err == nil {
			err = ardp.WriteFrame(peer, ardp.Frame{Kind: ardp.KindClose, Lane: 1, Body: []byte{0}})
		}
		result <- err
	}()
	if err := lifecycleResult(t, result); err != nil {
		t.Fatal(err)
	}
	end := time.Now().Add(2 * time.Second)
	for {
		s.mu.Lock()
		closed := l.peerClosed
		s.mu.Unlock()
		if closed {
			break
		}
		if !time.Now().Before(end) {
			t.Fatal("peer CLOSE not processed")
		}
		runtime.Gosched()
	}
	queues.mu.Lock()
	used := queues.used
	queues.mu.Unlock()
	if used != (16<<10)+4 {
		t.Fatalf("CLOSE refunded retained payload: %d", used)
	}
	if queues.reserve(1) {
		t.Fatal("data borrowed the reserved control capacity")
	}
	if _, err := queues.channel(); err == nil {
		t.Fatal("second channel admitted without its control reserve")
	}
	value := make([]byte, 4)
	if _, err := io.ReadFull(l, value); err != nil || string(value) != "held" {
		t.Fatalf("retained payload lost: %q %v", value, err)
	}
	queues.mu.Lock()
	used = queues.used
	queues.mu.Unlock()
	if used != 16<<10 {
		t.Fatalf("consumption failed to release payload: %d", used)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	release()
	queues.mu.Lock()
	used = queues.used
	queues.mu.Unlock()
	if used != 0 {
		t.Fatalf("final control reservation retained: %d", used)
	}
	// Child exhaustion does not occupy or borrow control bytes.
	other := &queueBudget{maximum: 16 << 10}
	for range 1024 {
		if !other.child() {
			t.Fatal("child capacity ended prematurely")
		}
	}
	if other.child() {
		t.Fatal("child bound not enforced")
	}
	free, err := other.channel()
	if err != nil {
		t.Fatalf("full child capacity starved control: %v", err)
	}
	free()
}
