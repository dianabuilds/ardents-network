package channel

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
)

func TestPreparedJoinRefusalJoinsWithoutStartingDataReader(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		local, remote := net.Pipe()
		defer remote.Close()
		conn := &joinReadObservation{Conn: local}
		queues := NewBudget(4 << 20)
		s, _, err := PrepareJoined(context.Background(), conn, time.Now().Add(time.Minute), 1<<20, nil, queues)
		if err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		if conn.reads.Load() != 0 {
			t.Fatal("prepared JOIN read data before both RESULT writes")
		}
		if err := s.Close(); err != nil {
			t.Fatal("prepared JOIN refusal did not join", err)
		}
		if conn.reads.Load() != 0 || queues.children != 0 || queues.used != 0 {
			t.Fatal("refused preparation read data or retained capacity")
		}
	})
}

type joinReadObservation struct {
	net.Conn
	reads atomic.Int32
}

func (c *joinReadObservation) Read(p []byte) (int, error) {
	c.reads.Add(1)
	return c.Conn.Read(p)
}

func TestJoinedInputRetainsBytesButRawEOFIsNotCompletion(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		local, remote := net.Pipe()
		queues := NewBudget(4 << 20)
		s, lane, err := newJoinedSession(context.Background(), local, time.Now().Add(time.Minute), 1<<20, nil, queues)
		if err != nil {
			t.Fatal(err)
		}
		payload := []byte("retained opaque ciphertext")
		written := make(chan error, 1)
		go func() {
			written <- ardp.WriteFrame(remote, ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: payload})
			_ = remote.Close()
		}()
		<-s.Done()
		if err := <-written; err != nil {
			t.Fatal(err)
		}
		got, err := io.ReadAll(lane)
		if !bytes.Equal(got, payload) || !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("accepted bytes or missing-terminal outcome lost: bytes=%d error=%v", len(got), err)
		}
		if err := s.Close(); !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatal("joined retirement hid raw EOF", err)
		}
	})
}

func TestJoinedInputRequiresInnerCloseAndRetainsRefusal(t *testing.T) {
	for _, status := range []byte{0, 1} {
		t.Run(string(rune('0'+status)), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				local, remote := net.Pipe()
				s, lane, err := newJoinedSession(context.Background(), local, time.Now().Add(time.Minute), 1<<20, nil, NewBudget(4<<20))
				if err != nil {
					t.Fatal(err)
				}
				payload := []byte("already received ciphertext")
				written := make(chan error, 1)
				go func() {
					err := ardp.WriteFrame(remote, ardp.Frame{Kind: ardp.KindBytes, Lane: 1, Body: payload})
					if err == nil {
						err = ardp.WriteFrame(remote, ardp.Frame{Kind: ardp.KindClose, Lane: 1, Body: []byte{status}})
					}
					written <- err
					_ = remote.Close()
				}()
				<-s.Done()
				if err := <-written; err != nil {
					t.Fatal(err)
				}
				got, readErr := io.ReadAll(lane)
				if !bytes.Equal(got, payload) || (readErr != nil) != (status != 0) {
					t.Fatalf("inner terminal outcome lost: bytes=%d error=%v", len(got), readErr)
				}
				if closeErr := s.Close(); (closeErr != nil) != (status != 0) {
					t.Fatal("joined retirement changed peer terminal outcome", closeErr)
				}
			})
		})
	}
}
