package prefix

import (
	"context"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/route/role"

	"github.com/dianabuilds/ardents-network/internal/successor/admission"
	"github.com/dianabuilds/ardents-network/internal/successor/route/ardp"
	framing "github.com/dianabuilds/ardents-network/internal/successor/route/channel"
)

// replenishParent composes holder presentation with one channel exchange.
// Stock owns the durable presentation; the channel retains its original
// witness and commits the replacement once, before the acknowledgement debit.
func replenishParent(s *framing.Session, ctx, caller context.Context, hello ardp.Hello, present role.Presentation, holds ...func(context.Context, uint64) error) error {
	if s == nil || ctx == nil || caller == nil || present == nil || hello.Purpose != ardp.PurposeForwarding {
		return errors.New("route replenishment composition unavailable")
	}
	return s.ExchangeParent(ctx, caller, hello.Deadline, admission.ForwardClass.ByteLimit(), func(ctx context.Context) (ardp.Frame, error) {
		raw, err := present(ctx, hello)
		defer clear(raw)
		if err != nil {
			return ardp.Frame{}, err
		}
		if len(raw) != 354 {
			return ardp.Frame{}, errors.New("route refill token length invalid")
		}
		return ardp.Frame{Kind: ardp.KindAdmit, Body: append([]byte{2}, raw...)}, nil
	}, holds...)
}
