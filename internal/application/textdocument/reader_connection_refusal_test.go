//go:build linux

package textdocument_test

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/interfacev2/connection"
	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
)

type blockedReaderService struct {
	entered   chan struct{}
	closed    chan struct{}
	done      chan connection.Outcome
	readOnce  sync.Once
	closeOnce sync.Once
}

func (stream *blockedReaderService) Read([]byte) (int, error) {
	stream.readOnce.Do(func() { close(stream.entered) })
	<-stream.closed
	return 0, io.EOF
}
func (stream *blockedReaderService) Write([]byte) (int, error) {
	<-stream.closed
	return 0, io.ErrClosedPipe
}
func (stream *blockedReaderService) CloseInput() error               { return nil }
func (stream *blockedReaderService) Done() <-chan connection.Outcome { return stream.done }
func (stream *blockedReaderService) Close() error {
	stream.closeOnce.Do(func() { close(stream.closed); close(stream.done) })
	return nil
}

func TestReaderWorkerRefusesUnsolicitedOutputBeforeServiceCompletion(t *testing.T) {
	for _, test := range []struct {
		name string
		kind byte
		id   uint32
		body []byte
	}{
		{"early result", 6, 2, []byte{0, 0, 0, 0, 0}}, {"worker open", 1, 1, nil}, {"foreign stream", 2, 3, []byte{1}}, {"credit overflow", 3, 1, []byte{0, 1, 0, 1}},
	} {
		t.Run(test.name, func(t *testing.T) {
			endpoint, peer := net.Pipe()
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			service := &blockedReaderService{entered: make(chan struct{}), closed: make(chan struct{}), done: make(chan connection.Outcome)}
			finished := make(chan error, 1)
			joined := make(chan struct{})
			go func() {
				defer close(joined)
				body, err := textdocument.ReadWorkerConnection(ctx, endpoint, service)
				if body != nil {
					t.Error("refused worker returned document bytes")
				}
				finished <- err
			}()
			t.Cleanup(func() {
				cancel()
				_ = peer.Close()
				select {
				case <-joined:
				case <-time.After(time.Second):
					t.Error("reader did not join")
				}
			})
			if err := peer.SetDeadline(time.Now().Add(time.Second)); err != nil {
				t.Fatal(err)
			}
			open := readWorkerWire(t, peer)
			if open.kind != 1 || open.id != 1 {
				t.Fatal("reader did not open its one Service stream")
			}
			writeWorkerWire(t, peer, test.kind, test.id, test.body)
			select {
			case err := <-finished:
				if err == nil {
					t.Fatal("invalid worker output accepted")
				}
			case <-ctx.Done():
				t.Fatal("invalid worker output did not fail promptly")
			}
			select {
			case <-service.closed:
			default:
				t.Fatal("refusal did not close Service")
			}
		})
	}
}

func TestReaderWorkerCancellationJoinsBlockedServiceAndWorkerIO(t *testing.T) {
	endpoint, peer := net.Pipe()
	defer peer.Close()
	ctx, cancel := context.WithCancel(t.Context())
	service := &blockedReaderService{entered: make(chan struct{}), closed: make(chan struct{}), done: make(chan connection.Outcome)}
	finished := make(chan error, 1)
	joined := make(chan struct{})
	go func() {
		defer close(joined)
		_, err := textdocument.ReadWorkerConnection(ctx, endpoint, service)
		finished <- err
	}()
	t.Cleanup(func() {
		cancel()
		_ = peer.Close()
		select {
		case <-joined:
		case <-time.After(time.Second):
			t.Error("reader cleanup did not join")
		}
	})
	if err := peer.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if open := readWorkerWire(t, peer); open.kind != 1 {
		t.Fatal("missing Service OPEN")
	}
	select {
	case <-service.entered:
	case <-time.After(time.Second):
		t.Fatal("Service read did not start")
	}
	cancel()
	select {
	case err := <-finished:
		if err == nil {
			t.Fatal("cancelled reader succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("reader cancellation did not join")
	}
}

func TestReaderWorkerRejectsOversizedFrameBeforePayloadRead(t *testing.T) {
	endpoint, peer := net.Pipe()
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	service := &blockedReaderService{entered: make(chan struct{}), closed: make(chan struct{}), done: make(chan connection.Outcome)}
	finished := make(chan error, 1)
	joined := make(chan struct{})
	go func() {
		defer close(joined)
		_, err := textdocument.ReadWorkerConnection(ctx, endpoint, service)
		finished <- err
	}()
	t.Cleanup(func() {
		cancel()
		_ = peer.Close()
		select {
		case <-joined:
		case <-time.After(time.Second):
			t.Error("reader did not join")
		}
	})
	if err := peer.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	_ = readWorkerWire(t, peer)
	var header [9]byte
	header[0] = 2
	binary.BigEndian.PutUint32(header[1:], 1)
	binary.BigEndian.PutUint32(header[5:], 16385)
	if _, err := peer.Write(header[:]); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-finished:
		if err == nil {
			t.Fatal("oversized frame accepted")
		}
	case <-ctx.Done():
		t.Fatal("oversized frame waited for its payload")
	}
}
