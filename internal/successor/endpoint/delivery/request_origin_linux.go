package delivery

import (
	"context"
	"errors"

	requestinput "github.com/dianabuilds/ardents-network/internal/successor/endpoint/delivery/request"
)

// Initial and successor preparation independently reobserve their original
// request through its original document; neither borrows the other's lifetime.
func observeRequestCustody(ctx context.Context, input requestinput.Document) error {
	err := input.Observe(ctx)
	if errors.Is(err, requestinput.ErrChanged) {
		return ErrBinding
	}
	return err
}
