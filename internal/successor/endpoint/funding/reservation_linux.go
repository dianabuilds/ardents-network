//go:build linux

package funding

import (
	"context"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
)

func releaseRouteReservation(reservation *hosting.Reservation) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return reservation.Release(ctx)
}
