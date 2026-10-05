//go:build linux

package transport

import (
	"errors"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

// A real prefix of the first terminal frame reaches the peer before this
// wrapper returns the physical failure. Other I/O remains the actual pipe.
type joinTerminalWriteFault struct {
	net.Conn
	cause error
	fired atomic.Bool
}

func (c *joinTerminalWriteFault) Write(raw []byte) (int, error) {
	if len(raw) >= ardp.HeaderSize && raw[6] == ardp.KindClose && c.fired.CompareAndSwap(false, true) {
		n, err := c.Conn.Write(raw[:8])
		if err != nil {
			return n, err
		}
		return n, c.cause
	}
	return c.Conn.Write(raw)
}

func TestJoinPartialTerminalFailurePreventsLaterTerminalOutput(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fixture := newJoinBoundsFixture()
		defer fixture.close(t)
		receiver := &Receiver{}
		fixture.pairs.record = receiver.record
		failure := errors.New("partially emitted JOIN terminal failed")
		var fault *joinTerminalWriteFault
		first := fixture.start(t, joinBoundsRequest(1, 1), func(conn net.Conn) net.Conn {
			fault = &joinTerminalWriteFault{Conn: conn, cause: failure}
			return fault
		})
		second := fixture.start(t, joinBoundsRequest(2, 2), nil)
		readJoinBoundsResult(t, first, 1)
		readJoinBoundsResult(t, second, 2)
		synctest.Wait()
		if err := ardp.WriteFrame(first.conn, ardp.Frame{Kind: ardp.KindClose, Lane: 1, Body: []byte{0}}); err != nil {
			t.Fatal(err)
		}
		var prefix [8]byte
		if _, err := io.ReadFull(first.conn, prefix[:]); err != nil || prefix[6] != ardp.KindClose {
			t.Fatal("terminal frame did not physically start before failure", err)
		}
		frame, err := ardp.ReadFrame(second.conn)
		if err == nil {
			t.Errorf("opposite terminal output followed a partially failed CLOSE: kind=%d body=%v", frame.Kind, frame.Body)
		} else if err != io.EOF {
			t.Error("opposite physical retirement returned an unexpected framing error", err)
		}
		for _, peer := range []*joinBoundsPeer{first, second} {
			var physical *physicalWriteFailure
			if err := peer.wait(); !errors.Is(err, failure) || !errors.As(err, &physical) {
				t.Error("joined handler lost its terminal physical failure", err)
			}
		}
		if !fault.fired.Load() {
			t.Fatal("terminal write failure was not exercised")
		}
		receiver.mu.Lock()
		retained := receiver.err
		receiver.mu.Unlock()
		if !errors.Is(retained, failure) {
			t.Fatal("Receiver.record lost the partially failed terminal", retained)
		}
	})
}
