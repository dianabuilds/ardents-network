package carrier

import "time"

// quicDeadline preserves the supplied instant with the current local monotonic
// clock. quic-go otherwise converts wall time against its startup epoch, so a
// later wall clock correction can shift the physical timeout.
func quicDeadline(deadline time.Time) time.Time {
	if deadline.IsZero() {
		return deadline
	}
	now := time.Now()
	return now.Add(deadline.Sub(now))
}
