package channel

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

// Hold a real encoded physical frame after its header prefix was emitted.
// This is nested TLS/framing only; no successful domain authority is supplied.
type nestedControlCloseConn struct {
	net.Conn
	armed                         atomic.Bool
	emitted, release, interrupted chan struct{}
	once                          sync.Once
	late                          error
}

func (conn *nestedControlCloseConn) Write(body []byte) (int, error) {
	if !conn.armed.CompareAndSwap(true, false) {
		return conn.Conn.Write(body)
	}
	n, err := conn.Conn.Write(body[:8])
	close(conn.emitted)
	<-conn.release
	if err != nil {
		return n, err
	}
	rest, err := conn.Conn.Write(body[8:])
	return n + rest, errors.Join(err, conn.late)
}

func (conn *nestedControlCloseConn) SetWriteDeadline(end time.Time) error {
	err := conn.Conn.SetWriteDeadline(end)
	select {
	case <-conn.emitted:
		conn.once.Do(func() { close(conn.interrupted) })
	default:
	}
	return err
}

func TestNestedTLSCreditCloseRetainsWholeOriginalPhysicalFrame(t *testing.T) {
	for _, failure := range []string{"none", "nested-owner", "payload", "late-control"} {
		t.Run(failure, func(t *testing.T) { nestedTLSControlClose(t, failure) })
	}
}

func nestedTLSControlClose(t *testing.T, failure string) {
	t.Helper()
	var physical *nestedControlCloseConn
	late := errors.New("original late physical failure")
	lower, _, secured, peerTLS := nestedRetirementTLSPhysical(t, func(conn net.Conn) net.Conn {
		physical = &nestedControlCloseConn{Conn: conn, emitted: make(chan struct{}),
			release: make(chan struct{}), interrupted: make(chan struct{})}
		if failure == "late-control" {
			physical.late = late
		}
		return physical
	})
	var release sync.Once
	unblock := func() { release.Do(func() { close(physical.release) }) }
	t.Cleanup(unblock)
	physical.armed.Store(true)
	body := make([]byte, 4)
	binary.BigEndian.PutUint32(body, 1)
	kind := uint8(ardp.KindCredit)
	if failure == "payload" {
		kind = ardp.KindBytes
	}
	written := make(chan FrameOutput, 1)
	closeLane := lower
	if failure == "nested-owner" {
		upper := New(context.Background(), secured, time.Now().Add(5*time.Second), 1<<20, nil, false, &Budget{maximum: 4 << 20}, nil)
		closeLane = lifecycleLane(t, upper, 1)
		t.Cleanup(func() { _ = upper.Close() })
		t.Cleanup(unblock)
		go func() {
			written <- FrameOutput{failure: upper.write(closeLane, ardp.Frame{Kind: kind, Lane: 1, Body: body}, false)}
		}()
	} else {
		go func() { written <- WriteFrame(secured, ardp.Frame{Kind: kind, Lane: 1, Body: body}) }()
	}
	select {
	case <-physical.emitted:
	case <-time.After(time.Second):
		t.Fatal("nested TLS control did not emit its real lower frame prefix")
	}
	closed := make(chan error, 1)
	go func() { closed <- closeLane.Close() }()
	select {
	case <-physical.interrupted:
	case <-time.After(time.Second):
		t.Fatal("retirement did not bound its original physical writer")
	}
	unblock()
	output := <-written
	if failure == "payload" || failure == "late-control" {
		_ = lifecycleResult(t, closed)
		if output.Err() == nil || output.PhysicalFailure() == nil || lower.s.Live() {
			t.Fatal("control retirement concealed payload interruption or late physical failure", output.Err())
		}
		if failure == "late-control" && !errors.Is(output.PhysicalFailure(), late) {
			t.Fatal("original late physical failure was replaced", output.PhysicalFailure())
		}
		return
	}
	if output.Err() != nil || output.PhysicalFailure() != nil {
		t.Fatal("nested CREDIT retirement corrupted its original physical frame", output.Err(), output.PhysicalFailure())
	}
	if err := lifecycleResult(t, closed); err != nil || !lower.s.Live() {
		t.Fatal("nested CREDIT close retired its healthy parent", err)
	}
	frame, err := ardp.ReadFrame(peerTLS)
	if err != nil || frame.Kind != ardp.KindCredit || binary.BigEndian.Uint32(frame.Body) != 1 {
		t.Fatal("peer did not receive the original complete control", err)
	}
}
