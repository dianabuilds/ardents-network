package artifacts

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"syscall"
	"testing"
	"time"
)

type outputWriter func([]byte) (int, error)

func (write outputWriter) Write(raw []byte) (int, error) { return write(raw) }

func TestCommandAndArtifactOutputKeepDifferentWriterFailures(t *testing.T) {
	refused := errors.New("original output failed")
	payload := []byte("report\n")
	for _, test := range []struct {
		name         string
		count        int
		failure      error
		wantReport   error
		receiptShort bool
	}{
		{name: "complete", count: len(payload)},
		{name: "short", count: 1, wantReport: io.ErrShortWrite, receiptShort: true},
		{name: "writer-failure", count: 1, failure: refused, wantReport: refused, receiptShort: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			writer := outputWriter(func([]byte) (int, error) { return test.count, test.failure })
			if err := WriteReport(t.Context(), writer, payload); err != test.wantReport {
				t.Fatalf("command report lost its exact original result: %v", err)
			}
			err := WriteReceipt(t.Context(), writer, payload)
			if errors.Is(err, io.ErrShortWrite) != test.receiptShort ||
				(test.failure != nil && !errors.Is(err, test.failure)) ||
				(!test.receiptShort && err != nil) {
				t.Fatalf("artifact receipt changed its failure classification: %v", err)
			}
		})
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	writer := outputWriter(func([]byte) (int, error) {
		t.Fatal("canceled context reached output")
		return 0, refused
	})
	for _, write := range []func(context.Context, io.Writer, []byte) error{WriteReport, WriteReceipt} {
		if err := write(ctx, writer, payload); !errors.Is(err, context.Canceled) {
			t.Fatalf("original cancellation lost: %v", err)
		}
	}
}

// TestWriteReportJoinsCanceledFullPipe covers both a small public receipt and
// a large report using real native output and the original cancellation cause.
func TestWriteReportJoinsCanceledFullPipe(t *testing.T) {
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"receipt", []byte("public receipt\n")},
		{"large", bytes.Repeat([]byte{1}, 4<<20)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := errors.Join(r.Close(), w.Close()); err != nil {
					t.Error(err)
				}
			})
			fd := int(w.Fd())
			if err := syscall.SetNonblock(fd, true); err != nil {
				t.Fatal(err)
			}
			chunk := bytes.Repeat([]byte{1}, 4096)
			for {
				_, err := syscall.Write(fd, chunk)
				if errors.Is(err, syscall.EAGAIN) {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- WriteReport(ctx, w, tc.data) }()
			select {
			case err := <-done:
				t.Fatal("full pipe completed before cancellation", err)
			case <-time.After(20 * time.Millisecond):
			}
			cancel()
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) {
					t.Fatal(err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("blocked report did not join")
			}
		})
	}
}
