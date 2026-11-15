// Package journal retains the Endpoint delivery preparation and transition
// records and their original native custody. Delivery owns admission, phase
// policy, Release checks and actual process completion. This transitional
// package cannot authorize an update, start or recovery by itself.
package journal

import "errors"

var (
	ErrInput       = errors.New("invalid Installation preparation journal")
	ErrBinding     = errors.New("installation preparation journal differs")
	ErrUnavailable = errors.New("installation preparation journal unavailable")
)
