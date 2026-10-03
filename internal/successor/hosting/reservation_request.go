package hosting

import "time"

// ReservationRequest fixes work and cleanup coverage before any durable I/O.
// WorkUntil is the last time a reservation may be transferred for work.
// HoldUntil covers completion; neither date automatically refunds a reservation.
type ReservationRequest struct {
	Work, Termination    Traffic
	WorkUntil, HoldUntil time.Time
}
