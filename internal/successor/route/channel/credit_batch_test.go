package channel

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

// Mechanical scheduling fixtures start from consumed, queued credit rather
// than minting it by calling the physical frame writer. No authority is supplied.
func writeConsumedCredit(s *Session, l *Lane, frame ardp.Frame, terminal bool) error {
	if frame.Kind == ardp.KindCredit {
		s.mu.Lock()
		l.queuedCredit += binary.BigEndian.Uint32(frame.Body)
		s.mu.Unlock()
	}
	return s.write(l, frame, terminal)
}

// Consume genuine peer frames below the batching threshold. These bytes grant
// no new receive credit until a subsequent read fills the batch.
func consumeBeforeCredit(t *testing.T, peer net.Conn, lane *Lane, count int) {
	t.Helper()
	for count > 0 {
		body := bytes.Repeat([]byte{42}, min(count, ardp.MaximumBodySize))
		sent := make(chan error, 1)
		go func() { sent <- ardp.WriteFrame(peer, ardp.Frame{Kind: ardp.KindBytes, Lane: lane.id, Body: body}) }()
		got := make([]byte, len(body))
		if _, err := io.ReadFull(lane, got); err != nil || !bytes.Equal(got, body) {
			t.Fatal("prior consumption failed", err)
		}
		if err := <-sent; err != nil {
			t.Fatal(err)
		}
		count -= len(body)
	}
}

type creditBatchConn struct {
	net.Conn
	credits atomic.Int32
}

func (c *creditBatchConn) Write(raw []byte) (int, error) {
	if len(raw) >= ardp.HeaderSize && raw[6] == ardp.KindCredit {
		c.credits.Add(1)
	}
	return c.Conn.Write(raw)
}

func TestCreditBatchMakesProgressAcrossWindows(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		a, b := net.Pipe()
		observed := &creditBatchConn{Conn: b}
		end := time.Now().Add(time.Minute)
		left, writer, err := newJoinedSession(t.Context(), a, end, 4<<20, nil, NewBudget(4<<20))
		if err != nil {
			t.Fatal(err)
		}
		right, reader, err := newJoinedSession(t.Context(), observed, end, 4<<20, nil, NewBudget(4<<20))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = a.Close(); _ = b.Close(); _ = left.Close(); _ = right.Close() })
		body := bytes.Repeat([]byte{17}, 3*int(Window)+17)
		sent := make(chan error, 1)
		go func() {
			_, err := writer.Write(body)
			if err == nil {
				err = writer.CloseWrite()
			}
			sent <- err
		}()
		// Tiny consumer reads must not each produce a control frame.
		var got bytes.Buffer
		buffer := make([]byte, 37)
		for {
			n, err := reader.Read(buffer)
			got.Write(buffer[:n])
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
		}
		if err := <-sent; err != nil || !bytes.Equal(got.Bytes(), body) {
			t.Fatal("batched credit stalled or changed bytes", err)
		}
		if count := observed.credits.Load(); count < 5 || count > 6 {
			t.Fatal("credit was lost or fragmented per read", count)
		}
	})
}

func TestCreditBatchPendingConsumptionDoesNotGrantPeerBytes(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		local, peer := net.Pipe()
		s, lane, err := newJoinedSession(t.Context(), local, time.Now().Add(time.Minute), 4<<20, nil, NewBudget(4<<20))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = peer.Close(); _ = s.Close() })
		consumeBeforeCredit(t, peer, lane, int(Window/2)-1)
		// The peer has only its unspent initial credit, although the queue was
		// consumed. Fill exactly that remainder, then attempt one extra byte.
		for remaining := int(Window/2) + 1; remaining > 0; {
			body := make([]byte, min(remaining, ardp.MaximumBodySize))
			if err := ardp.WriteFrame(peer, ardp.Frame{Kind: ardp.KindBytes, Lane: lane.id, Body: body}); err != nil {
				t.Fatal(err)
			}
			remaining -= len(body)
		}
		if err := ardp.WriteFrame(peer, ardp.Frame{Kind: ardp.KindBytes, Lane: lane.id, Body: []byte{1}}); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		select {
		case <-s.Done():
		default:
			t.Fatal("unemitted CREDIT authorized extra input")
		}
	})
}

func TestCreditBatchQueuedOutputDoesNotGrantPeerBytes(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		local, peer := net.Pipe()
		s, lane, err := newJoinedSession(t.Context(), local, time.Now().Add(time.Minute), 4<<20, nil, NewBudget(4<<20))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = peer.Close(); _ = s.Close() })
		consumeBeforeCredit(t, peer, lane, int(Window/2)-1)
		// A sibling retains the physical turn before this CREDIT can start.
		sibling := lifecycleLane(t, s, 3)
		release, err := s.turn(sibling, ardp.Frame{Kind: ardp.KindBytes, Lane: 3, Body: []byte{2}}, false, ardp.HeaderSize+1)
		if err != nil {
			t.Fatal(err)
		}
		defer release()
		if err := ardp.WriteFrame(peer, ardp.Frame{Kind: ardp.KindBytes, Lane: lane.id, Body: []byte{42}}); err != nil {
			t.Fatal(err)
		}
		consumed := make(chan error, 1)
		go func() { var body [1]byte; _, err := lane.Read(body[:]); consumed <- err }()
		synctest.Wait()
		s.mu.Lock()
		queued := lane.queuedCredit == Window/2 && lane.pendingCredit == 0
		s.mu.Unlock()
		if !queued {
			t.Fatal("CREDIT did not wait for its physical turn")
		}
		for remaining := int(Window / 2); remaining > 0; {
			body := make([]byte, min(remaining, ardp.MaximumBodySize))
			if err := ardp.WriteFrame(peer, ardp.Frame{Kind: ardp.KindBytes, Lane: lane.id, Body: body}); err != nil {
				t.Fatal(err)
			}
			remaining -= len(body)
		}
		if err := ardp.WriteFrame(peer, ardp.Frame{Kind: ardp.KindBytes, Lane: lane.id, Body: []byte{1}}); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		select {
		case <-s.Done():
		default:
			t.Fatal("queued CREDIT authorized extra input")
		}
		if err := <-consumed; err == nil {
			t.Fatal("peer credit violation disappeared from queued consumption")
		}
	})
}
