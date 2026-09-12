//go:build linux

package textdocument_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/interfacev2/connection"
)

type publisherFrameObservation struct {
	net.Conn
	mu        sync.Mutex
	header    [9]byte
	headerN   int
	remaining int
	total     int
	active    int
	maximum   int
	lastID    uint32
	invalidID bool
}

func (observer *publisherFrameObservation) Write(body []byte) (int, error) {
	n, err := observer.Conn.Write(body)
	observer.mu.Lock()
	defer observer.mu.Unlock()
	pending := body[:n]
	for len(pending) > 0 {
		if observer.remaining > 0 {
			consumed := min(len(pending), observer.remaining)
			observer.remaining -= consumed
			pending = pending[consumed:]
			continue
		}
		consumed := copy(observer.header[observer.headerN:], pending)
		observer.headerN += consumed
		pending = pending[consumed:]
		if observer.headerN != len(observer.header) {
			continue
		}
		observer.headerN = 0
		observer.remaining = int(binary.BigEndian.Uint32(observer.header[5:]))
		switch observer.header[0] {
		case 1:
			id := binary.BigEndian.Uint32(observer.header[1:5])
			observer.invalidID = observer.invalidID || id%2 != 1 || id <= observer.lastID
			observer.lastID = id
			observer.total++
			observer.active++
			observer.maximum = max(observer.maximum, observer.active)
		case 5:
			observer.active--
		}
	}
	return n, err
}

func TestPublisherWorkerBoundsOpenAndActiveConnections(t *testing.T) {
	var observer *publisherFrameObservation
	body := bytes.Repeat([]byte("p"), (64<<10)+1)
	harness := startPublisherHarness(t, body, func(conn net.Conn) io.ReadWriteCloser {
		observer = &publisherFrameObservation{Conn: conn}
		return observer
	})
	gate := make(chan struct{})
	streams := make([]*publisherFixture, 257)
	for index := range streams {
		streams[index] = newPublisherFixture(publisherRequest(), gate)
		if index < 256 {
			harness.admit(t, streams[index])
		}
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		observer.mu.Lock()
		active := observer.active
		observer.mu.Unlock()
		if active == 64 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("Publisher did not make progress to 64 active streams: %d", active)
		}
		time.Sleep(time.Millisecond)
	}
	select {
	case harness.incoming <- streams[256]:
		t.Fatal("Publisher admitted a 257th open Connection")
	case <-harness.finished:
		t.Fatalf("Publisher failed at its supported boundary: %v", harness.err)
	case <-time.After(100 * time.Millisecond):
	}
	close(gate)
	harness.admit(t, streams[256])
	harness.drain(t)
	for _, stream := range streams {
		stream.assertResponse(t, body)
	}
	observer.mu.Lock()
	defer observer.mu.Unlock()
	if observer.maximum != 64 || observer.active != 0 || observer.total != 257 || observer.lastID != 513 || observer.invalidID {
		t.Fatalf("Publisher stream accounting: max=%d active=%d total=%d last=%d invalid=%v", observer.maximum, observer.active, observer.total, observer.lastID, observer.invalidID)
	}
}

func TestPublisherWorkerDrainsCancelledStreamWithoutLosingSnapshot(t *testing.T) {
	body := bytes.Repeat([]byte("a"), 160<<10)
	harness := startPublisherHarness(t, body, nil)
	failed := newPublisherFixture(publisherRequest(), make(chan struct{}))
	harness.admit(t, failed)
	select {
	case <-failed.started:
	case <-time.After(5 * time.Second):
		t.Fatal("failed stream did not start")
	}
	failed.finish(connection.IndeterminateFailure)
	valid := newPublisherFixture(publisherRequest(), nil)
	harness.admit(t, valid)
	harness.drain(t)
	valid.assertResponse(t, body)
}

func TestPublisherWorkerPreservesServiceCleanupFailure(t *testing.T) {
	harness := startPublisherHarness(t, []byte("snapshot"), nil)
	stream := newPublisherFixture(publisherRequest(), nil)
	cleanupErr := errors.New("fixture cleanup did not join")
	stream.closeErr = cleanupErr
	harness.admit(t, stream)
	close(harness.incoming)
	select {
	case <-harness.finished:
		if !errors.Is(harness.err, cleanupErr) {
			t.Fatalf("cleanup error lost: %v", harness.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cleanup failure did not finish the Publisher job")
	}
}
