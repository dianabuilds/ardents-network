package textdocument

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/application/interfacev2/connection"
	"io"
	"unicode/utf8"
)

const (
	MaximumBytes = 4 << 20
	requestBytes = 512
	magic        = "ARDTXT01"
)

// Read sends one explicit document request. It returns content only after an
// exact bounded response, directional EOF, authenticated terminal success and
// joined stream cleanup. No failure retries or emits a second request.
func Read(ctx context.Context, stream connection.Stream) (body []byte, resultErr error) {
	if ctx == nil || stream == nil {
		return nil, errors.New("text read is unavailable")
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
	var request [requestBytes]byte
	copy(request[:], magic)
	request[8] = 1
	if _, err := io.Copy(stream, bytes.NewReader(request[:])); err != nil {
		return nil, err
	}
	if err := stream.CloseInput(); err != nil {
		return nil, err
	}
	var header [13]byte
	if _, err := io.ReadFull(stream, header[:]); err != nil {
		return nil, errors.New("text response is interrupted")
	}
	length := binary.BigEndian.Uint32(header[9:])
	if string(header[:8]) != magic || header[8] > 1 || length > MaximumBytes || (header[8] == 1 && length != 0) {
		return nil, errors.New("text response is invalid")
	}
	body, err := io.ReadAll(io.LimitReader(stream, int64(length)+1))
	if err != nil || len(body) != int(length) || !utf8.Valid(body) {
		return nil, errors.New("text response is interrupted or invalid")
	}
	select {
	case outcome, ok := <-stream.Done():
		if !ok || outcome.Class != connection.CleanClose {
			return nil, errors.New("text Connection did not complete")
		}
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if header[8] == 1 {
		return nil, errors.New("text document is unavailable")
	}
	return body, nil
}
