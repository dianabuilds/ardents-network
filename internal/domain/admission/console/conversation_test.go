package console

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"testing/synctest"
)

func TestRunCancellationClosesOriginalInput(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		input, writer := io.Pipe()
		defer input.Close()
		defer writer.Close()
		var output bytes.Buffer
		done := make(chan int, 1)
		go func() {
			done <- Run(ctx, input, &output, func(_ context.Context, request []byte) (any, bool, error) {
				return string(request), false, nil
			})
		}()
		if _, err := io.WriteString(writer, "status\n"); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		if output.String() != "\"status\"\n" {
			t.Fatalf("original conversation did not render the request: %q", output.String())
		}
		cancel()
		synctest.Wait()
		select {
		case code := <-done:
			if code != 130 {
				t.Fatalf("canceled conversation returned %d", code)
			}
		default:
			t.Fatal("canceled conversation still holds the original input")
		}
		if _, err := io.WriteString(writer, "late\n"); !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("cancellation left the original input open: %v", err)
		}
	})
}
