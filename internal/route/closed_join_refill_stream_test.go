//go:build linux

package route

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route/ardp"
)

func TestClosedJoinStreamReplenishmentAcknowledgesAcceptedReserve(t *testing.T) {
	f := newClosedJoinStreamFixture(t)
	defer f.cancel()
	accepted := make(chan struct{}, 1)
	f.pair.pairs.mu.Lock()
	f.pair.pairs.replenish = func(ClosedAdmissionVerification) (func() error, error) {
		accepted <- struct{}{}
		return nil, nil
	}
	f.pair.pairs.mu.Unlock()
	// The actual admitted paired Serve owner must generate its own ACK.
	if err := ardp.WriteFrame(f.clients[0], joinRefillFrame()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-accepted:
	case <-time.After(time.Second):
		t.Fatal("refill did not reach admission policy")
	}
	if err := f.clients[0].SetReadDeadline(time.Now().Add(250 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	frame, err := ardp.ReadFrame(f.clients[0])
	if err != nil {
		t.Fatalf("accepted JOIN refill has no server ACCEPT: %v", err)
	}
	status, credit, err := ardp.DecodeAcceptFrame(frame)
	if err != nil || status != 0 || credit != 64<<10 {
		t.Fatalf("refill ACCEPT = %v, %d, %d", err, status, credit)
	}
	f.pair.pairs.mu.Lock()
	remaining := f.sides[0].byteLimit - f.sides[0].used
	f.pair.pairs.mu.Unlock()
	// The next read may already have reserved its 16-byte header.
	if remaining > (32<<20)-ardp.HeaderSize-5 || remaining < (32<<20)-2*ardp.HeaderSize-5 {
		t.Fatalf("ACK debit remainder = %d", remaining)
	}
	f.cancel()
	f.joined(t)
}

type joinRefillBlockedHeader struct {
	kind uint8
	net.Conn
	entered, resume chan struct{}
	overlap         chan struct{}
	active          atomic.Int32
}

func (connection *joinRefillBlockedHeader) Write(raw []byte) (int, error) {
	if connection.active.Add(1) != 1 {
		select {
		case connection.overlap <- struct{}{}:
		default:
		}
	}
	defer connection.active.Add(-1)
	if len(raw) > ardp.HeaderSize && ardp.ValidHeader(raw[:ardp.HeaderSize]) && raw[6] == connection.kind {
		// A legal short physical Write exposes the real header/body boundary
		// inside ARDP's writeAll loop, rather than inventing a different codec.
		n, err := connection.Conn.Write(raw[:ardp.HeaderSize])
		close(connection.entered)
		<-connection.resume
		return n, err
	}
	return connection.Conn.Write(raw)
}

func TestClosedJoinRefillSerializesWithOppositeSideFrame(t *testing.T) {
	for _, kind := range []uint8{ardp.KindBytes, ardp.KindCredit} {
		t.Run(map[uint8]string{ardp.KindBytes: "BYTES", ardp.KindCredit: "CREDIT"}[kind], func(t *testing.T) { checkClosedJoinRefillOutput(t, kind) })
	}
}

func checkClosedJoinRefillOutput(t *testing.T, kind uint8) {
	t.Helper()
	entered, resume := make(chan struct{}), make(chan struct{})
	var resumeOnce sync.Once
	unblock := func() { resumeOnce.Do(func() { close(resume) }) }
	defer unblock()
	var output *joinRefillBlockedHeader
	f := newClosedJoinStreamWithConnection(t, func(connection net.Conn) net.Conn {
		output = &joinRefillBlockedHeader{kind: kind, Conn: connection, entered: entered, resume: resume, overlap: make(chan struct{}, 1)}
		return output
	})
	defer f.cancel()
	accepted := make(chan struct{}, 1)
	f.pair.pairs.mu.Lock()
	f.pair.pairs.replenish = func(ClosedAdmissionVerification) (func() error, error) { accepted <- struct{}{}; return nil, nil }
	f.pair.pairs.mu.Unlock()
	payload := []byte("opposite-side ciphertext")
	if kind == ardp.KindCredit {
		f.transfer(t, 0, ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: payload})
		payload = binary.BigEndian.AppendUint32(nil, uint32(len(payload)))
	}
	if err := ardp.WriteFrame(f.clients[1], ardp.Frame{Kind: kind, Lane: 1, Body: payload}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("opposite output did not start")
	}
	if err := ardp.WriteFrame(f.clients[0], joinRefillFrame()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-accepted:
	case <-time.After(time.Second):
		t.Fatal("refill policy blocked behind physical output")
	}
	// A partially emitted frame retains the whole output owner while the
	// independent receiving policy can proceed. No ACK may interleave.
	select {
	case <-output.overlap:
		t.Fatal("ACK interleaved with opposite frame header/body")
	case <-time.After(25 * time.Millisecond):
	}
	unblock()
	data, err := ardp.ReadFrame(f.clients[0])
	if err != nil || data.Kind != kind || !bytes.Equal(data.Body, payload) {
		t.Fatalf("opposite output corrupted: %v", err)
	}
	ack, err := ardp.ReadFrame(f.clients[0])
	if err != nil {
		t.Fatal(err)
	}
	status, credit, err := ardp.DecodeAcceptFrame(ack)
	if err != nil || status != 0 || credit != 64<<10 {
		t.Fatal("missing matching refill acknowledgement")
	}
	f.pair.pairs.mu.Lock()
	senderCredit := f.sides[1].stream.credit
	receiverCredit := f.sides[0].stream.credit
	f.pair.pairs.mu.Unlock()
	expectedSender := uint64(closedLaneCredit)
	if kind == ardp.KindBytes {
		expectedSender -= uint64(len(payload))
	}
	if senderCredit != expectedSender || receiverCredit != closedLaneCredit {
		t.Fatal("ACK changed JOIN windows")
	}
	f.cancel()
	f.joined(t)
}

func TestClosedJoinRefillRefusalEmitsNoAcceptedACK(t *testing.T) {
	f := newClosedJoinStreamFixture(t)
	defer f.cancel()
	refused := errors.New("actual admission policy refused")
	f.pair.pairs.mu.Lock()
	f.pair.pairs.replenish = func(ClosedAdmissionVerification) (func() error, error) { return nil, refused }
	f.pair.pairs.mu.Unlock()
	if err := ardp.WriteFrame(f.clients[0], joinRefillFrame()); err != nil {
		t.Fatal(err)
	}
	if frame, err := ardp.ReadFrame(f.clients[0]); err == nil {
		t.Fatalf("refusal emitted frame kind %d", frame.Kind)
	}
	select {
	case err := <-f.results[0]:
		if !errors.Is(err, refused) {
			t.Fatalf("refusal cause lost: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("refused handler not joined")
	}
	select {
	case <-f.results[1]:
	case <-time.After(time.Second):
		t.Fatal("refused counterpart not joined")
	}
}

func TestClosedJoinRefillRetirementDuringReserveEmitsNoACK(t *testing.T) {
	for _, mode := range []string{"cancel", "deadline"} {
		t.Run(mode, func(t *testing.T) {
			f := newClosedJoinStreamFixture(t)
			defer f.cancel()
			entered, resume := make(chan struct{}), make(chan struct{})
			var releaseResume sync.Once
			unblock := func() { releaseResume.Do(func() { close(resume) }) }
			defer unblock()
			var released atomic.Int32
			f.pair.pairs.mu.Lock()
			before := f.sides[0].byteLimit
			f.pair.pairs.replenish = func(ClosedAdmissionVerification) (func() error, error) {
				close(entered)
				<-resume
				return func() error { released.Add(1); return nil }, nil
			}
			f.pair.pairs.mu.Unlock()
			if err := ardp.WriteFrame(f.clients[0], joinRefillFrame()); err != nil {
				t.Fatal(err)
			}
			select {
			case <-entered:
			case <-time.After(time.Second):
				t.Fatal("reserve policy not reached")
			}
			if mode == "cancel" {
				f.cancel()
			} else {
				f.pair.clock.Store(f.sides[0].deadline.Unix())
			}
			unblock()
			if frame, err := ardp.ReadFrame(f.clients[0]); err == nil {
				t.Fatalf("retired refill emitted frame kind %d", frame.Kind)
			}
			f.joined(t)
			if f.sides[0].byteLimit != before || released.Load() != 1 {
				t.Fatal("retired refill changed allowance or leaked fresh reservation")
			}
		})
	}
}

type joinRefillFailingACK struct {
	net.Conn
	cause error
}

func (connection *joinRefillFailingACK) Write(raw []byte) (int, error) {
	if len(raw) >= ardp.HeaderSize && ardp.ValidHeader(raw[:ardp.HeaderSize]) && raw[6] == ardp.KindAccept {
		n, err := connection.Conn.Write(raw[:ardp.HeaderSize])
		return n, errors.Join(connection.cause, err)
	}
	return connection.Conn.Write(raw)
}

func TestClosedJoinRefillPartialACKFailureJoinsPair(t *testing.T) {
	cause := errors.New("partial ACK physical failure")
	f := newClosedJoinStreamWithConnection(t, func(connection net.Conn) net.Conn { return &joinRefillFailingACK{Conn: connection, cause: cause} })
	defer f.cancel()
	f.pair.pairs.mu.Lock()
	f.pair.pairs.replenish = func(ClosedAdmissionVerification) (func() error, error) { return nil, nil }
	f.pair.pairs.mu.Unlock()
	if err := ardp.WriteFrame(f.clients[0], joinRefillFrame()); err != nil {
		t.Fatal(err)
	}
	if frame, err := ardp.ReadFrame(f.clients[0]); err == nil {
		t.Fatalf("partial ACK became complete kind %d", frame.Kind)
	}
	select {
	case err := <-f.results[0]:
		if !errors.Is(err, cause) {
			t.Fatalf("physical ACK cause lost: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("partial ACK handler not joined")
	}
	select {
	case <-f.results[1]:
	case <-time.After(time.Second):
		t.Fatal("partial ACK counterpart not joined")
	}
	if f.pair.limits.channels != 0 || f.pair.limits.children != 0 || f.pair.limits.queued != 0 {
		t.Fatal("partial ACK retained reservations")
	}
}
