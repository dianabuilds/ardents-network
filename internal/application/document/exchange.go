package document

import (
	"context"
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/application/endpointstream"
)

const MaximumBytes = 4 << 20

// Read closes the unused sending direction and receives bounded raw file bytes.
// It returns content after EOF, authenticated success and joined cleanup.
func Read(ctx context.Context, stream endpointstream.Stream) (body []byte, resultErr error) {
	if ctx == nil || stream == nil {
		return nil, errors.New("document read is unavailable")
	}
	stop := context.AfterFunc(ctx, func() { _ = stream.Close() })
	defer func() {
		stop()
		resultErr = errors.Join(resultErr, stream.Close(), ctx.Err())
		if resultErr != nil {
			body = nil
		}
	}()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := stream.CloseInput(); err != nil {
		return nil, err
	}
	body, err := io.ReadAll(io.LimitReader(stream, MaximumBytes+1))
	if err != nil || len(body) > MaximumBytes {
		return nil, errors.New("file transfer is interrupted or exceeds its bound")
	}
	select {
	case outcome, ok := <-stream.Done():
		if !ok || outcome.Class != endpointstream.CleanClose {
			return nil, errors.New("document Connection did not complete")
		}
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return body, nil
}
