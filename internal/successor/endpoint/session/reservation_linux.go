//go:build linux

package session

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/hosting"
)

func releaseRouteReservation(reservation *hosting.Reservation) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return reservation.Release(ctx)
}
func independentRouteRoots(roots ...string) bool {
	for i, root := range roots {
		if !filepath.IsAbs(root) || filepath.Clean(root) != root {
			return false
		}
		for _, other := range roots[:i] {
			if root == other || strings.HasPrefix(root, other+string(filepath.Separator)) || strings.HasPrefix(other, root+string(filepath.Separator)) {
				return false
			}
		}
	}
	return true
}
func minRouteDeadline(end time.Time, bounds ...time.Time) time.Time {
	for _, bound := range bounds {
		if bound.Before(end) {
			end = bound
		}
	}
	return end
}
