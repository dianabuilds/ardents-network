package document

import (
	"bytes"
	"context"
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/application/endpointstream"
)

const MaximumBytes = 4 << 20

// Read closes the unused sending direction and receives bounded raw file bytes.
// It returns content after EOF, authenticated success and joined cleanup.
func Read(ctx context.Context, stream endpointstream.Stream) ([]byte, error) {
	body := bytes.NewBuffer([]byte{})
	if err := receiveDocument(ctx, stream, body); err != nil {
		clear(body.Bytes())
		return nil, err
	}
	return body.Bytes(), nil
}

// receiveDocument writes only to provisional output. The caller can expose
// output after this returns successfully.
func receiveDocument(ctx context.Context, stream endpointstream.Stream, output io.Writer) (resultErr error) {
	if ctx == nil || stream == nil {
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
	n, err := io.CopyBuffer(plainOutput, io.LimitReader(stream, MaximumBytes), scratch[:])
	if err != nil {
		return errors.New("file transfer is interrupted or exceeds its bound")
	}
	if n == MaximumBytes {
		var excess [1]byte
		if count, err := io.ReadFull(stream, excess[:]); count != 0 || err != io.EOF {
			return errors.New("file transfer is interrupted or exceeds its bound")
		}
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
