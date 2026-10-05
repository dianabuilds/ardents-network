//go:build linux

package transport

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

// The second read has already entered physical I/O, but its result returns only
// when retirement interrupts it. This models late input completion without
// claiming any authority, Admission or genuine Carrier evidence.
type joinLateInput struct {
	net.Conn
	input  *bytes.Reader
	late   chan struct{}
	once   sync.Once
	writes int
}

func (c *joinLateInput) Read(p []byte) (int, error) {
	if c.late != nil {
		<-c.late
	}
	return c.input.Read(p)
}
func (c *joinLateInput) Write(p []byte) (int, error) { c.writes++; return len(p), nil }
func (c *joinLateInput) SetDeadline(time.Time) error {
	if c.late != nil {
		c.once.Do(func() { close(c.late) })
	}
	return nil
}
func (c *joinLateInput) SetWriteDeadline(time.Time) error { return nil }
func (c *joinLateInput) Close() error                     { return nil }

func TestJoinCleanTerminalCannotEraseLateOppositeRefusal(t *testing.T) {
	for _, late := range []ardp.Frame{
		{Kind: ardp.KindClose, Lane: 1, Body: []byte{1}},
		{Kind: ardp.KindAdmit, Body: make([]byte, 355)},
	} {
		synctest.Test(t, func(t *testing.T) {
			owner := &joinPairs{}
			p := &joinPair{owner: owner, stopped: make(chan struct{})}
			for i, frame := range []ardp.Frame{{Kind: ardp.KindClose, Lane: 1, Body: []byte{0}}, late} {
				raw, err := ardp.EncodeFrame(frame)
				if err != nil {
					t.Fatal(err)
				}
				c := &joinLateInput{input: bytes.NewReader(raw)}
				if i == 1 {
					c.late = make(chan struct{})
				}
				p.sides[i] = &joinSide{ctx: context.Background(), conn: c, hello: ardp.Hello{Deadline: time.Now().Add(time.Minute)}, pair: p, limit: 1 << 20, credit: window}
			}
			p.relay(p.sides)
			if p.err == nil {
				t.Fatal("clean terminal erased the opposite late refusal")
			}
		})
	}
}

type joinCancelAtAttach struct {
	context.Context
	cancel context.CancelFunc
	calls  atomic.Int32
}

func (c *joinCancelAtAttach) Err() error {
	count := c.calls.Add(1)
	err := c.Context.Err()
	if count == 2 {
		// Capture a still-live final observation, then cancel before the
		// subsequent locked attachment. Context cancellation is real.
		c.cancel()
	}
	return err
}

func TestJoinCanceledAttachmentCannotLeaveUnownedPair(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		base, cancel := context.WithCancel(context.Background())
		defer cancel()
		owner := &joinPairs{}
		ctx := &joinCancelAtAttach{Context: base, cancel: cancel}
		local, remote := net.Pipe()
		defer remote.Close()
		capacity, err := reserveJoinCapacity(&queueBudget{maximum: 1 << 20})
		if err != nil {
			t.Fatal(err)
		}
		defer capacity.release()
		end := time.Now().Add(time.Minute).UTC().Truncate(time.Second)
		raw, err := ardp.EncodeJoinRequest(ardp.JoinRequest{Nonce: [32]byte{1}, Secret: [32]byte{2}, Context: [32]byte{3}, Side: 1, Deadline: end})
		if err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() {
			done <- owner.serve(ctx, local, ardp.Hello{Purpose: ardp.PurposeDataJoin, Deadline: end}, 1<<20, nil, capacity)
		}()
		if err := ardp.WriteFrame(remote, ardp.Frame{Kind: ardp.KindOperation, Lane: 1, Body: raw}); err != nil && !errors.Is(err, io.ErrClosedPipe) {
			t.Fatal(err)
		}
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Fatal("missing caller refusal", err)
		}
		if len(owner.entries) != 0 {
			t.Fatal("canceled attachment left an unowned pair without timer or join")
		}
	})
}
