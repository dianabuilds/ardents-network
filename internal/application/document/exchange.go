package document

import (
	"context"
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/application/endpointstream"
)

const MaximumBytes = 4 << 20

// ReceiveFile writes to a provisional sink. It succeeds after authenticated
// terminal completion and joined cleanup, without buffering the whole file.
func ReceiveFile(ctx context.Context, stream endpointstream.Stream, output io.Writer) (resultErr error) {
	if ctx == nil || stream == nil || output == nil {
		return errors.New("document read is unavailable")
	}
	callbackDone := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		defer close(callbackDone)
		_ = stream.Close()
	})
	defer func() {
		stopped := stop()
		resultErr = errors.Join(resultErr, stream.Close(), ctx.Err())
		if !stopped {
			<-callbackDone
		}
	}()
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := stream.CloseInput(); err != nil {
		return err
	}
	var scratch [16 << 10]byte
	defer clear(scratch[:])
	// Hide ReaderFrom so the copy always uses the caller-owned scratch.
	plainOutput := struct{ io.Writer }{output}
	if _, err := io.CopyBuffer(plainOutput, stream, scratch[:]); err != nil {
		return errors.New("file transfer is interrupted")
	}
	select {
	case outcome, ok := <-stream.Done():
		if !ok || outcome.Class != endpointstream.CleanClose {
			return errors.New("document Connection did not complete")
		}
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}
