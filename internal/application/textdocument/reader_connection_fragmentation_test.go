//go:build linux

package textdocument_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/interfacev2/connection"
	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
)

// Split actual RunWorker output, preserving its bytes, credit and direction.
// This changes framing only; it does not substitute a different reader job.
type fragmentedRequestAttachment struct {
	net.Conn
	ready        bool
	pending      []byte
	alterPadding bool
}

func (attachment *fragmentedRequestAttachment) Write(body []byte) (int, error) {
	if !attachment.ready {
		if len(body) != 72 {
			return 0, errors.New("unexpected readiness write")
		}
		_, err := io.Copy(attachment.Conn, bytes.NewReader(body))
		if err != nil {
			return 0, err
		}
		attachment.ready = true
		return len(body), nil
	}
	if len(attachment.pending) == 0 {
		if len(body) != 9 {
			return 0, errors.New("unexpected frame header write")
		}
		attachment.pending = bytes.Clone(body)
	} else {
		attachment.pending = append(attachment.pending, body...)
	}
	length := binary.BigEndian.Uint32(attachment.pending[5:9])
	if length > 16384 || len(attachment.pending) > 9+int(length) {
		return 0, errors.New("unexpected frame body")
	}
	if len(attachment.pending) == 9+int(length) {
		frame := attachment.pending
		if frame[0] == 2 && binary.BigEndian.Uint32(frame[1:5]) == 1 {
			var fragment [10]byte
			fragment[0] = 2
			binary.BigEndian.PutUint32(fragment[1:5], 1)
			binary.BigEndian.PutUint32(fragment[5:9], 1)
			for index, value := range frame[9:] {
				if attachment.alterPadding && index == len(frame[9:])-1 {
					value = 1
				}
				fragment[9] = value
				if _, err := io.Copy(attachment.Conn, bytes.NewReader(fragment[:])); err != nil {
					return 0, err
				}
			}
		} else if _, err := io.Copy(attachment.Conn, bytes.NewReader(frame)); err != nil {
			return 0, err
		}
		attachment.pending = nil
	}
	return len(body), nil
}

func TestReaderWorkerRequestMayUseOneByteFramesWithoutQueueDeadlock(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	endpoint, worker := net.Pipe()
	finished := make(chan error, 1)
	go func() {
		finished <- textdocument.RunWorker(ctx, &fragmentedRequestAttachment{Conn: worker}, textdocument.ReaderWorker)
	}()
	t.Cleanup(func() {
		cancel()
		_ = endpoint.Close()
		select {
		case <-finished:
		case <-time.After(time.Second):
			t.Error("fragmented worker did not join")
		}
	})
	if err := textdocument.InitializeWorker(ctx, endpoint, textdocument.ReaderWorker, [32]byte{1}, nil); err != nil {
		t.Fatal(err)
	}
	stream := &responseStream{input: bytes.NewReader([]byte("ARDTXT01\x00\x00\x00\x00\x02ok")), done: make(chan connection.Outcome, 1)}
	stream.done <- connection.Outcome{Class: connection.CleanClose}
	close(stream.done)
	body, err := textdocument.ReadWorkerConnection(ctx, endpoint, stream)
	if err != nil || string(body) != "ok" || stream.request.Len() != 512 || !stream.inputClosed || !stream.closed {
		t.Fatalf("fragmented exchange: %q %v request=%d", body, err, stream.request.Len())
	}
}

func TestReaderWorkerMalformedRequestNeverReachesService(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	endpoint, worker := net.Pipe()
	finished := make(chan error, 1)
	go func() {
		finished <- textdocument.RunWorker(ctx, &fragmentedRequestAttachment{Conn: worker, alterPadding: true}, textdocument.ReaderWorker)
	}()
	t.Cleanup(func() {
		cancel()
		_ = endpoint.Close()
		select {
		case <-finished:
		case <-time.After(time.Second):
			t.Error("malformed worker did not join")
		}
	})
	if err := textdocument.InitializeWorker(ctx, endpoint, textdocument.ReaderWorker, [32]byte{1}, nil); err != nil {
		t.Fatal(err)
	}
	stream := &responseStream{input: bytes.NewReader([]byte("ARDTXT01\x00\x00\x00\x00\x02ok")), done: make(chan connection.Outcome, 1)}
	stream.done <- connection.Outcome{Class: connection.CleanClose}
	close(stream.done)
	body, err := textdocument.ReadWorkerConnection(ctx, endpoint, stream)
	if err == nil || body != nil || stream.request.Len() != 0 || !stream.closed {
		t.Fatalf("malformed request reached Service: bytes=%d result=%q err=%v", stream.request.Len(), body, err)
	}
}
