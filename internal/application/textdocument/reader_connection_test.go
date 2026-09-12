//go:build linux

package textdocument_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/interfacev2/connection"
	"github.com/dianabuilds/ardents-network/internal/application/textdocument"
)

func TestReaderWorkerUsesOneAdmittedServiceStreamAndJoinsItsResult(t *testing.T) {
	for _, size := range []int{0, 64 << 10, 160 << 10, textdocument.MaximumBytes} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			body := bytes.Repeat([]byte("x"), size)
			response := make([]byte, 13, len(body)+13)
			copy(response, "ARDTXT01")
			binary.BigEndian.PutUint32(response[9:], uint32(size))
			response = append(response, body...)
			stream := &responseStream{input: bytes.NewReader(response), done: make(chan connection.Outcome, 1)}
			stream.done <- connection.Outcome{Class: connection.CleanClose}
			close(stream.done)
			worker, _ := startTextWorker(t, textdocument.ReaderWorker, nil)
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			actual, err := textdocument.ReadWorkerConnection(ctx, worker, stream)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(actual, body) || !stream.closed || !stream.inputClosed || stream.request.Len() != 512 {
				t.Fatal("worker result or one-request joined lifecycle was lost")
			}
			if string(stream.request.Bytes()[:9]) != "ARDTXT01\x01" {
				t.Fatal("Service did not receive the worker's fixed request")
			}
		})
	}
}

func TestReaderWorkerNeverReturnsContentOnInterruptedServiceOrCleanup(t *testing.T) {
	for _, test := range []struct {
		name     string
		terminal connection.OutcomeClass
		cleanup  error
	}{
		{"missing terminal", "", nil}, {"non-clean terminal", connection.LocalFailure, nil}, {"cleanup failure", connection.CleanClose, io.ErrClosedPipe},
	} {
		t.Run(test.name, func(t *testing.T) {
			stream := &responseStream{input: bytes.NewReader([]byte("ARDTXT01\x00\x00\x00\x00\x02ok")), done: make(chan connection.Outcome, 1), closeErr: test.cleanup}
			if test.terminal != "" {
				stream.done <- connection.Outcome{Class: test.terminal}
			}
			close(stream.done)
			worker, _ := startTextWorker(t, textdocument.ReaderWorker, nil)
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			body, err := textdocument.ReadWorkerConnection(ctx, worker, stream)
			if err == nil || body != nil || !stream.closed {
				t.Fatal("failed Connection or cleanup returned content")
			}
		})
	}
}

func TestReaderWorkerTransfersThroughAAI3WithoutSecondOpen(t *testing.T) {
	directory, err := os.MkdirTemp("", "text-aai3-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Remove(directory); err != nil {
			t.Error(err)
		}
	})
	body := bytes.Repeat([]byte("closed text\n"), 8192)
	snapshot, err := textdocument.NewSnapshot(body)
	if err != nil {
		t.Fatal(err)
	}
	owner := &snapshotConnectionOwner{snapshot: snapshot}
	address := filepath.Join(directory, "connection.sock")
	server, err := connection.Listen(address, owner)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Close(); err != nil {
			t.Error(err)
		}
	})
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	stream, err := connection.Dial(ctx, address, connection.Request{Destination: connection.TargetLink, Value: "fixture-link"})
	if err != nil {
		t.Fatal(err)
	}
	worker, _ := startTextWorker(t, textdocument.ReaderWorker, nil)
	actual, err := textdocument.ReadWorkerConnection(ctx, worker, stream)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual, body) || owner.opens.Load() != 1 || owner.bytes.Load() != 512 {
		t.Fatal("AAI3 exchange lost its exact content or one-request binding")
	}
}
