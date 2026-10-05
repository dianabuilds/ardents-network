//go:build linux

package carrier

import "time"

// quicDeadline preserves the absolute deadline while attaching the current
// local monotonic clock. quic-go converts wall-only times against its package
// startup epoch; a wall-clock adjustment since startup would otherwise shift
// the physical timeout away from the deadline Route supplied.
func quicDeadline(deadline time.Time) time.Time {
	if deadline.IsZero() {
		return deadline
	}
	now := time.Now()
	return now.Add(deadline.Sub(now))
}
