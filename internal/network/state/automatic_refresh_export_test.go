package state

import (
	"time"
)

// OpenWithAutomaticTicksForTest opens State with test-owned automatic ticks.
func OpenWithAutomaticTicksForTest(input Config, ticks <-chan time.Time, results chan<- error) (*networkState, error) {
	return open(input, ticks, results)
}
