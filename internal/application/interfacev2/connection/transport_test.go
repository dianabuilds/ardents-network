//go:build linux

package connection_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/interfacev2/connection"
)

type echoOwner struct{ calls atomic.Int32 }

func (owner *echoOwner) Open(_ context.Context, request connection.Request) (connection.Stream, error) {
	owner.calls.Add(1)
	reader, writer := io.Pipe()
	return &echoStream{reader: reader, writer: writer, done: make(chan connection.Outcome, 1)}, nil
}

type echoStream struct {
	reader    *io.PipeReader
	writer    *io.PipeWriter
	done      chan connection.Outcome
	inputOnce sync.Once
	closeOnce sync.Once
}

func (stream *echoStream) Read(body []byte) (int, error)  { return stream.reader.Read(body) }
func (stream *echoStream) Write(body []byte) (int, error) { return stream.writer.Write(body) }
func (stream *echoStream) CloseInput() error {
	stream.inputOnce.Do(func() {
		_ = stream.writer.Close()
		stream.done <- connection.Outcome{Class: connection.CleanClose}
		close(stream.done)
	})
	return nil
}
func (stream *echoStream) Close() error {
	stream.closeOnce.Do(func() { _ = stream.reader.Close(); _ = stream.writer.Close() })
	return nil
}
func (stream *echoStream) Done() <-chan connection.Outcome { return stream.done }

func TestTypedConnectionCarriesOrderedBytesAndTerminal(t *testing.T) {
	owner := &echoOwner{}
	path := shortSocketPath(t)
	server, err := connection.Listen(path, owner)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	stream, err := connection.Dial(ctx, path, connection.Request{Destination: connection.TargetLink, Value: "explicit-link"})
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	payload := bytes.Repeat([]byte("bounded"), 4000)
	written := make(chan error, 1)
	go func() {
		_, err := stream.Write(payload)
		if err == nil {
			err = stream.CloseInput()
		}
		written <- err
	}()
	got, err := io.ReadAll(stream)
	if err != nil || !bytes.Equal(got, payload) {
		t.Fatalf("ordered stream = %d bytes, %v", len(got), err)
	}
	if err := <-written; err != nil {
		t.Fatal(err)
	}
	outcome, ok := <-stream.Done()
	if !ok || outcome.Class != connection.CleanClose {
		t.Fatalf("terminal = %+v / %v", outcome, ok)
	}
	if owner.calls.Load() != 1 {
		t.Fatal("local request was retried")
	}
}

func TestReservedNameRefusesBeforeOwnerEffects(t *testing.T) {
	owner := &echoOwner{}
	path := shortSocketPath(t)
	server, err := connection.Listen(path, owner)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	stream, err := connection.Dial(ctx, path, connection.Request{Destination: connection.Name, Value: "reserved"})
	if stream != nil || err == nil || !strings.Contains(err.Error(), "not-selected") || owner.calls.Load() != 0 {
		t.Fatalf("reserved Name = %v, %v; calls=%d", stream, err, owner.calls.Load())
	}
}

func TestAAI2RequestRefusesBeforeOwnerEffects(t *testing.T) {
	owner := &echoOwner{}
	path := shortSocketPath(t)
	server, err := connection.Listen(path, owner)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Errorf("close AAI3 server: %v", err)
		}
	})
	peer, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := peer.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Errorf("close AAI2 peer: %v", err)
		}
	})
	if err := peer.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	const target = "ardents-target:v1:retired"
	request := make([]byte, 6+len(target)+4+len("forbidden frame"))
	copy(request, "AAI2")
	binary.BigEndian.PutUint16(request[4:6], uint16(len(target)))
	copy(request[6:], target)
	frame := request[6+len(target):]
	binary.BigEndian.PutUint32(frame[:4], uint32(len("forbidden frame")))
	copy(frame[4:], "forbidden frame")
	if _, err := peer.Write(request); err != nil {
		t.Fatal(err)
	}
	var status [1]byte
	if _, err := io.ReadFull(peer, status[:]); err != nil || status[0] != 0 {
		t.Fatalf("AAI2 refusal status = %d, %v", status[0], err)
	}
	if owner.calls.Load() != 0 {
		t.Fatalf("AAI2 request reached Application owner %d times", owner.calls.Load())
	}
}

func shortSocketPath(t *testing.T) string {
	t.Helper()
	root, err := os.MkdirTemp("", "aai3-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Remove(root); err != nil {
			t.Error(err)
		}
	})
	return filepath.Join(root, "connection.sock")
}
