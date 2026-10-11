package channel

import (
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	"github.com/dianabuilds/ardents-network/internal/successor/route/transport"
)

// Failure-only input isolates whole-frame classification. Actual adapter
// custody and native socket admission are tested separately by its owner.
type nativeRefusalConn struct {
	net.Conn
	cause error
}

func TestNestedNativeRefusalCannotAdvancePhysicalWitness(t *testing.T) {
	physical := newLifecycleConn(false)
	refused := transport.MarkUnstartedWrite(net.ErrClosed)
	end := time.Now().Add(5 * time.Second)
	lower := Prepare(t.Context(), nativeRefusalConn{Conn: physical, cause: refused}, end, 32<<20, nil, false, &Budget{maximum: 64 << 20}, Handlers{})
	lane := lifecycleLane(t, lower, 1)
	lane.openEmitted = true
	upper := Prepare(t.Context(), lane, end, 32<<20, nil, false, &Budget{maximum: 64 << 20}, Handlers{})
	inner := lifecycleLane(t, upper, 1)
	inner.openEmitted = true
	t.Cleanup(func() { _ = upper.Close(); _ = lower.Close() })
	if _, err := inner.Write([]byte("x")); err == nil || !errors.Is(err, net.ErrClosed) {
		t.Fatalf("refused nested operation became successful: %v", err)
	}
	for _, item := range []struct {
		name    string
		session *Session
		lane    *Lane
	}{
		{"lower", lower, lane}, {"upper", upper, inner},
	} {
		item.session.mu.Lock()
		attempts, payload := item.lane.physicalAttempts, item.lane.payloadAttempts
		item.session.mu.Unlock()
		if attempts != 0 || payload != 0 {
			t.Fatalf("%s unemitted output minted a nested physical witness: %d/%d", item.name, attempts, payload)
		}
		first := item.session.Close()
		if first == nil || !errors.Is(first, net.ErrClosed) || item.session.PhysicalFailure() != nil || item.session.Close() != first {
			t.Fatalf("%s refusal lost joined failed outcome or minted physical failure: %v physical=%v", item.name, first, item.session.PhysicalFailure())
		}
	}
}

func (conn nativeRefusalConn) Write([]byte) (int, error) { return 0, conn.cause }

type partialNativeRefusalConn struct {
	net.Conn
	written bool
}

func (conn *partialNativeRefusalConn) Write(raw []byte) (int, error) {
	if conn.written {
		return 0, transport.MarkUnstartedWrite(net.ErrClosed)
	}
	conn.written = true
	return conn.Conn.Write(raw[:1])
}

// A later unstarted record cannot erase bytes already written by this frame.
// The independent pipe reader observes the actual first byte, not a counter.
func TestPartialFrameThenNativeRefusalRetainsPhysicalFailure(t *testing.T) {
	writer, reader := net.Pipe()
	defer writer.Close()
	defer reader.Close()
	if err := writer.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := reader.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	observed := make(chan error, 1)
	go func() {
		var value [1]byte
		_, err := io.ReadFull(reader, value[:])
		observed <- err
	}()
	out := WriteFrame(&partialNativeRefusalConn{Conn: writer}, ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: []byte("x")})
	if err := <-observed; err != nil {
		t.Fatal("actual partial output absent", err)
	}
	if out.Err() == nil || !errors.Is(out.Err(), net.ErrClosed) || out.PhysicalFailure() == nil {
		t.Fatalf("later unstarted refusal erased original partial frame output: %v physical=%v", out.Err(), out.PhysicalFailure())
	}
}

func TestNativeWriteRefusalDoesNotMintPhysicalAttemptOrSuccess(t *testing.T) {
	marked := transport.MarkUnstartedWrite(net.ErrClosed)
	for _, test := range []struct {
		name     string
		cause    error
		physical bool
	}{
		{"owned-before-output", marked, false},
		{"raw-closed", net.ErrClosed, true},
		{"mixed", errors.Join(marked, errors.New("other failure")), true},
	} {
		t.Run(test.name, func(t *testing.T) {
			out := WriteFrame(nativeRefusalConn{cause: test.cause}, ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: []byte("x")})
			if out.Err() == nil || !errors.Is(out.Err(), test.cause) || (out.PhysicalFailure() != nil) != test.physical {
				t.Fatalf("failed output provenance changed: %v physical=%v", out.Err(), out.PhysicalFailure())
			}
		})
	}
}
