package recoverysmoke

import (
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/recovery"
)

func sameResourceObservation(left, right recovery.ResourceSample) bool {
	interval := right.AtNanos - left.AtNanos
	if interval < 0 || interval > int64(10*time.Millisecond) {
		return false
	}
	left.AtNanos, right.AtNanos = 0, 0
	return left == right
}
