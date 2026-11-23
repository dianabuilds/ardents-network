package artifacts

import (
	"context"
	"errors"
	"io"
	"testing"
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
