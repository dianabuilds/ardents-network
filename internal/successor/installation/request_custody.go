package installation

import (
	"context"
	"errors"
)

var ErrNativeUnavailable = errors.New("installation: native request custody unavailable")

// ReadOwnedRequest freezes one canonical request from a direct root-owned file
// and trusted ancestors. It grants no Release proof or installed authority.
// Native effects must reobserve this exact provenance under their own lease.
func ReadOwnedRequest(ctx context.Context, filename string, initial bool) (Request, error) {
	if ctx == nil {
		return Request{}, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return Request{}, err
	}
	return readOwnedRequest(ctx, filename, initial)
}
