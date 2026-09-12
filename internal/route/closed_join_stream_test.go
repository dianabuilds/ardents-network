//go:build linux

package route

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"net"
	"sync"
	"testing"
	"time"
)

// The stream tests use real loopback TCP carrying canonical frames and actual
// admission/pair governors. Role TLS authentication and Node selection remain
// the listener integration's separate obligation.
func closedJoinTCP(t *testing.T) (net.Conn, net.Conn) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := listener.Close(); err != nil {
			t.Error(err)
		}
	}()
	client, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	server, err := listener.Accept()
	if err != nil {
		client.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, connection := range []net.Conn{client, server} {
			if err := connection.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				t.Error(err)
			}
		}
	})
	if err := client.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	return client, server
}

type closedJoinStreamFixture struct {
	pair    *closedJoinFixture
	sides   [2]*ClosedJoinSide
	clients [2]net.Conn
	results [2]chan error
	cancel  context.CancelFunc
}

func newClosedJoinStreamFixture(t *testing.T) *closedJoinStreamFixture {
	return newClosedJoinStreamWithConnection(t, nil)
}

func newClosedJoinStreamWithConnection(t *testing.T, wrap func(net.Conn) net.Conn) *closedJoinStreamFixture {
	t.Helper()
	f := &closedJoinStreamFixture{pair: newClosedJoinFixture(t)}
	ctx, cancel := context.WithCancel(t.Context())
	f.cancel = cancel
	t.Cleanup(cancel)
	for i := range f.sides {
		var err error
		f.sides[i], err = f.pair.join(t, uint8(i+1), 21, 10)
		if err != nil {
			t.Fatal(err)
		}
		client, server := closedJoinTCP(t)
		if i == 0 && wrap != nil {
			server = wrap(server)
		}
		f.clients[i] = client
		f.results[i] = make(chan error, 1)
		go func() { f.results[i] <- f.sides[i].Serve(ctx, server) }()
	}
	for i, client := range f.clients {
		result, err := ReadClosedLaneFrame(client)
		if err != nil {
			t.Fatal(err)
		}
		if result.Kind != closedFrameResult || result.Lane != 1 {
			t.Fatal("missing local JOIN result")
		}
		if status, err := DecodeClosedJoinResult(result.Body, f.sides[i].nonce); err != nil || status != 0 {
			t.Fatal("wrong local JOIN result")
		}
	}
	return f
}

func (f *closedJoinStreamFixture) transfer(t *testing.T, from int, frame ClosedLaneFrame) {
	t.Helper()
	if err := WriteClosedLaneFrame(f.clients[from], frame); err != nil {
		t.Fatal(err)
	}
	received, err := ReadClosedLaneFrame(f.clients[1-from])
	if err != nil {
		t.Fatal(err)
	}
	if received.Kind != frame.Kind || received.Lane != 1 || !bytes.Equal(received.Body, frame.Body) {
		t.Fatal("relayed frame differs")
	}
}

func (f *closedJoinStreamFixture) joined(t *testing.T) {
	t.Helper()
	for _, result := range f.results {
		select {
		case <-result:
		case <-time.After(3 * time.Second):
			t.Fatal("stream did not join")
		}
	}
	if f.pair.limits.channels != 0 || f.pair.limits.children != 0 || f.pair.limits.queued != 0 {
		t.Fatal("joined streams retained resource reservations")
	}
}

func TestClosedJoinStreamBidirectionalCreditAndEOF(t *testing.T) {
	f := newClosedJoinStreamFixture(t)
	block := bytes.Repeat([]byte{41}, closedLaneMaximum)
	for i := 0; i < 4; i++ {
		f.transfer(t, 0, ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: block})
	}
	credit := binary.BigEndian.AppendUint32(nil, uint32(closedLaneCredit))
	f.transfer(t, 1, ClosedLaneFrame{Kind: closedFrameCredit, Lane: 1, Body: credit})
	f.transfer(t, 0, ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: []byte("after credit")})
	f.transfer(t, 0, ClosedLaneFrame{Kind: closedFrameEOF, Lane: 1})
	f.transfer(t, 1, ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: []byte("reverse after EOF")})
	f.transfer(t, 1, ClosedLaneFrame{Kind: closedFrameEOF, Lane: 1})
	f.transfer(t, 1, ClosedLaneFrame{Kind: closedFrameClose, Lane: 1, Body: []byte{0}})
	f.joined(t)
}

