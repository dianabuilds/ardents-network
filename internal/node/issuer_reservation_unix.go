//go:build unix

package node

import (
	"context"
	"errors"
	"time"

	"github.com/dianabuilds/ardents-network/internal/hosting"
	"github.com/dianabuilds/ardents-network/internal/route/receiver"
)

// holdIssuerReservation transfers the original work and termination budget to
// the channel. Termination remains held until the channel's physical join.
func holdIssuerReservation(channel receiver.Channel, held *hosting.Reservation) (func() error, error) {
	terminal, err := held.DetachTermination()
	if err != nil {
		release, holdErr := channel.HoldReservation(held.ReleaseJoined)
		return release, errors.Join(err, holdErr)
	}
	return channel.HoldIssuerReservation(held.ReleaseJoined, func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return terminal(ctx)
	})
}
