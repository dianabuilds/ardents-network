package endpoint

import (
	"context"
	"errors"

	requestinput "github.com/dianabuilds/ardents-network/internal/successor/installation/request"
)

// Initial and successor preparation independently reobserve their original
// request through its native Origin; neither borrows the other's lifetime.
func observeRequestCustody(ctx context.Context, input Request) error {
	if ctx == nil || input.custody == nil || input.declared == nil {
		return ErrInput
	}
	err := input.custody.Observe(ctx, *input.declared)
	if errors.Is(err, requestinput.ErrChanged) {
		return ErrBinding
	}
	return err
}