func TestClosedJoinStreamRefusesCreditAndPostEOFData(t *testing.T) {
	for _, reason := range []string{"credit", "after-eof", "duplicate-eof", "second-join", "wrong-lane", "window"} {
		t.Run(reason, func(t *testing.T) {
			f := newClosedJoinStreamFixture(t)
			frame := ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: []byte{1}}
			switch reason {
			case "credit":
				frame = ClosedLaneFrame{Kind: closedFrameCredit, Lane: 1, Body: binary.BigEndian.AppendUint32(nil, 1)}
			case "after-eof", "duplicate-eof":
				f.transfer(t, 0, ClosedLaneFrame{Kind: closedFrameEOF, Lane: 1})
				if reason == "duplicate-eof" {
					frame = ClosedLaneFrame{Kind: closedFrameEOF, Lane: 1}
				}
			case "second-join":
				frame = ClosedLaneFrame{Kind: closedFrameOperation, Lane: 1, Body: make([]byte, 4096)}
			case "wrong-lane":
				frame.Lane = 3
			case "window":
				for i := 0; i < 4; i++ {
					f.transfer(t, 0, ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: make([]byte, closedLaneMaximum)})
				}
			}
			if err := WriteClosedLaneFrame(f.clients[0], frame); err != nil {
				t.Fatal(err)
			}
			if _, err := ReadClosedLaneFrame(f.clients[1]); err == nil {
				t.Fatal("invalid frame relayed")
			}
			f.joined(t)
		})
	}
}

func TestClosedJoinStreamCancellationJoinsBlockedReads(t *testing.T) {
	f := newClosedJoinStreamFixture(t)
	f.cancel()
	f.joined(t)
	for _, client := range f.clients {
		if _, err := ReadClosedLaneFrame(client); err == nil {
			t.Fatal("canceled stream remained usable")
		}
	}
}

// A precise cleanup fault on top of a real connection must remain observable.
type closedJoinCloseFault struct {
	net.Conn
	fault error
}

func (connection closedJoinCloseFault) Close() error {
	return errors.Join(connection.Conn.Close(), connection.fault)
}

func TestClosedJoinStreamReportsCloseFailure(t *testing.T) {
	f := newClosedJoinFixture(t)
	side, err := f.join(t, 1, 21, 10)
	if err != nil {
		t.Fatal(err)
	}
	_, server := closedJoinTCP(t)
	fault := errors.New("injected connection cleanup failure")
	ctx, cancel := context.WithCancel(t.Context())
	result := make(chan error, 1)
	go func() { result <- side.Serve(ctx, closedJoinCloseFault{Conn: server, fault: fault}) }()
	// Wait until the handler owns the connection, then cancel the waiting pair.
	for {
		f.pairs.mu.Lock()
		started := side.stream != nil
		f.pairs.mu.Unlock()
		if started {
			break
		}
		select {
		case err := <-result:
			t.Fatalf("handler failed before ownership: %v", err)
		case <-time.After(time.Millisecond):
		}
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, fault) {
			t.Fatalf("cleanup error hidden: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cleanup did not finish")
	}
}

// Only the completion of an actual TCP write is held, after the result write.
// Closing the socket cannot stand in for joining this outstanding writer.
type closedJoinHeldWriter struct {
	net.Conn
	writes           int
	entered, release chan struct{}
}

func (writer *closedJoinHeldWriter) Write(raw []byte) (int, error) {
	n, err := writer.Conn.Write(raw)
	writer.writes++
	if writer.writes == 2 {
		close(writer.entered)
		<-writer.release
	}
	return n, err
}

