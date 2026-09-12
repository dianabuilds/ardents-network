package node

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

type closedOuterCountedWrites struct {
	net.Conn
	writes atomic.Int32
}

func (connection *closedOuterCountedWrites) Write(body []byte) (int, error) {
	connection.writes.Add(1)
	return connection.Conn.Write(body)
}

// The queued owner loses its deadline before entering the physical writer.
// Its failure cannot destroy a sibling when no frame has been attempted.
func TestClosedOuterExpiredQueuedWritePreservesSibling(t *testing.T) {
	for _, kind := range []byte{6, 7} {
		t.Run(map[byte]string{6: "bytes", 7: "credit"}[kind], func(t *testing.T) {
			local, peer := net.Pipe()
			observed := &closedOuterCountedWrites{Conn: local}
			writer := &closedOuterWriter{connection: observed}
			writer.writer.Lock()
			var unlock sync.Once
			release := func() { unlock.Do(writer.writer.Unlock) }
			var helpers sync.WaitGroup
			t.Cleanup(func() { release(); local.Close(); peer.Close(); helpers.Wait() })
			var mu sync.Mutex
			end := time.Now().Add(time.Minute)
			body := []byte{1}
			if kind == 7 {
				body = binary.BigEndian.AppendUint32(nil, 1)
			}
			requested, completed := make(chan struct{}), make(chan error, 1)
			helpers.Go(func() {
				close(requested)
				completed <- writer.write(route.ClosedLaneFrame{Kind: kind, Lane: 1, Body: body}, func() time.Time { mu.Lock(); defer mu.Unlock(); return end })
			})
			<-requested
			mu.Lock()
			end = time.Now().Add(-time.Second)
			mu.Unlock()
			release()
			if err := <-completed; !errors.Is(err, os.ErrDeadlineExceeded) {
				t.Fatalf("expired queued write: %v", err)
			}
			if count := observed.writes.Load(); count != 0 {
				t.Errorf("expired owner attempted %d physical writes", count)
			}
			received := make(chan error, 1)
			helpers.Go(func() {
				frame, err := route.ReadClosedLaneFrame(peer)
				if err == nil && (frame.Kind != 6 || frame.Lane != 3 || len(frame.Body) != 1 || frame.Body[0] != 42) {
					err = errors.New("sibling frame changed")
				}
				received <- err
			})
			if err := writer.write(route.ClosedLaneFrame{Kind: 6, Lane: 3, Body: []byte{42}}, func() time.Time { return time.Now().Add(time.Second) }); err != nil {
				t.Fatalf("unattempted expired frame closed sibling: %v", err)
			}
			if err := <-received; err != nil {
				t.Fatal(err)
			}
			helpers.Wait()
		})
	}
}

// Once even one byte was accepted by the peer, a cancelled frame must poison
// the Carrier. A later lane cannot treat that truncated frame as its own.
func TestClosedOuterPartialCreditStillClosesCarrier(t *testing.T) {
	local, peer := net.Pipe()
	writer := &closedOuterWriter{connection: local}
	var helpers sync.WaitGroup
	t.Cleanup(func() { local.Close(); peer.Close(); helpers.Wait() })
	completed := make(chan error, 1)
	helpers.Go(func() {
		completed <- writer.write(route.ClosedLaneFrame{Kind: 7, Lane: 1, Body: binary.BigEndian.AppendUint32(nil, 1)}, func() time.Time { return time.Now().Add(time.Second) })
	})
	if err := peer.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	var first [1]byte
	if _, err := io.ReadFull(peer, first[:]); err != nil {
		t.Fatal(err)
	}
	if err := writer.update(1, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := <-completed; err == nil {
		t.Fatal("partially emitted CREDIT reported success")
	}
	if err := writer.write(route.ClosedLaneFrame{Kind: 6, Lane: 3, Body: []byte{42}}, func() time.Time { return time.Now().Add(time.Second) }); err == nil {
		t.Fatal("sibling reused a truncated physical frame")
	}
	helpers.Wait()
}
