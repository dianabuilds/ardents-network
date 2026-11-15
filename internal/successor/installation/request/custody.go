package request

import (
	"context"

	"github.com/dianabuilds/ardents-network/internal/successor/protectedfiles"
)

// ErrNativeUnavailable refuses unavailable or foreign native custody.
var ErrNativeUnavailable = protectedfiles.ErrNativeUnavailable

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
var ErrChanged = protectedfiles.ErrChanged