func TestClosedJoinStreamCancellationJoinsOppositeWriter(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	releaseWriter := func() { once.Do(func() { close(release) }) }
	defer releaseWriter()
	f := newClosedJoinStreamWithConnection(t, func(connection net.Conn) net.Conn {
		return &closedJoinHeldWriter{Conn: connection, entered: entered, release: release}
	})
	f.transfer(t, 1, ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: []byte("held completion")})
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("writer barrier was not reached")
	}
	f.cancel()
	for _, side := range f.sides {
		select {
		case <-side.Done():
		case <-time.After(time.Second):
			t.Fatal("cancel did not reach both sides")
		}
	}
	for _, result := range f.results {
		select {
		case err := <-result:
			t.Fatalf("handler released while opposite write was outstanding: %v", err)
		default:
		}
	}
	f.pair.limits.mu.Lock()
	retained := f.pair.limits.channels == 2 && f.pair.limits.queued == 2*closedJoinStreamQueue
	f.pair.limits.mu.Unlock()
	if !retained {
		t.Fatal("resources released before writer completion")
	}
	releaseWriter()
	f.joined(t)
}

func TestClosedJoinStreamOriginalByteBudgetIncludesControl(t *testing.T) {
	f := newClosedJoinStreamFixture(t)
	block := make([]byte, closedLaneMaximum)
	credit := binary.BigEndian.AppendUint32(nil, uint32(len(block)))
	// Each round consumes a data header/body and a returned CREDIT header/body
	// on each role channel. Less than the nominal payload-only count must fit.
	rounds := int(closedClassBytes(2) / uint64(closedLaneHeaderSize+len(block)+closedLaneHeaderSize+len(credit)))
	for i := 0; i < rounds-1; i++ {
		f.transfer(t, 0, ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: block})
		f.transfer(t, 1, ClosedLaneFrame{Kind: closedFrameCredit, Lane: 1, Body: credit})
	}
	if err := WriteClosedLaneFrame(f.clients[0], ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: block}); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadClosedLaneFrame(f.clients[1]); err == nil {
		t.Fatal("JOIN/RESULT and control traffic escaped original budget")
	}
	f.joined(t)
}

func TestClosedJoinStreamLateCloseCannotRelabelCancellation(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	releaseWriter := func() { once.Do(func() { close(release) }) }
	defer releaseWriter()
	f := newClosedJoinStreamWithConnection(t, func(connection net.Conn) net.Conn {
		return &closedJoinHeldWriter{Conn: connection, entered: entered, release: release}
	})
	f.transfer(t, 1, ClosedLaneFrame{Kind: closedFrameClose, Lane: 1, Body: []byte{0}})
	<-entered
	f.sides[0].Abort()
	releaseWriter()
	f.joined(t)
	if f.sides[0].pair.graceful {
		t.Fatal("late CLOSE completion relabeled cancellation")
	}
}

func TestClosedJoinStreamNonzeroCloseIsNotSuccess(t *testing.T) {
	f := newClosedJoinStreamFixture(t)
	f.transfer(t, 0, ClosedLaneFrame{Kind: closedFrameClose, Lane: 1, Body: []byte{1}})
	f.joined(t)
	if f.sides[0].pair.graceful {
		t.Fatal("nonzero terminal status became success")
	}
}

// Hold an already emitted reverse frame while the other direction completes.
// A graceful pair close must join that writer before closing its TLS transport.
type closedJoinObservedClose struct {
	*closedJoinHeldWriter
	closed    chan struct{}
	closeOnce sync.Once
}

func (connection *closedJoinObservedClose) Close() error {
	connection.closeOnce.Do(func() { close(connection.closed) })
	return connection.Conn.Close()
}

func TestClosedJoinGracefulCloseJoinsOppositeWriterBeforeTransport(t *testing.T) {
	entered, release, closed := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	releaseWriter := func() { once.Do(func() { close(release) }) }
	defer releaseWriter()
	f := newClosedJoinStreamWithConnection(t, func(connection net.Conn) net.Conn {
		return &closedJoinObservedClose{closedJoinHeldWriter: &closedJoinHeldWriter{Conn: connection, entered: entered, release: release}, closed: closed}
	})
	f.transfer(t, 1, ClosedLaneFrame{Kind: closedFrameBytes, Lane: 1, Body: []byte("held completion")})
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("writer not reached")
	}
	f.transfer(t, 0, ClosedLaneFrame{Kind: closedFrameClose, Lane: 1, Body: []byte{0}})
	select {
	case <-closed:
		t.Fatal("graceful pair closed physical transport with outstanding writer")
	case <-time.After(50 * time.Millisecond):
	}
	releaseWriter()
	f.joined(t)
	select {
	case <-closed:
	default:
		t.Fatal("joined pair retained physical transport")
	}
}
