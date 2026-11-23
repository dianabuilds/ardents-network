//go:build linux

package connection

import (
	"context"
	"errors"
)

// DialPublishedStream requests the fixed byte-stream profile through local
// Service Administration. Its established exchange uses the same byte frames.
func DialPublishedStream(ctx context.Context, path string) (Client, error) {
	if ctx == nil || path == "" {
		return nil, errors.New("local Application dial input is invalid")
	}
	return dialLocalStream(ctx, path, []byte(PublishByteStreamRequest), true)
}
