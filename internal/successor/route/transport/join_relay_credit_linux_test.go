//go:build linux

package transport

import (
	"encoding/binary"
	"errors"
	"net"
	"os"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

// These real pipe controls isolate post-admission relay termination. They do
// not supply Network, token, Hosting or Service authority.
type joinRelayCreditConn struct {
	net.Conn
	credit, interrupted       chan struct{}
	creditOnce, interruptOnce sync.Once
}

func TestJoinRelayPeerCloseCannotWaitBeyondCreditCleanupBound(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fixture := newJoinBoundsFixture()
		defer fixture.close(t)
		first := fixture.start(t, joinBoundsRequest(1, 1), nil)
		var observed *joinRelayCreditConn
		second := fixture.start(t, joinBoundsRequest(2, 2), func(conn net.Conn) net.Conn {
			observed = &joinRelayCreditConn{Conn: conn, credit: make(chan struct{}), interrupted: make(chan struct{})}
			return observed
		})
		readJoinBoundsResult(t, first, 1)
		readJoinBoundsResult(t, second, 2)
		written := make(chan error, 1)
		go func() {
			written <- ardp.WriteFrame(second.conn, ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: []byte{92}})
		}()
		if frame, err := ardp.ReadFrame(first.conn); err != nil || frame.Kind != ardp.KindBytes {
			t.Fatal("consumed data absent", err)
		}
		if err := <-written; err != nil {
			t.Fatal(err)
		}
		if err := ardp.WriteFrame(first.conn, ardp.Frame{Kind: ardp.KindCredit, Lane: 1, Body: []byte{0, 0, 0, 1}}); err != nil {
			t.Fatal(err)
		}
		<-observed.credit
		start := time.Now()
		if err := ardp.WriteFrame(second.conn, ardp.Frame{Kind: ardp.KindClose, Lane: 1, Body: []byte{0}}); err != nil {
			t.Fatal(err)
		}
		<-observed.interrupted
		// No peer consumes the selected CREDIT. Its actual physical timeout
		// must fail the pair and return, rather than grant unbounded cleanup.
		for _, peer := range []*joinBoundsPeer{first, second} {
			if err := peer.wait(); !errors.Is(err, os.ErrDeadlineExceeded) {
				t.Fatal("failed CREDIT was erased", err)
			}
		}
		if elapsed := time.Since(start); elapsed != time.Second {
			t.Fatal("CREDIT cleanup did not retain its one-second bound", elapsed)
		}
	})
}

func (c *joinRelayCreditConn) Write(body []byte) (int, error) {
	if len(body) >= ardp.HeaderSize && body[6] == ardp.KindCredit {
		c.creditOnce.Do(func() { close(c.credit) })
	}
	return c.Conn.Write(body)
}

func (c *joinRelayCreditConn) SetDeadline(end time.Time) error {
	err := c.Conn.SetDeadline(end)
	if !end.After(time.Now()) {
		c.interruptOnce.Do(func() { close(c.interrupted) })
	}
	return err
}

func (c *joinRelayCreditConn) SetReadDeadline(end time.Time) error {
	err := c.Conn.SetReadDeadline(end)
	if !end.After(time.Now()) {
		c.interruptOnce.Do(func() { close(c.interrupted) })
	}
	return err
}

func TestJoinRelayPeerCloseAllowsSelectedCreditToJoin(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fixture := newJoinBoundsFixture()
		defer fixture.close(t)
		first := fixture.start(t, joinBoundsRequest(1, 1), nil)
		var observed *joinRelayCreditConn
		second := fixture.start(t, joinBoundsRequest(2, 2), func(conn net.Conn) net.Conn {
			observed = &joinRelayCreditConn{Conn: conn, credit: make(chan struct{}), interrupted: make(chan struct{})}
			return observed
		})
		readJoinBoundsResult(t, first, 1)
		readJoinBoundsResult(t, second, 2)
		written := make(chan error, 1)
		go func() {
			written <- ardp.WriteFrame(second.conn, ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: []byte{91}})
		}()
		data, err := ardp.ReadFrame(first.conn)
		if err != nil || data.Kind != ardp.KindBytes || len(data.Body) != 1 || data.Body[0] != 91 {
			t.Fatal("actual consumed data absent", err)
		}
		if err := <-written; err != nil {
			t.Fatal(err)
		}
		var credit [4]byte
		binary.BigEndian.PutUint32(credit[:], 1)
		if err := ardp.WriteFrame(first.conn, ardp.Frame{Kind: ardp.KindCredit, Lane: 1, Body: credit[:]}); err != nil {
			t.Fatal(err)
		}
		<-observed.credit
		if err := ardp.WriteFrame(second.conn, ardp.Frame{Kind: ardp.KindClose, Lane: 1, Body: []byte{0}}); err != nil {
			t.Fatal(err)
		}
		<-observed.interrupted
		firstTerminal := make(chan error, 1)
		go func() {
			frame, err := ardp.ReadFrame(first.conn)
			if err == nil && (frame.Kind != ardp.KindClose || len(frame.Body) != 1 || frame.Body[0] != 0) {
				err = net.ErrClosed
			}
			firstTerminal <- err
		}()
		frame, err := ardp.ReadFrame(second.conn)
		if err != nil || frame.Kind != ardp.KindCredit || len(frame.Body) != 4 || binary.BigEndian.Uint32(frame.Body) != 1 {
			t.Fatal("selected CREDIT was interrupted instead of joined", err)
		}
		frame, err = ardp.ReadFrame(second.conn)
		if err != nil || frame.Kind != ardp.KindClose || len(frame.Body) != 1 || frame.Body[0] != 0 {
			t.Fatal("opposite terminal absent", err)
		}
		if err := <-firstTerminal; err != nil {
			t.Fatal("first terminal absent", err)
		}
		if err := first.wait(); err != nil {
			t.Fatal("first side failed retirement", err)
		}
		if err := second.wait(); err != nil {
			t.Fatal("second side failed retirement", err)
		}
	})
}
