package request

import (
	"context"
	"errors"
)

// ErrNativeUnavailable refuses unavailable or foreign native custody.
var ErrNativeUnavailable = errors.New("installation: native request custody unavailable")

// ReadOwned freezes one canonical request from a direct root-owned file
// and trusted ancestors. It grants no Release proof or installed authority.
// Native effects must reobserve this exact provenance under their own lease.
func ReadOwned(ctx context.Context, filename string, initial bool) (Document, error) {
	if ctx == nil {
		return Document{}, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return Document{}, err
	}
	return readOwned(ctx, filename, initial)
}

// ErrChanged refuses a different declaration or original file after admission.
var ErrChanged = errors.New("installation request: original input changed")
