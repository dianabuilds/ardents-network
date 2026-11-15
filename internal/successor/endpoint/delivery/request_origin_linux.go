package delivery

import (
	"context"
	"errors"

	requestinput "github.com/dianabuilds/ardents-network/internal/successor/endpoint/delivery/request"
)

// Initial and successor preparation independently reobserve their original
// request through its native Origin; neither borrows the other's lifetime.
func observeRequestCustody(ctx context.Context, input requestinput.Document) error {
	declared, valid := input.Declaration()
	origin := input.Origin()
	if ctx == nil || origin == nil || !valid {
		return ErrInput
	}
	err := origin.Observe(ctx, declared)
	if errors.Is(err, requestinput.ErrChanged) {
		return ErrBinding
	}
	return err
}
